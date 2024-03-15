package link

import (
	"math/rand"
	"testing"

	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Fake_TCP_FIN(t *testing.T) {
	const pseudoChecksum1 = 0
	var buildTCP = func(fin bool) header.TCP {
		var tcphdr = make(header.TCP, header.TCPMinimumSize)
		f := header.TCPFlagAck
		if fin {
			f |= header.TCPFlagFin
		}
		tcphdr.Encode(&header.TCPFields{
			SrcPort:    19986,
			DstPort:    8080,
			SeqNum:     rand.Uint32(),
			AckNum:     rand.Uint32(),
			DataOffset: header.TCPMinimumSize,
			Flags:      f,
			WindowSize: uint16(rand.Uint32()),
			Checksum:   0,
		})
		sum := checksum.Checksum(tcphdr, checksum.Combine(pseudoChecksum1, uint16(len(tcphdr))))
		tcphdr.SetChecksum(^sum)

		test.ValidTCP(t, tcphdr, pseudoChecksum1)
		return tcphdr
	}

	t.Run("checksum-delta", func(t *testing.T) {
		var old = []byte{0b10, 0b101}
		var new = []byte{0b11, 0b100}

		act := checksum.Checksum(old, 0)
		act = checksum.Combine(act, checksumDelta)

		exp := checksum.Checksum(new, 0)

		require.Equal(t, act, exp)
	})

	t.Run("not-need-update-checksum", func(t *testing.T) {
		tcphdr := buildTCP(true)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)

		encodeFakeFIN(tcphdr)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)
		require.True(t, IsFakeFIN(tcphdr))
		require.False(t, tcphdr.Flags().Contains(header.TCPFlagFin))

		decodeFakeFIN(tcphdr)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)
		require.True(t, tcphdr.Flags().Contains(header.TCPFlagFin))
	})

	t.Run("encode-not-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)

		encodeFakeFIN(tcphdr)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)
		require.False(t, IsFakeFIN(tcphdr))
	})

	t.Run("decode-not-custom-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)

		decodeFakeFIN(tcphdr)
		test.ValidTCP(t, tcphdr, pseudoChecksum1)
		require.False(t, IsFakeFIN(tcphdr))
	})

}
