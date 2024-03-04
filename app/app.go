package app

import (
	"context"

	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
)

type Recver interface {
	Recv(context.Context, *relraw.Packet) session.ID
	MTU() int
}

type Sender interface {
	Send(b *relraw.Packet, id session.ID)
	MTU() int
}
