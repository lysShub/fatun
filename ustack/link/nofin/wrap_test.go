package nofin

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Custom_TCP_Flag(t *testing.T) {
	var buildTCP = func(fin bool) header.TCP {
		const PseudoChecksum = 0

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
		tcphdr.SetChecksum(^checksum.Checksum(tcphdr, PseudoChecksum))
		return tcphdr
	}
	var validChecksum = func(tcphdr header.TCP) bool {
		act := ^tcphdr.Checksum()

		tcphdr.SetChecksum(0)
		exp := tcphdr.CalculateChecksum(0)
		ok := act == exp

		tcphdr.SetChecksum(^act)
		return ok
	}

	t.Run("not-need-update-checksum", func(t *testing.T) {
		tcphdr := buildTCP(true)
		require.True(t, validChecksum(tcphdr))

		EncodeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.True(t, isCustomFIN(tcphdr))
		require.False(t, tcphdr.Flags().Contains(header.TCPFlagFin))

		DecodeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.True(t, tcphdr.Flags().Contains(header.TCPFlagFin))
	})

	t.Run("encode-not-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		require.True(t, validChecksum(tcphdr))

		EncodeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.False(t, isCustomFIN(tcphdr))
	})

	t.Run("decode-not-custom-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		require.True(t, validChecksum(tcphdr))

		DecodeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.False(t, isCustomFIN(tcphdr))
	})

	t.Run("checksum-delta", func(t *testing.T) {
		var old = []byte{0b10, 0b101}
		var new = []byte{0b11, 0b100}

		act := checksum.Checksum(old, 0)
		act = checksum.Combine(act, checksumDelta)

		exp := checksum.Checksum(new, 0)

		require.Equal(t, act, exp)
	})
}
