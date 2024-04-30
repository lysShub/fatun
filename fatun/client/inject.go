//go:build windows
// +build windows

package client

import (
	"fmt"
	"math/rand"
	"net/netip"
	"strings"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/route"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Inject struct {
	addr netip.Addr
	ids  map[session.ID]uint16 // todo: reduce

	handle     *divert.Handle
	injectAddr divert.Address
}

func NewInject(addr netip.Addr) (*Inject, error) {
	var i = &Inject{
		addr:       addr,
		ids:        map[session.ID]uint16{},
		injectAddr: divert.Address{},
	}

	var err error
	i.handle, err = divert.Open("false", divert.Network, 0, divert.WriteOnly)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	t, err := route.GetTable()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if e := t.Match(addr); !e.Valid() {
		panic(fmt.Sprintf("invalid addr %s", addr.String()))
	} else {
		i.injectAddr.Network().IfIdx = e.Interface

		idx, err := getIfidxByAddr(addr)
		if err != nil {
			panic(err)
		} else if idx != e.Interface {
			panic("")
		}
	}

	return i, nil
}

func (i *Inject) Inject(pkt *packet.Packet, id session.ID) error {
	fields := header.IPv4Fields{
		TOS:            0,
		TotalLength:    header.IPv4MinimumSize + uint16(pkt.Data()),
		ID:             0, //  i.next(id),
		Flags:          0,
		FragmentOffset: 0,
		TTL:            64,
		Protocol:       uint8(id.Proto),
		Checksum:       0,
		SrcAddr:        tcpip.AddrFrom4(id.Remote.As4()),
		DstAddr:        tcpip.AddrFrom4(i.addr.As4()),
		Options:        nil,
	}

	ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
	ip.Encode(&fields)

	ip.SetChecksum(^ip.CalculateChecksum())
	sum := header.PseudoHeaderChecksum(id.Proto, ip.SourceAddress(), ip.DestinationAddress(), ip.PayloadLength())
	switch id.Proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(ip.Payload())
		tcp.SetChecksum(0)
		sum = checksum.Checksum(tcp, sum)
		tcp.SetChecksum(^sum)
	case header.UDPProtocolNumber:
		udp := header.UDP(ip.Payload())
		udp.SetChecksum(0)
		sum = checksum.Checksum(udp, sum)
		udp.SetChecksum(^sum)
	default:
		panic("")
	}

	if debug.Debug() {
		test.ValidIP(test.T(), ip)
	}

	_, err := i.handle.Send(ip, &i.injectAddr)
	return err
}

func (i *Inject) next(id session.ID) uint16 {
	v, has := i.ids[id]
	if !has {
		v = uint16(rand.Uint32())
	}
	i.ids[id] = v + 1
	return v + 1
}

func getIfidxByAddr(addr netip.Addr) (uint32, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return 0, errors.WithStack(err)
	}
	a := addr.String()
	for _, e := range ifs {
		for _, addr := range e.Addrs {
			if strings.HasPrefix(addr.Addr, a) {
				return uint32(e.Index), nil
			}
		}
	}
	return 0, errors.Errorf("not find adapter with address %s", a)
}
