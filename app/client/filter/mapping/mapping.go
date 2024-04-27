package mapping

import (
	"encoding/hex"
	"fmt"
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// mapping of socket <==> process
type Mapping interface {
	Close() error

	Name(Endpoint) (string, error)
	Pid(Endpoint) (uint32, error)

	Pids() []uint32
	Names() []string
}

type LocalAddr struct {
	Proto tcpip.TransportProtocolNumber
	Addr  netip.AddrPort
}

func New() (Mapping, error) {
	return newMapping()
}

type Endpoint struct {
	Addr  netip.AddrPort                // local address
	Proto tcpip.TransportProtocolNumber // protocol
}

func (e Endpoint) Valid() bool {
	return e.Proto != 0 && e.Addr.IsValid()
}

func FromOutbound(ip []byte) Endpoint {
	var iphdr header.Network
	var addr netip.Addr
	switch header.IPVersion(ip) {
	case 4:
		iphdr = header.IPv4(ip)
		addr = netip.AddrFrom4(iphdr.SourceAddress().As4())
	case 6:
		iphdr = header.IPv6(ip)
		addr = netip.AddrFrom16(iphdr.SourceAddress().As16())
	default:
		panic(fmt.Sprintf("invalid ip: %s", hex.EncodeToString(ip[:min(len(ip), 20)])))
	}

	switch iphdr.TransportProtocol() {
	case header.TCPProtocolNumber:
		return Endpoint{
			Proto: header.TCPProtocolNumber,
			Addr:  netip.AddrPortFrom(addr, header.TCP(iphdr.Payload()).SourcePort()),
		}
	case header.UDPProtocolNumber:
		return Endpoint{
			Proto: header.UDPProtocolNumber,
			Addr:  netip.AddrPortFrom(addr, header.UDP(iphdr.Payload()).SourcePort()),
		}
	default:
		panic(fmt.Sprintf("invalid ip: %s", hex.EncodeToString(ip[:min(len(ip), 20)])))
	}
}