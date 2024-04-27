package session

import (
	"context"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/server/proxyer/sender"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"github.com/pkg/errors"

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

func (s *Session) close(cause error) error {
	if s.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if s.mgr != nil {
			s.mgr.del(s)
		}
		s.srvCancel()

		if s.sender != nil {
			if err := s.sender.Close(); err != nil {
				cause = err
			}
		}

		sess := s.session
		if s.proxyer != nil {
			if err := s.proxyer.Adapter().DelPort(
				sess.Proto, s.locAddr.Port(), sess.Dst,
			); err != nil {
				cause = err
			}
		}

		if cause != nil {
			if errorx.Temporary(cause) {
				s.proxyer.Logger().Info(errors.WithMessage(cause, "session close").Error())
			} else {
				s.proxyer.Logger().Error(cause.Error(), errorx.TraceAttr(cause))
			}
			s.closeErr.Store(&cause)
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *Session) ID() session.ID {
	return s.id
}

func (s *Session) downlinkService() error {
	var (
		mtu = s.proxyer.MTU()
		seg = packet.Make(0, mtu)
	)

	for {
		s.cnt.Add(1)
		seg.Sets(0, mtu)
		err := s.sender.Recv(s.srvCtx, seg)
		if err != nil {
			return s.close(err)
		}

		switch s.session.Proto {
		case header.TCPProtocolNumber:
			header.TCP(seg.Bytes()).SetDestinationPortWithChecksumUpdate(s.session.Src.Port())
		case header.UDPProtocolNumber:
			header.UDP(seg.Bytes()).SetDestinationPortWithChecksumUpdate(s.session.Src.Port())
		default:
		}

		err = s.proxyer.Downlink(seg, s.id)
		if err != nil {
			return s.close(err)
		}
	}
}

func (s *Session) Send(pkt *packet.Packet) error {
	if errPtr := s.closeErr.Load(); errPtr != nil {
		return *errPtr
	}

	switch s.session.Proto {
	case header.TCPProtocolNumber:
		header.TCP(pkt.Bytes()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	case header.UDPProtocolNumber:
		header.UDP(pkt.Bytes()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	default:
	}

	err := s.sender.Send(pkt)
	if err != nil {
		return s.close(err)
	}

	s.cnt.Add(1)
	return nil
}

func (s *Session) keepalive() {
	const magic uint32 = 0x45a2319f
	switch s.cnt.Load() {
	case magic:
		s.close(errors.WithStack(app.KeepaliveExceeded))
	default:
		s.cnt.Store(magic)
		time.AfterFunc(s.proxyer.Keepalive(), s.keepalive)
	}
}

func (s *Session) LocalAddr() netip.AddrPort  { return s.locAddr }
func (s *Session) RemoteAddr() netip.AddrPort { return s.session.Dst }
