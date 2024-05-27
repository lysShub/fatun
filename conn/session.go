package conn

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

type Session interface {
	Valid() bool
	String() string
	Builtin() Session
	IsBuiltin() bool
	Overhead() int
	Encode(from *packet.Packet) error
	Decode(to *packet.Packet) error

	Reset(proto tcpip.TransportProtocolNumber, dst netip.Addr) Session
	Protocol() tcpip.TransportProtocolNumber
	Destionation() netip.Addr
}

type Default = *defaultSession

var _ Session = (Default)(nil)

type defaultSession struct {
	proto tcpip.TransportProtocolNumber
	dst   netip.Addr // only ipv4
}

func New(proto tcpip.TransportProtocolNumber, remote netip.Addr) Session {
	return &defaultSession{proto, remote}
}

func (p *defaultSession) Reset(proto tcpip.TransportProtocolNumber, remote netip.Addr) Session {
	p.proto, p.dst = proto, remote
	return p
}
func (p *defaultSession) Protocol() tcpip.TransportProtocolNumber { return p.proto }
func (p *defaultSession) Destionation() netip.Addr                { return p.dst }

func (p *defaultSession) Builtin() Session {
	return &defaultSession{dst: netip.IPv4Unspecified(), proto: tcp.ProtocolNumber}
}
func (p *defaultSession) IsBuiltin() bool {
	return p.Valid() && p.dst.Is4() && p.dst.IsUnspecified() && p.proto == tcp.ProtocolNumber
}
func (p *defaultSession) Overhead() int { return 5 }
func (p *defaultSession) Valid() bool {
	return p != nil && p.dst.IsValid() && p.dst.Is4() &&
		(p.proto == tcp.ProtocolNumber || p.proto == udp.ProtocolNumber)
}

func (p *defaultSession) String() string {
	if p == nil {
		return "nil"
	}
	var proto string
	switch p.proto {
	case tcp.ProtocolNumber:
		proto = "tcp"
	case udp.ProtocolNumber:
		proto = "udp"
	default:
		proto = fmt.Sprintf("unknown(%d)", p.proto)
	}
	return fmt.Sprintf("%s:%s", p.dst.String(), proto)
}

func (p *defaultSession) Encode(pkt *packet.Packet) error {
	if !p.Valid() {
		return errors.Errorf("encode from invalid Peer: %s", p.String())
	}
	if debug.Debug() {
		require.True(test.T(), p.dst.Is4())
	}

	pkt.Attach(p.dst.AsSlice())
	switch p.proto {
	case tcp.ProtocolNumber, udp.ProtocolNumber:
		pkt.Attach([]byte{byte(p.proto)})
	default:
		return errors.Errorf("not support protocol %d", p.proto)
	}
	return nil
}

func (p *defaultSession) Decode(seg *packet.Packet) (err error) {
	if p == nil {
		return errors.New("decode to nil Peer")
	}

	b := seg.Bytes()
	if len(b) < 5 {
		return errors.New("decode from invalid packet")
	}

	p.proto = tcpip.TransportProtocolNumber(b[0])
	p.dst = netip.AddrFrom4([4]byte(b[1:5]))
	seg.SetHead(seg.Head() + 5)
	return nil
}
