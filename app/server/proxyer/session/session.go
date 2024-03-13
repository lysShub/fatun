package session

import (
	"context"
	"log/slog"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app/server/proxyer/sender"
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/session"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/lysShub/relraw"
)

type Proxyer interface {
	// proxyer context
	Context() context.Context

	// session manager delete
	Del(id session.ID, cause error) (err error)

	// logger
	Logger() *slog.Logger

	// proxyer downlink
	Downlink(pkt *relraw.Packet, id session.ID) error
	MTU() int
}

type Session struct {
	ctx     cctx.CancelCtx
	proxyer Proxyer
	closed  atomic.Bool

	id      session.ID
	locAddr netip.AddrPort
	session session.Session

	sender sender.Sender

	cnt atomic.Uint32
}

func newSession(
	proxyer Proxyer,
	id session.ID, session session.Session,
	locAddr netip.AddrPort,
) (*Session, error) {
	var s = &Session{
		ctx:     cctx.WithContext(proxyer.Context()),
		proxyer: proxyer,

		id:      id,
		locAddr: locAddr,
		session: session,
	}

	var err error
	s.sender, err = sender.NewSender(locAddr, session.Proto, session.Dst)
	if err != nil {
		return nil, err
	}

	go s.downlinkService()
	return s, nil
}

func (s *Session) ID() session.ID {
	return s.id
}

func (s *Session) downlinkService() {
	var (
		mtu = s.proxyer.MTU()
		seg = relraw.NewPacket(0, mtu)
	)

	for {
		seg.Sets(0, mtu)
		s.cnt.Add(1)

		err := s.sender.Recv(s.ctx, seg)
		if err != nil {
			s.proxyer.Del(s.id, err)
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
			s.proxyer.Del(s.id, err)
			return
		}
	}
}

func (s *Session) Send(pkt *relraw.Packet) {
	switch s.session.Proto {
	case itun.TCP:
		header.TCP(pkt.Data()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	case itun.UDP:
		header.UDP(pkt.Data()).SetSourcePortWithChecksumUpdate(s.locAddr.Port())
	default:
	}

	err := s.sender.Send(pkt)
	if err != nil {
		s.proxyer.Del(s.id, err)
	}
}

func (s *Session) tick() bool {
	const magic uint32 = 0x45a2319f
	if s.cnt.Load() == magic {
		return true
	} else {
		s.cnt.Store(magic)
		return false
	}
}

func (s *Session) close(cause error) error {
	if s.closed.CompareAndSwap(false, true) {
		s.ctx.Cancel(cause)

		err := errorx.Join(
			s.ctx.Err(),
			s.sender.Close(),
		)

		if errorx.Temporary(err) {
			s.proxyer.Logger().Warn(err.Error())
		} else {
			s.proxyer.Logger().Error(err.Error(), errorx.TraceAttr(err))
		}
		return err
	}
	return nil
}

func (s *Session) LocalAddr() netip.AddrPort  { return s.locAddr }
func (s *Session) RemoteAddr() netip.AddrPort { return s.session.Dst }
