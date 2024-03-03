package link

import "gvisor.dev/gvisor/pkg/tcpip/header"

const FlagFinRst header.TCPFlags = header.TCPFlagFin | header.TCPFlagRst

func HandleTCPHdr(ip [][]byte, call func(hdr header.TCP) (update bool)) {
	if len(ip) == 0 {
		return
	} else if len(ip) == 1 {
		switch header.IPVersion(ip[0]) {
		case 4:
			ip := header.IPv4(ip[0])
			call(ip.Payload())
		case 6:
			ip := header.IPv6(ip[0])
			call(ip.Payload())
		}
	}

	handleTCPHdrSlices(ip, call)
}

func handleTCPHdrSlices(ip [][]byte, call func(header.TCP) bool) {
	var ipLen int
	switch header.IPVersion(ip[0]) {
	case 4:
		ipLen = int(header.IPv4(ip[0]).HeaderLength())
	case 6:
		ipLen = header.IPv6MinimumSize
	default:
		return
	}

	i1, i2 := slicesIdx(ip, ipLen)
	if i1 < 0 {
		return
	}

	var tcphdr = make(header.TCP, ipLen)
	if slicesMerge(ip, i1, i2, tcphdr) != len(tcphdr) {
		return
	}

	update := call(tcphdr)

	if update {
		slicesSplit(ip, i1, i2, tcphdr)
	}
}

func slicesIdx(ss [][]byte, idx int) (idx1, idx2 int) {
	for I := 0; idx1 < len(ss); idx1++ {
		I += len(ss[idx1])
		if I > idx {
			idx2 = len(ss[idx1]) - (I - idx)
			return
		}
	}
	return -1, -1
}

func slicesMerge(src [][]byte, i1, i2 int, dst []byte) int {
	n := copy(dst[0:], src[i1][i2:])

	for i := i1 + 1; i < len(src) && n < len(dst); i++ {
		n += copy(dst[n:], src[i])
	}
	return n
}

func slicesSplit(dst [][]byte, i1, i2 int, src []byte) int {
	n := copy(dst[i1][i2:], src[0:])

	for i := i1 + 1; i < len(dst) && n < len(src); i++ {
		n += copy(dst[i], src[n:])
	}
	return n
}
