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
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
)

type Session struct {
	mgr *SessionMgr

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
	mgr *SessionMgr,
	id session.ID, sess session.Session,
) (*Session, error) {
	var s = &Session{
		mgr: mgr,

		id:      id,
		session: sess,
	}

	locPort, err := mgr.ap.GetPort(sess.Proto, sess.Dst)
	if err != nil {
		return nil, err
	} else {
		s.locAddr = netip.AddrPortFrom(mgr.proxyer.Addr().Addr(), locPort)
	}

	s.srvCtx, s.srvCancel = context.WithCancel(context.Background())
	s.sender, err = sender.NewSender(s.locAddr, sess.Proto, sess.Dst)
	if err != nil {
		mgr.ap.DelPort(sess.Proto, locPort, sess.Dst)
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
		err := errorx.Join(cause, s.mgr.del(s))
		s.srvCancel()
		err = errorx.Join(err, s.sender.Close())

		s.closeErr.Store(&err)
	}
}

func (s *Session) ID() session.ID {
	return s.id
}

func (s *Session) downlinkService() {
	var (
		mtu = s.mgr.proxyer.MTU()
		seg = relraw.NewPacket(0, mtu)
	)

	for {
		seg.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.sender.Recv(s.srvCtx, seg)
		if err != nil {
			s.mgr.proxyer.Logger().Warn(err.Error())
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

		err = s.mgr.proxyer.Downlink(seg, s.id)
		if err != nil {
			s.mgr.proxyer.Logger().Warn(err.Error())
			s.close(err)
			return
		}
	}
}

func (s *Session) Send(pkt *relraw.Packet) error {
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
		time.AfterFunc(s.mgr.proxyer.Keepalive(), s.keepalive)
	}
}

func (s *Session) LocalAddr() netip.AddrPort  { return s.locAddr }
func (s *Session) RemoteAddr() netip.AddrPort { return s.session.Dst }
