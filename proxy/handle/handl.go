package handle

import (
	"fmt"
	"itun/pack"
	"itun/proxy/priority"
	"net"
	"net/netip"

	"github.com/lysShub/go-divert"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type handler struct {
	proxyConn net.Conn

	pack pack.Pack

	filter string
	hdl    divert.Handle
}

type Handler interface {
	Close() error
}

func Handle(pxyConn net.Conn, filter string) (Handler, error) {
	fmt.Println("handle", filter)

	var h = &handler{proxyConn: pxyConn, filter: filter}
	var err error
	h.hdl, err = divert.Open(filter, divert.LAYER_NETWORK, priority.HandlePriority, 0) // divert.FLAG_READ_ONLY
	if err != nil {
		panic(err)
		return nil, err
	}

	go h.handle()
	return h, nil
}

func (h *handler) handle() {
	var (
		b        = make([]byte, 1536)
		addr     divert.Address
		n        int
		err      error
		ipHdrLen uint8
		proto    divert.Proto
		dstAddr  netip.Addr
	)

	for {
		b = b[:cap(b)]
		if n, addr, err = h.hdl.Recv(b); err != nil {
			panic(err)
		}
		fmt.Println("handle recv")

		b = b[:n]
		if addr.IPv6() {
			const ipv6HdrLen = 40
			ipHdr := header.IPv6(b)
			ipHdrLen = ipv6HdrLen
			proto = divert.Proto(ipHdr.NextHeader())

			a := []byte(ipHdr.DestinationAddress())
			dstAddr = netip.AddrFrom16(([16]byte)(a))
		} else {
			ipHdr := header.IPv4(b)
			ipHdrLen = ipHdr.HeaderLength()
			proto = divert.Proto(ipHdr.Protocol())

			a := []byte(ipHdr.DestinationAddress())
			dstAddr = netip.AddrFrom4([4]byte(a))
		}

		_, err = h.proxyConn.Write(h.pack.Encode(b[ipHdrLen:], uint8(proto), dstAddr))
		if err != nil {
			panic(err)
		}
	}
}

func (h *handler) Close() error {
	return nil
}
