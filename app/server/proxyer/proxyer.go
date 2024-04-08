package proxyer

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
	ss "github.com/lysShub/itun/app/server/proxyer/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/sockit/packet"
)

type Server interface {
	Config() *app.Config
	Adapter() *adapter.Ports
	Endpoint(client netip.AddrPort) (ustack.Endpoint, error)
	Accept(ctx context.Context, client netip.AddrPort) (net.Conn, error)
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

type Proxyer struct {
	conn   *sconn.Conn
	srv    Server
	ep     ustack.Endpoint
	cfg    *app.Config
	logger *slog.Logger

	sessionMgr *ss.SessionMgr

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
			{Key: "src", Value: slog.StringValue(conn.RemoteAddrPort().String())},
		})),
	}
	p.sessionMgr = ss.NewSessionMgr(proxyerImplPtr(p))
	p.srvCtx, p.srvCancel = context.WithCancel(context.Background())

	var err error
	p.ep, err = p.srv.Endpoint(p.conn.RemoteAddrPort())
	if err != nil {
		return nil, err
	}

	p.logger.Info("accept")
	go p.uplinkService()
	go p.downlinkService()

	// todo: set timeout
	if tcp, err := srv.Accept(p.srvCtx, conn.RemoteAddrPort()); err != nil {
		return nil, err
	} else {
		p.ctr = control.NewServer(tcp, controlImplPtr(p))
	}

	return p, nil
}

func (p *Proxyer) close(cause error) error {
	if cause == nil {
		return *p.closeErr.Load()
	}

	if p.closeErr.CompareAndSwap(nil, &cause) {
		err := cause

		if p.ctr != nil {
			err = errorx.Join(
				err,
				p.ctr.Close(),
			)
		}
		p.srvCancel()
		if p.ep != nil {
			err = errorx.Join(
				err,
				p.ep.Close(),
			)
		}

		p.logger.Error(err.Error(), errorx.TraceAttr(err))
		return err
	} else {
		return *p.closeErr.Load()
	}
}

func (p *Proxyer) Proxy(ctx context.Context) error {
	err := p.ctr.Serve(ctx)
	return p.close(err)
}

func (p *Proxyer) downlinkService() error {
	var (
		tcp = packet.Make(0, p.cfg.MTU)
		err error
	)

	for {
		tcp.Sets(0, p.cfg.MTU)
		err = p.ep.Outbound(p.srvCtx, tcp)
		if err != nil {
			return p.close(err)
		}

		err = p.conn.Send(p.srvCtx, tcp, session.CtrSessID)
		if err != nil {
			return p.close(err)
		}
	}

}

func (p *Proxyer) uplinkService() error {
	var (
		tinyCnt int
		tcp     = packet.Make(0, p.cfg.MTU)
		id      session.ID
		s       *ss.Session
		err     error
	)

	for tinyCnt < 8 { // todo: from config
		tcp.Sets(0, p.cfg.MTU)
		id, err = p.conn.Recv(p.srvCtx, tcp)
		if err != nil {
			if errorx.IsTemporary(err) {
				tinyCnt++
				p.logger.Warn(err.Error())
			} else {
				return p.close(err)
			}
		}

		if id == session.CtrSessID {
			p.ep.Inbound(tcp)
		} else {
			s, err = p.sessionMgr.Get(id)
			if err != nil {
				p.logger.Warn(err.Error())
				continue
			}

			err = s.Send(tcp)
			if err != nil {
				return p.close(err)
			}
		}
	}
	return nil
}

func (p *Proxyer) downlink(pkt *packet.Packet, id session.ID) error {
	err := p.conn.Send(p.srvCtx, pkt, id)
	if err != nil {
		p.close(err)
	}
	return err
}
