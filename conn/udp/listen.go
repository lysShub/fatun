package udp

import (
	"net"
	"net/netip"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp/audp"
	"github.com/pkg/errors"
)

type Listener struct {
	addr   netip.AddrPort
	config *Config
	peer   conn.Peer

	l *audp.Listener
}

var _ conn.Listener = (*Listener)(nil)

func Listen[P conn.Peer](server string, config *Config) (conn.Listener, error) {
	var l = &Listener{config: config, peer: *new(P)}
	var err error

	if l.addr, err = resolve(server, true); err != nil {
		return nil, l.close(err)
	}

	l.l, err = audp.Listen(
		&net.UDPAddr{IP: l.addr.Addr().AsSlice(), Port: int(l.addr.Port())}, config.MaxRecvBuff,
	)
	if err != nil {
		return nil, l.close(err)
	}

	return l, nil
}

func (l *Listener) close(cause error) error {
	panic(cause)
}

func (l *Listener) Accept() (conn.Conn, error) {
	conn, err := l.l.Accept()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return newConn(conn, l.peer, server, l.config)
}

func (l *Listener) Addr() netip.AddrPort { return l.addr }
func (l *Listener) Close() error         { return l.close(nil) }
