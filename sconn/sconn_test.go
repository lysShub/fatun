package sconn_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/sconn"
	"github.com/lysShub/itun/session"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type tkClient struct{}

func (c *tkClient) Token() (tk []byte, key crypto.Key, err error) {
	return []byte("hello"), crypto.Key{1: 1}, nil
}

type tkServer struct{}

func (c *tkServer) Valid(tk []byte) (key crypto.Key, err error) {
	if string(tk) == "hello" {
		return crypto.Key{1: 1}, nil
	}
	return crypto.Key{}, errors.Errorf("invalid token")
}

var pps = sconn.PrevPackets{
	header.TCP("hello"),
	header.TCP("world"),
	header.TCP("abcdef"),
	header.TCP("xyz"),
}

func TestXxx(t *testing.T) {

	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		sid   = session.ID(123)
		msg   = "hellow world"
	)

	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum, //test.PacketLoss(0.1),
	)

	// server
	var srvCh = make(chan struct{})
	go func() {
		var cfg = sconn.Config{
			PrevPackets: pps,
			SwapKey:     &crypto.TokenServer{Valider: &tkServer{}},
			MTU:         1536,
		}

		l, err := sconn.NewListener(test.NewMockListener(t, s), &cfg)
		require.NoError(t, err)

		conn, err := l.Accept()
		require.NoError(t, err)

		var b = relraw.NewPacket(0, 1536)
		id, err := conn.Recv(context.Background(), b)
		require.NoError(t, err)
		require.Equal(t, sid, id)
		require.Equal(t, msg, string(b.Data()))
		close(srvCh)
	}()

	var cfg = sconn.Config{
		PrevPackets: pps,
		SwapKey:     &crypto.TokenClient{Tokener: &tkClient{}},
		MTU:         1536,
	}

	// wc, err := test.WrapPcap(c, "./test.pcap")
	// require.NoError(t, err)

	conn, err := sconn.Dial(c, &cfg)
	require.NoError(t, err)

	var b = relraw.NewPacket(0, 1536)
	copy(b.Data(), msg)
	b.SetLen(len(msg))

	err = conn.Send(context.Background(), b, sid)
	require.NoError(t, err)

	<-srvCh
}
