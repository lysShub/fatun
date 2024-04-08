package proxyer

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/server/adapter"
	ss "github.com/lysShub/itun/app/server/proxyer/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/sockit/packet"
)

type Server interface {
	Config() *app.Config
	Adapter() *adapter.Ports
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

	p.logger.Info("accept")
	go p.uplinkService()
	p.ctr = control.NewServer(conn, controlImplPtr(p))

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

func (p *Proxyer) uplinkService() error {
	var (
		tcp = packet.Make(64, p.cfg.MTU)
		id  session.ID
		s   *ss.Session
		err error
	)

	for {
		id, err = p.conn.Recv(p.srvCtx, tcp.SetHead(64))
		if err != nil {
			if errorx.IsTemporary(err) {
				p.logger.Warn(err.Error())
			} else {
				return p.close(err)
			}
		}

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

func (p *Proxyer) downlink(pkt *packet.Packet, id session.ID) error {
	err := p.conn.Send(p.srvCtx, pkt, id)
	if err != nil {
		p.close(err)
	}
	return err
}
