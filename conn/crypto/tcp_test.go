package crypto_test

import (
	"math/rand"
	"testing"

	"github.com/lysShub/fatun/conn/crypto"
	"github.com/lysShub/netkit/packet"

	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func buildTCP(t require.TestingT, msgSize int, prevAlloc bool) (*packet.Packet, uint16) {
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
	p := packet.Make(0, header.TCPMinimumSize+msgSize, tail)

	tcp := header.TCP(p.Bytes())
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

func Test_Tcp_Base(t *testing.T) {
	var (
		key = [crypto.Bytes]byte{0: 1}
	)

	for _, size := range []int{0, 1, 5, 16, 1024, 1480} {

		p, pseudoSum1 := buildTCP(t, size, true)

		c, err := crypto.NewTCP(key, pseudoSum1)
		require.NoError(t, err)

		c.Encrypt(p)
		test.ValidTCP(t, p.Bytes(), pseudoSum1)

		err = c.Decrypt(p)
		require.NoError(t, err)
		test.ValidTCP(t, p.Bytes(), pseudoSum1)

		msg := header.TCP(p.Bytes()).Payload()
		require.Equal(t, size, len(msg))
		for i, e := range msg {
			require.Equal(t, byte(i), e)
		}
	}
}

func Test_Tcp_NAT(t *testing.T) {
	// encrypt packet usually pass NAT gateway
	var (
		key = [crypto.Bytes]byte{0: 1}
	)

	p, pseudoSum1 := buildTCP(t, 16, true)

	c, err := crypto.NewTCP(key, pseudoSum1)
	require.NoError(t, err)

	c.Encrypt(p)
	test.ValidTCP(t, p.Bytes(), pseudoSum1)

	// change src/dest port
	header.TCP(p.Bytes()).SetSourcePortWithChecksumUpdate(1234)
	header.TCP(p.Bytes()).SetDestinationPortWithChecksumUpdate(1234)
	test.ValidTCP(t, p.Bytes(), pseudoSum1)

	err = c.Decrypt(p)
	require.NoError(t, err)
	test.ValidTCP(t, p.Bytes(), pseudoSum1)

	msg := header.TCP(p.Bytes()).Payload()
	require.Equal(t, 16, len(msg))
	for i, e := range msg {
		require.Equal(t, byte(i), e)
	}
}

/*

go test -benchmem  -bench .

cpu: Intel(R) Xeon(R) CPU E5-1650 v4 @ 3.60GHz
Benchmark_Encrypt_PrevAlloc-12           1764721               678.7 ns/op   2180.78 MB/s           0 B/op          0 allocs/op
Benchmark_Encrypt_NotPreAlloc-12         1198791              1013 ns/op     1461.44 MB/s        1536 B/op          1 allocs/op
Benchmark_Decrypt-12                     1948113               626.5 ns/op   2362.14 MB/s           0 B/op          0 allocs/op
Benchmark_Memcpy-12                     52166427                23.18 ns/op  63841.22 MB/s          0 B/op          0 allocs/op
Benchmark_MemAlloc-12                    3004122               391.1 ns/op   3784.30 MB/s        1536 B/op          1 allocs/op
PASS

*/

const packetLen = 1480

func Benchmark_Encrypt_PrevAlloc(b *testing.B) {
	var pt, pseudoSum1 = buildTCP(b, packetLen, true)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)

	var ct = packet.Make(0, pt.Data(), 16)
	for i := 0; i < b.N; i++ {
		b.SetBytes(packetLen)

		ct.SetData(0).Append(pt.Bytes())

		c.Encrypt(ct)
	}
}

func Benchmark_Encrypt_NotPreAlloc(b *testing.B) {
	var pt, pseudoSum1 = buildTCP(b, packetLen, false)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)

	for i := 0; i < b.N; i++ {
		b.SetBytes(packetLen)

		var ct = packet.Make(0, pt.Data(), 16)
		ct.SetData(0).Append(pt.Bytes())

		c.Encrypt(ct)
	}
}

func Benchmark_Decrypt(b *testing.B) {
	var ct, pseudoSum1 = buildTCP(b, packetLen, true)
	c, _ := crypto.NewTCP([16]byte{}, pseudoSum1)
	c.Encrypt(ct)

	var pt = packet.Make(0, ct.Data())
	for i := 0; i < b.N; i++ {
		b.SetBytes(packetLen)

		pt.SetData(0).Append(ct.Bytes())

		c.Decrypt(pt)
	}
}

func Benchmark_Memcpy(b *testing.B) {
	var pt, _ = buildTCP(b, packetLen, false)

	var ct = packet.Make(pt.Head(), pt.Data())
	for i := 0; i < b.N; i++ {
		b.SetBytes(packetLen)

		ct.SetData(0).Append(pt.Bytes())
	}
}

func Benchmark_MemAlloc(b *testing.B) {
	var pt, _ = buildTCP(b, packetLen, false)

	for i := 0; i < b.N; i++ {
		b.SetBytes(packetLen)

		var ct = packet.Make(0, pt.Data(), 0)
		ct.SetData(0)
	}
}
