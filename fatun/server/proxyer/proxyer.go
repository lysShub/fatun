package proxyer

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Server interface {
	MTU() int
	Logger() *slog.Logger
	AddSession(sess session.Session, pxy IProxyer) error
	Send(sess session.Session, pkt *packet.Packet) error
}

func Proxy(ctx context.Context, srv Server, conn *sconn.Conn) {
	p, err := NewProxyer(srv, conn)
	if err != nil {
		return
	}

	client := conn.RemoteAddr()
	err = p.Proxy(ctx)
	if err != nil {
		p.server.Logger().Error(err.Error(), errorx.TraceAttr(err), errorx.TraceAttr(err))
	} else {
		p.server.Logger().Info("close", "client", client.String())
	}
}

type IProxyer interface {
	Downlink(*packet.Packet, session.ID) error
	DecSession(session.Session)
}

type Proxyer struct {
	conn     *sconn.Conn
	server   Server
	sessions atomic.Int32
	start    time.Time

	ctr control.Server

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func NewProxyer(srv Server, conn *sconn.Conn) (*Proxyer, error) {
	var p = &Proxyer{
		conn:   conn,
		server: srv,
		start:  time.Now(),
	}
	p.srvCtx, p.srvCancel = context.WithCancel(context.Background())

	go p.uplinkService()
	p.keepalive()
	return p, nil
}

func (p *Proxyer) close(cause error) error {
	if p.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if p.ctr != nil {
			if err := p.ctr.Close(); err != nil {
				cause = err
			}
		}
		p.srvCancel()

		if p.conn != nil {
			if err := p.conn.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			if errorx.Temporary(cause) {
				p.server.Logger().Warn(cause.Error())
			} else {
				p.server.Logger().Error(cause.Error(), errorx.TraceAttr(cause))
			}
			p.closeErr.Store(&cause)
		}
		return cause
	}
	return *p.closeErr.Load()
}

func (p *Proxyer) Proxy(ctx context.Context) error {
	// todo: handle hadshake fail
	p.ctr = control.NewServer(p.conn.TCP(), controlImplPtr(p))

	err := p.ctr.Serve(ctx)
	return p.close(err)
}

func (p *Proxyer) uplinkService() error {
	var (
		pkt = packet.Make(p.server.MTU())
	)

	for {
		id, err := p.conn.Recv(p.srvCtx, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				p.server.Logger().LogAttrs(
					p.srvCtx, slog.LevelWarn, err.Error(),
					slog.String("client", p.conn.RemoteAddr().String()))
				continue
			} else {
				return p.close(err)
			}
		}

		var t header.Transport
		switch id.Proto {
		case header.TCPProtocolNumber:
			t = header.TCP(pkt.Bytes())
		case header.UDPProtocolNumber:
			t = header.UDP(pkt.Bytes())
		default:
			return p.close(errors.Errorf("not support protocol %d", id.Proto))
		}

		sess := session.Session{
			Src:   netip.AddrPortFrom(p.conn.RemoteAddr().Addr(), t.SourcePort()),
			Proto: id.Proto,
			Dst:   netip.AddrPortFrom(id.Remote, t.DestinationPort()),
		}
		err = p.server.Send(sess, pkt)
		if err != nil {
			if errors.Is(err, fatun.ErrNotRecord{}) {
				if err = p.server.AddSession(sess, (*serverImpl)(p)); err == nil {
					p.server.Logger().Info("add session", slog.String("session", sess.String()))

					p.incSession()
					err = p.server.Send(sess, pkt)
				}
			}

			if err != nil {
				p.server.Logger().LogAttrs(p.srvCtx, slog.LevelError, err.Error(),
					slog.String("clinet", p.conn.RemoteAddr().String()),
					errorx.TraceAttr(err))
			}
		}
	}
}

func (p *Proxyer) keepalive() {
	if p.sessions.Load() <= 0 && time.Since(p.start) > time.Second {
		p.close(fatun.ErrkeepaliveExceeded{})
	}
	time.AfterFunc(time.Minute*5, p.keepalive)
}
func (p *Proxyer) incSession() {
	p.sessions.Add(1)
}
func (p *Proxyer) decSession() {
	p.sessions.Add(-1)
}
