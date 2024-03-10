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
	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/config"
	"github.com/lysShub/itun/control"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	pkge "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	config.Config

	MTU  uint16
	IPv6 bool
}

type Client struct {
	ctx     cctx.CancelCtx
	cfg     *Config
	raw     *itun.RawConn
	logger  *slog.Logger
	capture capture.Capture

	self   session.Session
	closed atomic.Bool

	sessionMgr *cs.SessionMgr
	stack      ustack.Ustack

	pseudoSum1 uint16
	seq, ack   uint32
	ipstack    *relraw.IPStack

	inited atomic.Bool

	fake   *faketcp.FakeTCP
	crypto *crypto.TCP
	ctr    control.Client
}

func NewClient(parentCtx context.Context, raw relraw.RawConn, capture capture.Capture, cfg *Config) (*Client, error) {
	log := cfg.Log
	if log == nil {
		log = slog.NewJSONHandler(os.Stdout, nil)
	}

	var c = &Client{
		ctx: cctx.WithContext(parentCtx),
		cfg: cfg,
		raw: itun.WrapRawConn(raw, cfg.MTU),
		logger: slog.New(log.WithGroup("proxy").WithAttrs([]slog.Attr{
			{Key: "src", Value: slog.StringValue(raw.LocalAddrPort().String())},
		})),
		capture: capture,

		self: session.Session{
			Src:   raw.LocalAddrPort(),
			Proto: itun.TCP,
			Dst:   raw.RemoteAddrPort(),
		},

		sessionMgr: cs.NewSessionMgr(),
	}
	c.pseudoSum1 = header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		c.raw.LocalAddr().Addr, c.raw.RemoteAddr().Addr,
		0,
	)

	var err error
	if c.stack, err = ustack.NewUstack(
		c.raw.LocalAddrPort(),
		c.raw.MTU(),
	); err != nil {
		return nil, c.Close(err)
	}

	if c.ipstack, err = relraw.NewIPStack(
		c.raw.LocalAddrPort().Addr(), c.raw.RemoteAddrPort().Addr(),
		header.TCPProtocolNumber,
	); err != nil {
		return nil, c.Close(err)
	}

	return c, c.handshake()
}

func (c *Client) handshake() error {
	go c.downlinkService()
	go c.uplinkService()

	tcp, err := gonet.DialTCPWithBind(
		c.ctx, c.stack,
		c.raw.LocalAddrPort(), c.raw.RemoteAddrPort(),
		header.IPv4ProtocolNumber,
	)
	if err != nil {
		return c.Close(err)
	}

	err = c.cfg.PrevPackets.Client(c.ctx, tcp)
	if err != nil {
		return c.Close(err)
	}

	key, err := c.cfg.Config.SwapKey.SecretKey(c.ctx, tcp)
	if err != nil {
		return c.Close(err)
	}
	c.crypto, err = crypto.NewTCP(key, c.pseudoSum1)
	if err != nil {
		return c.Close(err)
	}
	c.fake = faketcp.NewFakeTCP(
		c.raw.LocalAddrPort().Port(),
		c.raw.RemoteAddrPort().Port(),
		c.seq, c.ack,
		&c.pseudoSum1,
	)

	c.inited.CompareAndSwap(false, true)

	c.ctr = control.NewClient(tcp)

	if _, err = c.ctr.IPv6(c.ctx); err != nil {
		return c.Close(err)
	}
	if err = c.ctr.EndConfig(c.ctx); err != nil {
		return c.Close(err)
	}

	return nil
}

func (c *Client) downlinkService() {
	mtu := c.raw.MTU()
	var tcp = relraw.NewPacket(0, mtu)

	for {
		tcp.Sets(0, mtu)

		err := c.raw.ReadCtx(c.ctx, tcp)
		if err != nil {
			c.Close(err)
			return
		}

		if faketcp.IsFakeTCP(tcp.Data()) {
			// if !c.inited.Load() {
			//     todo: maybe attack
			// }

			err = c.crypto.Decrypt(tcp)
			if err != nil {
				c.logger.Warn(err.Error(), app.TraceAttr(err))
				continue
			}

			c.fake.RecvStrip(tcp)

			id := session.GetID(tcp)
			if id == session.CtrSessID {
				c.ipstack.AttachInbound(tcp)
				if debug.Debug() {
					test.ValidIP(test.T(), tcp.Data())
				}

				c.stack.Inbound(tcp)
			} else {
				s, err := c.sessionMgr.Get(id)
				if err != nil {
					c.logger.Warn(err.Error(), app.TraceAttr(err))
					continue
				}

				s.Inject(tcp)
			}
		} else {
			c.ack = max(c.ack, header.TCP(tcp.Data()).AckNumber())

			c.ipstack.AttachInbound(tcp)
			if debug.Debug() {
				test.ValidIP(test.T(), tcp.Data())
			}

			c.stack.Inbound(tcp)
		}
	}
}

func (c *Client) uplinkService() {
	mtu := c.raw.MTU()
	var pkt = relraw.NewPacket(0, mtu)

	var err error
	for {
		pkt.Sets(0, mtu)
		err = c.stack.Outbound(c.ctx, pkt)
		if err != nil {
			break
		}

		if c.inited.Load() {
			err = c.uplink(pkt, session.CtrSessID)
			if err != nil {
				break
			}
		} else {
			c.seq = max(c.seq, header.TCP(pkt.Data()).SequenceNumber())

			// recover to ip packet
			pkt.SetHead(0)
			if debug.Debug() {
				test.ValidIP(test.T(), pkt.Data())
			}

			_, err = c.raw.Write(pkt.Data())
			if err != nil {
				break
			}
		}
	}
	c.Close(err)
}

func (c *Client) uplink(pkt *relraw.Packet, id session.ID) error {
	if debug.Debug() {
		require.True(test.T(), c.inited.Load())
	}

	session.SetID(pkt, id)

	c.fake.SendAttach(pkt)

	c.crypto.Encrypt(pkt)

	err := c.raw.WriteCtx(context.Background(), pkt)
	return err
}

func (c *Client) AddProxy(s capture.Session) error {
	if c.self == s.Session() {
		return pkge.Errorf("can't proxy self %s", s)
	}

	resp, err := c.ctr.AddSession(c.ctx, s.Session())
	if err != nil {
		return err
	} else if resp.Err != nil {
		c.logger.Warn(resp.Err.Error(), "add session", s.String())
		return resp.Err
	} else {
		return c.sessionMgr.Add(sessionIpmlPtr(c), s, resp.ID)
	}
}

func (c *Client) Close(cause error) (err error) {
	if c.closed.CompareAndSwap(false, true) {
		err = cause
		if c.ctr != nil {
			err = app.Join(err,
				c.ctr.Close(),
			)
		}
		if c.stack != nil {
			err = app.Join(err,
				c.stack.Close(),
			)
		}
		if c.sessionMgr != nil {
			err = app.Join(err,
				c.sessionMgr.Close(),
			)
		}
		if c.raw != nil {
			err = app.Join(err,
				c.raw.Close(),
			)
		}

		c.ctx.Cancel(err)
		return err
	} else {
		<-c.ctx.Done()
		return c.ctx.Err()
	}
}
