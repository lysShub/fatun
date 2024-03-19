package client

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/app"
	"github.com/lysShub/itun/app/client/capture"
	cs "github.com/lysShub/itun/app/client/session"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/errorx"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/itun/ustack/link"
	"github.com/lysShub/rsocket"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	cfg    *app.Config
	logger *slog.Logger

	conn *sconn.Conn

	sessMgr *cs.SessionMgr

	ep  ustack.Endpoint
	ctr control.Client

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func NewClient(ctx context.Context, conn *sconn.Conn, cfg *app.Config) (*Client, error) {
	var c = &Client{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("client").WithAttrs([]slog.Attr{
			{Key: "local", Value: slog.StringValue(conn.LocalAddr().String())},
			{Key: "proxy", Value: slog.StringValue(conn.RemoteAddr().String())},
		})),
		conn:    conn,
		sessMgr: cs.NewSessionMgr(),
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	if stack, err := ustack.NewUstack(
		link.NewList(8, cfg.MTU),
		conn.LocalAddr().Addr(),
	); err != nil {
		return nil, c.close(err)
	} else {
		c.ep, err = ustack.ToEndpoint(
			stack, conn.LocalAddr().Port(),
			conn.RemoteAddr(),
		)
		if err != nil {
			return nil, c.close(err)
		}
	}

	go c.uplinkService()
	go c.downService()

	if tcp, err := gonet.DialTCPWithBind(
		ctx, c.ep.Stack(),
		conn.LocalAddr(), conn.RemoteAddr(),
		header.IPv4ProtocolNumber,
	); err != nil {
		return nil, c.close(err)
	} else {
		c.ctr = control.NewClient(tcp)
	}

	// todo: init config
	if err := c.ctr.InitConfig(ctx, &control.Config{}); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func (c *Client) close(cause error) (err error) {
	if cause == nil {
		cause = os.ErrClosed
	}

	if c.closeErr.CompareAndSwap(nil, &cause) {
		err := cause

		if c.ctr != nil {
			err = errorx.Join(err, c.ctr.Close())
		}
		c.srvCancel()
		if c.ep != nil {
			err = errorx.Join(err, c.ep.Close())
		}
		if c.sessMgr != nil {
			err = errorx.Join(err, c.sessMgr.Close())
		}
		if c.conn != nil {
			err = errorx.Join(err, c.conn.Close())
		}

		c.logger.Info("close", "cause", err.Error())

		c.closeErr.Store(&err)
		return err
	} else {
		return *c.closeErr.Load()
	}
}

func (c *Client) uplinkService() {
	var (
		tcp = rsocket.NewPacket(0, c.cfg.MTU)
		err error
	)

	for {
		tcp.Sets(0, c.cfg.MTU)
		err = c.ep.Outbound(c.srvCtx, tcp)
		if err != nil {
			break
		}

		err = c.uplink(c.srvCtx, tcp, session.CtrSessID)
		if err != nil {
			break
		}
	}
	c.close(err)
}

func (c *Client) downService() {
	var (
		tinyCnt int
		tcp     = rsocket.NewPacket(0, c.cfg.MTU)
		id      session.ID
		s       *cs.Session
		err     error
	)

	for tinyCnt < 8 { // todo: from config
		tcp.Sets(0, c.cfg.MTU)
		id, err = c.conn.Recv(c.srvCtx, tcp)
		if err != nil {
			if errorx.IsTemporary(err) {
				tinyCnt++
				c.logger.Warn(err.Error())
			} else {
				break
			}
		}

		if id == session.CtrSessID {
			c.ep.Inbound(tcp)
		} else {
			s, err = c.sessMgr.Get(id)
			if err != nil {
				c.logger.Warn(err.Error())
				continue
			}

			err = s.Inject(tcp)
			if err != nil {
				break
			}
		}
	}
	c.close(err)
}

func (c *Client) uplink(ctx context.Context, pkt *rsocket.Packet, id session.ID) error {
	return c.conn.Send(ctx, pkt, id)
}

func (c *Client) AddSession(ctx context.Context, s capture.Session) error {
	self := session.Session{
		Src:   c.conn.LocalAddr(),
		Proto: itun.TCP,
		Dst:   c.conn.RemoteAddr(),
	}
	if s.Session() == self {
		return errors.Errorf("can't proxy self %s", self.String())
	}

	resp, err := c.ctr.AddSession(ctx, s.Session())
	if err != nil {
		return err
	} else if resp.Err != nil {
		return resp.Err
	} else {
		return c.sessMgr.Add(sessionImplPtr(c), s, resp.ID)
	}
}

func (c *Client) Close() error {
	return c.close(os.ErrClosed)
}
