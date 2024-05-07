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
	MaxRecvBuffSize() int
	Logger() *slog.Logger
	AddSession(sess session.Session, pxy Proxyer) error
	Send(sess session.Session, pkt *packet.Packet) error
	Close(client netip.AddrPort)
}

type Proxyer interface {
	DelSession(session.Session)
	Downlink(*packet.Packet, session.ID) error
	Closed() bool
}

func ProxyAndServe(ctx context.Context, srv Server, conn *sconn.Conn) {
	p, err := New(srv, conn)
	if err != nil {
		return
	}

	client := conn.RemoteAddr()
	err = p.Proxy(ctx)
	if err != nil {
		p.server.Logger().Error(err.Error(), errorx.Trace(err), errorx.Trace(err))
	} else {
		p.server.Logger().Info("close", "client", client.String())
	}
}

type Proxy struct {
	conn     *sconn.Conn
	server   Server
	sessions atomic.Int32
	start    time.Time

	ctr control.Server

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func New(srv Server, conn *sconn.Conn) (*Proxy, error) {
	var p = &Proxy{
		conn:   conn,
		server: srv,
		start:  time.Now(),
	}
	p.srvCtx, p.srvCancel = context.WithCancel(context.Background())

	go p.uplinkService()
	p.keepalive()
	return p, nil
}

func (p *Proxy) close(cause error) error {
	if p.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if p.server != nil {
			p.server.Close(p.conn.RemoteAddr())
		}

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
				p.server.Logger().Warn(cause.Error(), slog.String("client", p.conn.RemoteAddr().String()))
			} else {
				p.server.Logger().Error(cause.Error(), slog.String("client", p.conn.RemoteAddr().String()), errorx.Trace(cause))
			}
			p.closeErr.Store(&cause)
		}
		return cause
	}
	return *p.closeErr.Load()
}

func (p *Proxy) Proxy(ctx context.Context) error {
	tcp, err := p.conn.TCP(ctx)
	if err != nil {
		return p.close(err)
	}

	p.ctr = control.NewServer(tcp, controlImplPtr(p))
	err = p.ctr.Serve(ctx)
	return p.close(err)
}

func (p *Proxy) uplinkService() error {
	var (
		pkt = packet.Make(p.server.MaxRecvBuffSize())
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
					errorx.Trace(err))
			}
		}
	}
}

func (p *Proxy) keepalive() {
	if p.sessions.Load() <= 0 && time.Since(p.start) > time.Second {
		p.close(fatun.ErrKeepaliveExceeded{})
	}
	time.AfterFunc(time.Minute*5, p.keepalive)
}
func (p *Proxy) incSession() {
	p.sessions.Add(1)
}
func (p *Proxy) decSession() {
	p.sessions.Add(-1)
}
