package control

import (
	"time"

	"github.com/lysShub/itun/cctx"
	"github.com/lysShub/itun/sconn"
)

const (
	client = "client"
	server = "server"
)

func NewClient(ctx cctx.CancelCtx, tcpHandshakeTimeout time.Duration, conn *sconn.Conn) (*Client, CtrInject) {
	us := newUserStack(ctx, client, conn)
	if ctx.Err() != nil {
		return nil, nil
	}

	tcp := connect(
		ctx, tcpHandshakeTimeout, us,
		conn.Raw().LocalAddr(), conn.Raw().RemoteAddr(),
	)
	if ctx.Err() != nil {
		return nil, nil
	}

	return newClient(ctx, tcp), us
}

func Serve(ctx cctx.CancelCtx, tcpHandshakeTimeout, initConfigTimeout time.Duration, conn *sconn.Conn, hdl CtrServer) CtrInject {
	us := newUserStack(ctx, server, conn)
	if ctx.Err() != nil {
		return nil
	}

	go func() {
		tcp := accept(
			ctx, tcpHandshakeTimeout, us,
			conn.Raw().LocalAddr(), conn.Raw().RemoteAddr(),
		)
		if ctx.Err() != nil {
			return
		}

		serve(ctx, initConfigTimeout, tcp, hdl)
	}()

	return us
}
