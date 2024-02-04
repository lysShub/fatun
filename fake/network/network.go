/*
  gvisor/pkg/tcpip/network/ipv4 implemented

  it shift tcp FIN flag to tcp[13*8-1] bit
*/

package network

import (
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const (
	TCPCustomFlagFin            = 0b1
	TCPCustomFlagsOffset        = header.TCPDataOffset
	checksumDelta        uint16 = 255
)

func EnCustomFIN(tcphdr header.TCP) bool {
	newFlags := tcphdr.Flags()
	if newFlags.Contains(header.TCPFlagFin) {

		newFlags ^= header.TCPFlagFin
		if newFlags == 0 {
			panic("todo")
		}
		tcphdr.SetFlags(uint8(newFlags))
		tcphdr[TCPCustomFlagsOffset] = tcphdr[TCPCustomFlagsOffset] ^ TCPCustomFlagFin

		sum := ^tcphdr.Checksum()
		sum = checksum.Combine(sum, checksumDelta)
		tcphdr.SetChecksum(^sum)
		return true
	}
	return false
}

func isCustomFIN(tcphdr header.TCP) bool {
	return tcphdr[TCPCustomFlagsOffset]&TCPCustomFlagFin == TCPCustomFlagFin
}

func DeCustomFIN(tcphdr header.TCP) bool {
	if isCustomFIN(tcphdr) {
		tcphdr[TCPCustomFlagsOffset] = tcphdr[TCPCustomFlagsOffset] ^ TCPCustomFlagFin

		newFlags := uint8(tcphdr.Flags() | header.TCPFlagFin)
		tcphdr.SetFlags(newFlags)

		sum := ^tcphdr.Checksum()
		sum = checksum.Combine(sum, ^checksumDelta)
		tcphdr.SetChecksum(^sum)

		return true
	}
	return false
}
