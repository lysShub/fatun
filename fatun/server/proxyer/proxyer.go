package proxyer

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"os"
	"sync/atomic"

	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Server interface {
	Config() *fatun.Config
	// Adapter() *adapter.Ports
	AddSession(sess session.Session, pxy IProxyer) error
	Send(sess session.Session, pkt *packet.Packet) error
}

func Proxy(ctx context.Context, srv Server, conn *sconn.Conn) {
	p, err := NewProxyer(srv, conn)
	if err != nil {
		return
	}

	err = p.Proxy(ctx)
	if err != nil {
		p.logger.Error(err.Error(), errorx.TraceAttr(err))
	} else {
		p.logger.Info("close")
	}
}

type IProxyer interface {
	Downlink(*packet.Packet, session.ID) error
	DecRefs()
}

type Proxyer struct {
	conn   *sconn.Conn
	srv    Server
	cfg    *fatun.Config
	logger *slog.Logger
	refs   atomic.Int32

	ctr control.Server

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func NewProxyer(srv Server, conn *sconn.Conn) (*Proxyer, error) {
	cfg := srv.Config()
	var p = &Proxyer{
		conn: conn,
		srv:  srv,
		cfg:  cfg,
		logger: slog.New(cfg.Logger.WithGroup("proxyer").WithAttrs([]slog.Attr{
			{Key: "src", Value: slog.StringValue(conn.RemoteAddr().String())},
		})),
	}
	p.srvCtx, p.srvCancel = context.WithCancel(context.Background())

	// todo: 避免Proxyer没有任何数据包传输导致的协程泄露问题
	// todo: 现在也不会超时释放proxyer（比如只有一个proxyer时）
	// sess := session.Session{Src: conn.RemoteAddr(), Proto: header.TCPProtocolNumber}
	// srv.AddSession(sess, (*serverImpl)(p))

	go p.uplinkService()
	p.ctr = control.NewServer(conn.TCP(), controlImplPtr(p))
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

		if cause != nil {
			if errorx.Temporary(cause) {
				p.logger.Warn(cause.Error())
			} else {
				p.logger.Error(cause.Error(), errorx.TraceAttr(cause))
			}

			p.closeErr.Store(&cause)
		}
		return cause
	}
	return *p.closeErr.Load()
}

func (p *Proxyer) Proxy(ctx context.Context) error {
	err := p.ctr.Serve(ctx)
	return p.close(err)
}

// todo: add bandwidth limit
func (p *Proxyer) uplinkService() error {
	var (
		pkt = packet.Make(p.cfg.MTU)
	)

	for {
		id, err := p.conn.Recv(p.srvCtx, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				p.logger.Warn(err.Error())
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
			panic("")
		}

		sess := session.Session{
			Src:   netip.AddrPortFrom(p.conn.RemoteAddr().Addr(), t.SourcePort()),
			Proto: id.Proto,
			Dst:   netip.AddrPortFrom(id.Remote, t.DestinationPort()),
		}
		err = p.srv.Send(sess, pkt)
		if err != nil {
			if errors.Is(err, fatun.ErrNotRecord{}) {
				if err := p.srv.AddSession(sess, (*serverImpl)(p)); err != nil {
					p.logger.Warn(err.Error(), errorx.TraceAttr(err))
				}
				p.incRefs()

				err = p.srv.Send(sess, pkt)
			}

			if err != nil {
				p.logger.Warn(err.Error(), errorx.TraceAttr(err))
			}
		}
	}
}

func (p *Proxyer) incRefs() {
	p.refs.Add(1)
}
func (p *Proxyer) decRefs() {
	if p.refs.Add(-1) <= 0 {
		p.close(fatun.ErrkeepaliveExceeded{})
	}
}
