package peer

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

type Peer interface {
	fatcp.Attacher

	Make() Peer
	Reset(proto tcpip.TransportProtocolNumber, remote netip.Addr)
	Protocol() tcpip.TransportProtocolNumber
	Peer() netip.Addr
}

type Default = *defaultPeer

var _ Peer = (Default)(nil)

type defaultPeer struct {
	proto tcpip.TransportProtocolNumber
	peer  netip.Addr // only ipv4
}

func New(proto tcpip.TransportProtocolNumber, remote netip.Addr) Peer {
	return &defaultPeer{proto, remote}
}

func (p *defaultPeer) Make() Peer { return &defaultPeer{} }
func (p *defaultPeer) Reset(proto tcpip.TransportProtocolNumber, remote netip.Addr) {
	p.proto, p.peer = proto, remote
}
func (p *defaultPeer) Protocol() tcpip.TransportProtocolNumber { return p.proto }
func (p *defaultPeer) Peer() netip.Addr                        { return p.peer }
func (p *defaultPeer) Clone() Peer                             { return &defaultPeer{p.proto, p.peer} }

var _builtinPeer = &defaultPeer{peer: netip.IPv4Unspecified(), proto: tcp.ProtocolNumber}

func (p *defaultPeer) Builtin() fatcp.Attacher { return _builtinPeer }
func (p *defaultPeer) IsBuiltin() bool         { return p.Valid() && (*p) == (*_builtinPeer) }
func (p *defaultPeer) Overhead() int           { return 5 }
func (p *defaultPeer) Valid() bool {
	return p != nil && p.peer.IsValid() && p.peer.Is4() &&
		(p.proto == tcp.ProtocolNumber || p.proto == udp.ProtocolNumber)
}

func (p *defaultPeer) String() string {
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
	return fmt.Sprintf("%s:%s", p.peer.String(), proto)
}

func (p *defaultPeer) Encode(pkt *packet.Packet) error {
	if !p.Valid() {
		return errors.Errorf("encode from invalid Peer: %s", p.String())
	}
	if debug.Debug() {
		require.True(test.T(), p.peer.Is4())
	}

	pkt.Attach(p.peer.AsSlice())
	switch p.proto {
	case tcp.ProtocolNumber, udp.ProtocolNumber:
		pkt.Attach([]byte{byte(p.proto)})
	default:
		return errors.Errorf("not support protocol %d", p.proto)
	}
	return nil
}

func (p *defaultPeer) Decode(seg *packet.Packet) (err error) {
	if p == nil {
		return errors.New("decode to nil Peer")
	}

	b := seg.Bytes()
	if len(b) < 5 {
		return errors.New("decode from invalid packet")
	}

	p.proto = tcpip.TransportProtocolNumber(b[0])
	p.peer = netip.AddrFrom4([4]byte(b[1:5]))
	seg.SetHead(seg.Head() + 5)
	return nil
}
