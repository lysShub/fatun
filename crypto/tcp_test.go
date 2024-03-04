package crypto_test

import (
	"context"
	"io"
	"math/rand"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/itun"
	"github.com/lysShub/itun/crypto"
	"github.com/lysShub/itun/ustack"
	"github.com/lysShub/itun/ustack/gonet"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/lysShub/relraw/test/debug"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func buildTCP(t require.TestingT, msgSize int, prevAlloc bool) (*relraw.Packet, uint16) {
	var (
		src        = tcpip.AddrFrom4([4]byte{1, 2, 3, 4})
		dst        = tcpip.AddrFrom4([4]byte{5, 6, 7, 8})
		pseudoSum1 = header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			src, dst,
			0,
		)
	)

	tail := 0
	if prevAlloc {
		tail = 64
	}
	p := relraw.NewPacket(0, header.TCPMinimumSize+msgSize, tail)

	tcp := header.TCP(p.Data())
	tcp.Encode(&header.TCPFields{
		SrcPort:       19986,
		DstPort:       8080,
		SeqNum:        rand.Uint32(),
		AckNum:        rand.Uint32(),
		DataOffset:    header.TCPMinimumSize,
		Flags:         header.TCPFlagAck | header.TCPFlagPsh,
		WindowSize:    uint16(rand.Uint32()),
		Checksum:      0,
		UrgentPointer: 0,
	})
	msg := tcp.Payload()
	for i := range msg {
		msg[i] = byte(i)
	}
	sum := checksum.Checksum(tcp, checksum.Combine(pseudoSum1, uint16(len(tcp))))
	tcp.SetChecksum(^sum)

	test.ValidTCP(t, tcp, pseudoSum1)
	return p, pseudoSum1
}

func Test_TCP_Crypto(t *testing.T) {
	var (
		key     = [crypto.Bytes]byte{0: 1}
		msgSize = 5
	)

	p, pseudoSum1 := buildTCP(t, msgSize, false)

	c, err := crypto.NewTCP(key, pseudoSum1)
	require.NoError(t, err)

	c.Encrypt(p)
	test.ValidTCP(t, p.Data(), pseudoSum1)

	err = c.Decrypt(p)
	require.NoError(t, err)
	test.ValidTCP(t, p.Data(), pseudoSum1)

	msg := header.TCP(p.Data()).Payload()
	require.Equal(t, []byte{0, 1, 2, 3, 4}, msg)
}

func Test_Conn(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		saddr = netip.AddrPortFrom(test.LocIP(), test.RandPort())
		seed  = time.Now().UnixNano()
		r     = rand.New(rand.NewSource(seed))

		pseudoSum1 = header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			test.Address(caddr.Addr()),
			test.Address(saddr.Addr()),
			0,
		)
	)
	t.Log("seed", seed)
	c, s := test.NewMockRaw(
		t, header.TCPProtocolNumber,
		caddr, saddr,
		test.ValidAddr, test.ValidChecksum,
	)

	go func() {
		st, err := ustack.NewUstack(saddr, 1536)
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(s, 1536), pseudoSum1)

		l, err := gonet.ListenTCP(st, saddr, header.IPv4ProtocolNumber)
		require.NoError(t, err)
		defer l.Close()

		conn, err := l.Accept(context.Background())
		require.NoError(t, err)
		defer conn.Close()

		_, err = io.Copy(conn, conn)
		require.NoError(t, err)
	}()

	{ // client
		st, err := ustack.NewUstack(saddr, 1536)
		require.NoError(t, err)
		UnicomStackAndRaw(t, st, itun.WrapRawConn(c, 1536), pseudoSum1)

		conn, err := gonet.DialTCPWithBind(
			context.Background(), st,
			caddr, saddr,
			header.IPv4ProtocolNumber,
		)
		require.NoError(t, err)
		defer conn.Close()

		test.ValidPingPongConn(t, r, conn, 0xffff)
	}
}

/*

cpu: Intel(R) Xeon(R) CPU E5-1650 v4 @ 3.60GHz
Benchmark_Encrypt_PrevAlloc-12      	 1719616	       694.2 ns/op	2160.66 MB/s	       0 B/op	       0 allocs/op
Benchmark_Encrypt_NotPreAlloc-12    	 1000000	      1018 ns/op	1473.46 MB/s	    1568 B/op	       2 allocs/op
Benchmark_Decrypt-12                	 1885424	       638.7 ns/op	2373.47 MB/s	       0 B/op	       0 allocs/op
PASS

*/

const packetLen = 1480

func Benchmark_Encrypt_PrevAlloc(b *testing.B) {
	if debug.Debug() {
		b.Skip("debug mode")
	}

	var p, pseudoSum1 = buildTCP(b, packetLen, true)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)

	var pt = relraw.NewPacket(p.Head(), p.Len(), p.Tail())
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(p.Len()))

		pt.Sets(p.Head(), p.Len())
		copy(pt.Data(), p.Data())

		c.Encrypt(pt)
	}
}

func Benchmark_Encrypt_NotPreAlloc(b *testing.B) {
	if debug.Debug() {
		b.Skip("debug mode")
	}

	var p, pseudoSum1 = buildTCP(b, packetLen, false)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)

	var raw = make([]byte, p.Len())
	var pt = relraw.ToPacket(0, raw)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(p.Len()))

		pt = relraw.ToPacket(0, raw[:p.Len():p.Len()])
		copy(pt.Data(), p.Data())

		c.Encrypt(pt)
	}

}

func Benchmark_Decrypt(b *testing.B) {
	if debug.Debug() {
		b.Skip("debug mode")
	}

	var p, pseudoSum1 = buildTCP(b, packetLen, true)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)
	c.Encrypt(p)

	var ct = relraw.NewPacket(p.Head(), p.Len(), p.Tail())
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(p.Len()))

		ct.Sets(p.Head(), p.Len())
		copy(ct.Data(), p.Data())

		c.Decrypt(ct)
	}
}
