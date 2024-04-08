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
	"github.com/lysShub/sockit/packet"

	"github.com/pkg/errors"
)

type Client struct {
	cfg    *app.Config
	logger *slog.Logger

	conn *sconn.Conn

	sessMgr *cs.SessionMgr

	ctr control.Client

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func NewClient(ctx context.Context, conn *sconn.Conn, cfg *app.Config) (*Client, error) {
	var c = &Client{
		cfg: cfg,
		logger: slog.New(cfg.Logger.WithGroup("client").WithAttrs([]slog.Attr{
			{Key: "local", Value: slog.StringValue(conn.LocalAddrPort().String())},
			{Key: "proxyer", Value: slog.StringValue(conn.RemoteAddrPort().String())},
		})),
		conn:    conn,
		sessMgr: cs.NewSessionMgr(),
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	go c.downlinkService()
	c.ctr = control.NewClient(conn)

	// todo: init config
	if err := c.ctr.InitConfig(ctx, &control.Config{}); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func (c *Client) close(cause error) (err error) {
	if cause == nil {
		return *c.closeErr.Load()
	}

	if c.closeErr.CompareAndSwap(nil, &cause) {
		err := cause

		if c.ctr != nil {
			err = errorx.Join(err, c.ctr.Close())
		}
		c.srvCancel()
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

func (c *Client) downlinkService() error {
	var (
		tcp = packet.Make(64, c.cfg.MTU)
		id  session.ID
		s   *cs.Session
		err error
	)

	for {
		id, err = c.conn.Recv(c.srvCtx, tcp.SetHead(64))
		if err != nil {
			if errorx.IsTemporary(err) {
				c.logger.Warn(err.Error())
			} else {
				return c.close(err)
			}
		}

		s, err = c.sessMgr.Get(id)
		if err != nil {
			c.logger.Warn(err.Error())
			continue
		}

		err = s.Inject(tcp)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) uplink(ctx context.Context, pkt *packet.Packet, id session.ID) error {
	return c.conn.Send(ctx, pkt, id)
}

func (c *Client) AddSession(ctx context.Context, s capture.Session) error {
	self := session.Session{
		Src:   c.conn.LocalAddrPort(),
		Proto: itun.TCP,
		Dst:   c.conn.RemoteAddrPort(),
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
