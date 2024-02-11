package crypto

import (
	crand "crypto/rand"
	"math/rand"
	"testing"
	"unsafe"

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

	t.Run("without-payload", func(t *testing.T) {
		var key [16]byte
		crand.Read(key[:])
		var msg = []byte{}

		c, err := NewTCPCrypt(key)
		require.NoError(t, err)

		var pt = make(header.TCP, header.TCPMinimumSize+len(msg))
		withHdr(pt)
		copy(pt.Payload(), msg)

		ct := c.Encrypt(pt)
		require.Equal(t, len(pt)+Bytes, len(ct))
		require.Equal(t, ct[:ct.DataOffset()], pt[:pt.DataOffset()])

		pt2, err := c.Decrypt(ct)
		require.NoError(t, err)
		require.Equal(t, pt, pt2)
	})

	t.Run("with-payload", func(t *testing.T) {
		var key [16]byte
		crand.Read(key[:])
		var msg = []byte("abcedf")

		c, err := NewTCPCrypt(key)
		require.NoError(t, err)

		var pt = make(header.TCP, header.TCPMinimumSize+len(msg))
		withHdr(pt)
		copy(pt.Payload(), msg)

		ct := c.Encrypt(pt)
		require.Equal(t, len(pt)+Bytes, len(ct))
		require.Equal(t, ct[:ct.DataOffset()], pt[:pt.DataOffset()])

		pt2, err := c.Decrypt(ct)
		require.NoError(t, err)
		require.Equal(t, pt, pt2)
	})

	t.Run("with-pre-alloc-cap", func(t *testing.T) {
		var key [16]byte
		crand.Read(key[:])
		var msg = []byte("abcedf")

		c, err := NewTCPCrypt(key)
		require.NoError(t, err)

		var pt = make(header.TCP, header.TCPMinimumSize+len(msg), header.TCPMinimumSize+len(msg)+Bytes)
		withHdr(pt)
		copy(pt.Payload(), msg)

		ct := c.Encrypt(pt)
		require.Equal(t, len(pt)+Bytes, len(ct))
		require.Equal(t, ct[:ct.DataOffset()], pt[:pt.DataOffset()])

		pt2, err := c.Decrypt(ct)
		require.NoError(t, err)
		require.Equal(t, pt, pt2)
		require.Equal(t, unsafe.Pointer(&pt[0]), unsafe.Pointer(&pt2[0]))
	})

	t.Run("checkout", func(t *testing.T) {
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

		ct := c.EncryptChecksum(pt, pseudoSum1)
		require.Equal(t, len(pt)+Bytes, len(ct))
		require.Equal(t, ct.SequenceNumber(), pt.SequenceNumber())
		ok1 := ct.IsChecksumValid(
			src, dst,
			checksum.Checksum(ct.Payload(), 0),
			uint16(len(ct.Payload())),
		)
		require.True(t, ok1)

		pt2, err := c.DecryptChecksum(ct, pseudoSum1)
		require.NoError(t, err)
		require.Equal(t, unsafe.Pointer(&pt[0]), unsafe.Pointer(&pt2[0]))
		ok2 := pt2.IsChecksumValid(
			src, dst,
			checksum.Checksum(pt2.Payload(), 0),
			uint16(len(pt2.Payload())),
		)
		require.True(t, ok2)
	})
}

func Benchmark_Encrypt_PreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480, 1480+Bytes)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.Encrypt(pt)
	}
}

func Benchmark_EncryptChecksum_PreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480, 1480+Bytes)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.EncryptChecksum(pt, 0)
	}
}

func Benchmark_Encrypt_NotPreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.Encrypt(pt)
	}
}

func Benchmark_EncryptChecksum_NotPreAlloc(b *testing.B) {
	var pt = make(header.TCP, 1480)
	withHdr(pt)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		c.EncryptChecksum(pt, 0)
	}
}

func Benchmark_Decrypt(b *testing.B) {
	var pt = make(header.TCP, 1480)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)
	data := c.Encrypt(withHdr(make(header.TCP, 1480)))

	var ct = make([]byte, len(data))

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		copy(ct[0:], data)
		c.Decrypt(ct)
	}
}

func Benchmark_DecryptChecksum(b *testing.B) {
	var pt = make(header.TCP, 1480)

	c, err := NewTCPCrypt([16]byte{})
	require.NoError(b, err)
	data := c.EncryptChecksum(withHdr(make(header.TCP, 1480)), 0)

	var ct = make([]byte, len(data))

	for i := 0; i < b.N; i++ {
		b.SetBytes(int64(len(pt)))

		copy(ct[0:], data)
		c.DecryptChecksum(ct, 0)
	}
}
