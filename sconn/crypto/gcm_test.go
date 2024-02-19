package crypto

import (
	crand "crypto/rand"
	"math/rand"
	"testing"
	"unsafe"

	"github.com/lysShub/relraw"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func withHdr(b header.TCP) header.TCP {
	b.Encode(&header.TCPFields{
		SrcPort:    19986,
		DstPort:    8080,
		SeqNum:     rand.Uint32(),
		AckNum:     rand.Uint32(),
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagAck | header.TCPFlagPsh,
		WindowSize: uint16(rand.Uint32()),
		Checksum:   0,
	})
	return b
}

func Test_TCP_Crypter(t *testing.T) {

	t.Run("crypter", func(t *testing.T) {
		var key [16]byte
		crand.Read(key[:])

		c, err := NewTCPCrypt(key)
		require.NoError(t, err)

		for _, msg := range [][]byte{
			nil,
			{},
			[]byte("abcedf"),
		} {
			for _, pt := range []header.TCP{
				make(header.TCP, header.TCPMinimumSize+len(msg)),
				make(header.TCP, header.TCPMinimumSize+len(msg), header.TCPMinimumSize+len(msg)+Bytes),
			} {
				withHdr(pt)
				copy(pt.Payload(), msg)

				p := relraw.ToPacket(0, pt)
				c.Encrypt(p)
				require.Equal(t, len(pt)+Bytes, p.Len())
				ho := pt.DataOffset()
				require.Equal(t, []byte(pt[:ho]), p.Data()[:ho])

				err = c.Decrypt(p)
				require.NoError(t, err)
				require.Equal(t, []byte(pt), p.Data())
			}
		}
	})

	t.Run("checksum", func(t *testing.T) {
		var (
			key        [16]byte
			msg        = []byte("abcedf")
			src        = tcpip.AddrFrom4([4]byte{1, 2, 3, 4})
			dst        = tcpip.AddrFrom4([4]byte{5, 6, 7, 8})
			pseudoSum1 = header.PseudoHeaderChecksum(
				header.TCPProtocolNumber,
				src, dst,
				0,
			)
		)
		crand.Read(key[:])
		c, err := NewTCPCrypt(key)
		require.NoError(t, err)

		var pt = make(header.TCP, header.TCPMinimumSize+len(msg), header.TCPMinimumSize+len(msg)+Bytes)
		withHdr(pt)
		copy(pt.Payload(), msg)

		p := relraw.ToPacket(0, pt)
		c.EncryptChecksum(p, pseudoSum1)

		ct := header.TCP(p.Data())

		ok1 := ct.IsChecksumValid(
			src, dst,
			checksum.Checksum(ct.Payload(), 0),
			uint16(len(ct.Payload())),
		)
		require.True(t, ok1)

		err = c.DecryptChecksum(p, pseudoSum1)
		require.NoError(t, err)

		pt2 := header.TCP(p.Data())
		require.Equal(t, unsafe.Pointer(&pt[0]), unsafe.Pointer(&p.Data()[0]))
		ok2 := pt2.IsChecksumValid(
			src, dst,
			checksum.Checksum(pt2.Payload(), 0),
			uint16(len(pt2.Payload())),
		)
		require.True(t, ok2)
	})
}

/*

goos: linux
goarch: amd64
pkg: github.com/lysShub/itun/sconn/crypto
cpu: Intel(R) Xeon(R) CPU E5-1650 v4 @ 3.60GHz
Benchmark_Encrypt_PrevAlloc-12              	 2359156	       510.4 ns/op	2899.77 MB/s	       0 B/op	       0 allocs/op
Benchmark_EncryptChecksum_PrevAlloc-12      	 1872739	       628.0 ns/op	2356.54 MB/s	       0 B/op	       0 allocs/op
Benchmark_Encrypt_NotPreAlloc-12            	 1393096	       854.4 ns/op	1732.26 MB/s	    1568 B/op	       2 allocs/op
Benchmark_EncryptChecksum_NotPreAlloc-12    	 1226776	       980.8 ns/op	1508.96 MB/s	    1568 B/op	       2 allocs/op
Benchmark_Decrypt-12                        	 2487850	       481.8 ns/op	3071.57 MB/s	       0 B/op	       0 allocs/op
Benchmark_DecryptChecksum-12                	 2479730	       486.2 ns/op	3043.83 MB/s	       0 B/op	       0 allocs/op
PASS
ok  	github.com/lysShub/itun/sconn/crypto	11.195s

*/

func Benchmark_Encrypt_PrevAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480, 1480+Bytes)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	p := relraw.ToPacket(0, pt)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.Encrypt(p)
		p.SetLen(len(pt))
	}
}

func Benchmark_EncryptChecksum_PrevAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480, 1480+Bytes)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	p := relraw.ToPacket(0, pt)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.EncryptChecksum(p, 0)
		p.SetLen(len(pt))
	}
}

func Benchmark_Encrypt_NotPreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	p := relraw.ToPacket(0, pt)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.Encrypt(p)
		p = relraw.ToPacket(0, p.Data()[:len(pt):len(pt)])
	}
}

func Benchmark_EncryptChecksum_NotPreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	p := relraw.ToPacket(0, pt)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.EncryptChecksum(p, 0)

		p = relraw.ToPacket(0, p.Data()[:len(pt):len(pt)])
	}
}

func Benchmark_Decrypt(b *testing.B) {
	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	var data = relraw.NewPacket(0, 1480, 0)
	withHdr(data.Data())
	c.Encrypt(data)

	var ct = relraw.NewPacket(0, data.Len(), 0)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(ct.Len()))

		copy(ct.Data(), data.Data())
		c.Decrypt(ct)
	}
}

func Benchmark_DecryptChecksum(b *testing.B) {
	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	var data = relraw.NewPacket(0, 1480, 0)
	withHdr(data.Data())
	c.Encrypt(data)

	var ct = relraw.NewPacket(0, data.Len(), 0)
	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(ct.Len()))

		copy(ct.Data(), data.Data())
		c.DecryptChecksum(ct, 0)
	}
}
