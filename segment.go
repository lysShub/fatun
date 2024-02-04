package itun

import "gvisor.dev/gvisor/pkg/tcpip/header"

func IsMgrSeg(tcp header.TCP) bool {
	const (
		offset = 12
		flag   = 0b10111111
	)
	// NS previous bit set 0 means MgrSeg(default)

	return tcp[offset]|flag == flag
}

// func
