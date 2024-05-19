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

	New() Peer
	Reset(proto tcpip.TransportProtocolNumber, remote netip.Addr)
	Protocol() tcpip.TransportProtocolNumber
	Peer() netip.Addr
	Clone() Peer
}

type Default struct {
	proto tcpip.TransportProtocolNumber
	peer  netip.Addr // only ipv4
}

var _ fatcp.Attacher = (*Default)(nil)

func New(proto tcpip.TransportProtocolNumber, remote netip.Addr) Peer {
	return &Default{proto, remote}
}

func (p *Default) New() Peer { return &Default{} }
func (p *Default) Reset(proto tcpip.TransportProtocolNumber, remote netip.Addr) {
	p.proto, p.peer = proto, remote
}
func (p *Default) Protocol() tcpip.TransportProtocolNumber { return p.proto }
func (p *Default) Peer() netip.Addr                        { return p.peer }
func (p *Default) Clone() Peer                             { return &Default{p.proto, p.peer} }

var _builtinPeer = &Default{peer: netip.IPv4Unspecified(), proto: tcp.ProtocolNumber}

func (p *Default) Builtin() fatcp.Attacher { return _builtinPeer }
func (p *Default) IsBuiltin() bool         { return p.Valid() && (*p) == (*_builtinPeer) }
func (p *Default) Overhead() int           { return 5 }
func (p *Default) Valid() bool {
	return p != nil && p.peer.IsValid() && p.peer.Is4() &&
		(p.proto == tcp.ProtocolNumber || p.proto == udp.ProtocolNumber)
}

func (p *Default) String() string {
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

func (p *Default) Encode(pkt *packet.Packet) error {
	if !p.Valid() {
		return errors.Errorf("invalid peer: %s", p.String())
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

func (p *Default) Decode(seg *packet.Packet) (err error) {
	if p == nil {
		return errors.Errorf("invalid peer: %s", p.String())
	}

	b := seg.Bytes()
	if len(b) < 5 {
		return errors.New("invalid segment")
	}

	p.proto = tcpip.TransportProtocolNumber(b[0])
	p.peer = netip.AddrFrom4([4]byte(b[1:5]))
	seg.SetHead(seg.Head() + 5)
	return nil
}
