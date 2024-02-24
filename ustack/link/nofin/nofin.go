package nofin

import "gvisor.dev/gvisor/pkg/tcpip/link/channel"

// implement link.LinkEndpoint, the link endpoint can close
// tcp connection without FIN flag, replace by 104 bit
func New(size int, mtu uint32) *Endpoint {
	return &Endpoint{
		Endpoint: channel.New(size, mtu, ""),
	}
}
