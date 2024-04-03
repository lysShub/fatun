package session

import (
	"context"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/server/proxyer/sender"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/packet"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type mgrDel interface {
	del(*Session)
}

type Session struct {
	mgr     mgrDel
	proxyer Proxyer

	id      session.ID
	session session.Session
	locAddr netip.AddrPort

	srvCtx    context.Context
	srvCancel context.CancelFunc
	sender    sender.Sender

	closeErr atomic.Pointer[error]
	cnt      atomic.Uint32
}

func newSession(
	mgr mgrDel, proxyer Proxyer,
	id session.ID, sess session.Session,
) (*Session, error) {
	var s = &Session{
		mgr:     mgr,
		proxyer: proxyer,

		id:      id,
		session: sess,
	}

	locPort, err := proxyer.Adapter().GetPort(sess.Proto, sess.Dst)
	if err != nil {
		return nil, err
	} else {
		s.locAddr = netip.AddrPortFrom(proxyer.Addr().Addr(), locPort)
	}

	s.srvCtx, s.srvCancel = context.WithCancel(context.Background())
	s.sender, err = sender.NewSender(s.locAddr, sess.Proto, sess.Dst)
	if err != nil {
		proxyer.Adapter().DelPort(sess.Proto, locPort, sess.Dst)
		return nil, err
	}

	go s.downlinkService()
	s.keepalive()
	return s, nil
}

func (s *Session) close(cause error) {
	if cause == nil {
		cause = os.ErrClosed
	}

	if s.closeErr.CompareAndSwap(nil, &cause) {
		err := cause

		if s.mgr != nil {
			s.mgr.del(s)
		}
		s.srvCancel()

		if s.sender != nil {
			err = errorx.Join(err, s.sender.Close())
		}

		sess := s.session
		e := s.proxyer.Adapter().DelPort(sess.Proto, s.locAddr.Port(), sess.Dst)
		err = errorx.Join(err, e)

		s.proxyer.Logger().Info("session close")
		s.closeErr.Store(&err)
	}
}

func (s *Session) ID() session.ID {
	return s.id
}

func (s *Session) downlinkService() {
	var (
		mtu = s.proxyer.MTU()
		seg = packet.NewPacket(0, mtu)
	)

	for {
		s.cnt.Add(1)
		seg.Sets(0, mtu)
		err := s.sender.Recv(s.srvCtx, seg)
		if err != nil {
			s.close(err)
			return
		}

		switch s.session.Proto {
		case itun.TCP:
			header.TCP(seg.Data()).SetDestinationPortWithChecksumUpdate(s.session.Src.Port())
		case itun.UDP:
			header.UDP(seg.Data()).SetDestinationPortWithChecksumUpdate(s.session.Src.Port())
		default:
		}

		err = s.proxyer.Downlink(seg, s.id)
		if err != nil {
			s.close(err)
			return
		}
	}
}

func (s *Session) Send(pkt *packet.Packet) error {
	if errPtr := s.closeErr.Load(); errPtr != nil {
		return *errPtr
	}

	switch s.session.Proto {
	case itun.TCP:
		header.TCP(pkt.Data()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	case itun.UDP:
		header.UDP(pkt.Data()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	default:
	}

	err := s.sender.Send(pkt)
	if err != nil {
		s.close(err)
	}

	s.cnt.Add(1)
	return err
}

func (s *Session) keepalive() {
	const magic uint32 = 0x45a2319f
	switch s.cnt.Load() {
	case magic:
		s.close(itun.KeepaliveExceeded)
	default:
		s.cnt.Store(magic)
		time.AfterFunc(s.proxyer.Keepalive(), s.keepalive)
	}
}

func (s *Session) LocalAddr() netip.AddrPort  { return s.locAddr }
func (s *Session) RemoteAddr() netip.AddrPort { return s.session.Dst }
