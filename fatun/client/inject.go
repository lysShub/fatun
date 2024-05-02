//go:build windows
// +build windows

package client

import (
	"net/netip"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun/fatun"
	"github.com/lysShub/fatun/sconn"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Inject struct {
	addr netip.Addr

	handle     *divert.Handle
	injectAddr divert.Address
}

func NewInject(addr netip.Addr) (*Inject, error) {
	var i = &Inject{
		addr:       addr,
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
		return nil, errors.Errorf("invalid addr %s", addr.String())
	} else {
		i.injectAddr.Network().IfIdx = e.Interface
	}

	return i, nil
}

func (i *Inject) Inject(pkt *packet.Packet, id session.ID) error {
	if id.Proto == header.TCPProtocolNumber {
		fatun.UpdateMSS(pkt.Bytes(), -sconn.Overhead)
	}

	fields := header.IPv4Fields{
		TOS:            0,
		TotalLength:    header.IPv4MinimumSize + uint16(pkt.Data()),
		ID:             0,
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

	// clinet will re-calcuate checksum always.
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
