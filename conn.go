package itun

import (
	"context"
	"net"
)

type Conn interface {
	net.Conn
	context.Context
}
