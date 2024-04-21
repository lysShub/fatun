package client

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/app/client/capture"
	cs "github.com/lysShub/fatun/app/client/session"
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/packet"
	"gvisor.dev/gvisor/pkg/tcpip/header"

	"github.com/pkg/errors"
)

type Client struct {
	cfg    *app.Config
	logger *slog.Logger
	self   session.Session

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
			{Key: "local", Value: slog.StringValue(conn.LocalAddr().String())},
			{Key: "proxyer", Value: slog.StringValue(conn.RemoteAddr().String())},
		})),
		self: session.Session{
			Src:   conn.LocalAddr(),
			Proto: header.TCPProtocolNumber,
			Dst:   conn.RemoteAddr(),
		},
		conn:    conn,
		sessMgr: cs.NewSessionMgr(),
	}
	c.srvCtx, c.srvCancel = context.WithCancel(context.Background())

	go c.downlinkService()
	c.ctr = control.NewClient(conn.TCP())

	// todo: init config
	if err := c.ctr.InitConfig(ctx, &control.Config{}); err != nil {
		return nil, c.close(err)
	}
	return c, nil
}

func (c *Client) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if c.ctr != nil {
			if err := c.ctr.Close(); err != nil {
				cause = err
			}
		}
		c.srvCancel()
		if c.sessMgr != nil {
			if err := c.sessMgr.Close(); err != nil {
				cause = err
			}
		}
		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			c.closeErr.Store(&cause)
			c.logger.Info("close", "cause", cause.Error(), errorx.TraceAttr(errors.WithStack(cause)))
		}
		return cause
	}
	return *c.closeErr.Load()
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
			if errorx.Temporary(err) {
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
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	if s.ID() == c.self {
		return errors.Errorf("can't proxy self %s", c.self.String())
	}

	resp, err := c.ctr.AddSession(ctx, s.ID())
	if err != nil {
		if c.closeErr.Load() != nil {
			return *c.closeErr.Load()
		}
		return err
	} else if resp.Err != "" {
		return errors.New(resp.Err)
	} else {
		return c.sessMgr.Add(sessionImplPtr(c), s, resp.ID)
	}
}

func (c *Client) Close() error { return c.close(nil) }
