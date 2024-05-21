package fatun

import (
	"context"
	"encoding/gob"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type Controller interface {
	Control(ctx context.Context, conn fatcp.Conn)
}

// DefaultController simaple ping control demo
type DefaultController struct {
	HandshakeTimeout time.Duration
	Logger           *slog.Logger
	marshal          *Marshal
}

func (c *DefaultController) Control(ctx context.Context, conn fatcp.Conn) {
	handshakeCtx, cancel := context.WithTimeout(ctx, c.HandshakeTimeout)
	defer cancel()
	tcp, err := conn.BuiltinTCP(handshakeCtx)
	if err != nil {
		conn.Close()
		return
	}
	defer conn.Close()

	c.marshal = NewMarshal(tcp)
	if conn.Role().Client() {
		for {
			err := c.marshal.Encode(Ping{Request: time.Now()})
			if err != nil {
				c.Logger.Error(err.Error(), errorx.Trace(err))
				return
			}

			var msg Message
			if err := c.marshal.Decode(&msg); err != nil {
				c.Logger.Error(err.Error(), errorx.Trace(err))
				return
			}
			ping := msg.(Ping)

			c.Logger.Info("ping", slog.String("uplink", ping.Response.Sub(ping.Request).String()),
				slog.String("downlink", time.Since(ping.Response).String()),
				slog.String("total", time.Since(ping.Request).String()),
			)
			time.Sleep(time.Second * 5)
		}

	} else {
		for {
			var msg Message
			if err := c.marshal.Decode(&msg); err != nil {
				c.Logger.Error(err.Error(), errorx.Trace(err))
				return
			}
			if msg.Kind() == KindPing {
				err := c.marshal.Encode(Ping{Request: msg.(Ping).Request, Response: time.Now()})
				if err != nil {
					c.Logger.Error(err.Error(), errorx.Trace(err))
					return
				}
			} else {
				c.Logger.Warn("not support control message", slog.String("kind", strconv.Itoa(int(msg.Kind()))))
			}
		}
	}
}

type Marshal struct {
	enc *gob.Encoder
	dec *gob.Decoder
}

func NewMarshal(tcp io.ReadWriter) *Marshal {

	return &Marshal{
		enc: gob.NewEncoder(tcp),
		dec: gob.NewDecoder(tcp),
	}
}

func (m *Marshal) Encode(msg Message) error {
	return errors.WithStack(m.enc.Encode(&msg))
}

func (m *Marshal) Decode(msg *Message) error {
	return errors.WithStack(m.dec.Decode(msg))
}

type Message interface {
	Kind() Kind
	Clone() Message
}

type Kind uint8

const (
	_ Kind = iota
	KindPing
)

type Ping struct {
	Request  time.Time
	Response time.Time
}

func (p Ping) Kind() Kind     { return KindPing }
func (p Ping) Clone() Message { return Ping{Request: p.Request, Response: p.Response} }
