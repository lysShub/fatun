package segment

import (
	"encoding/binary"

	"github.com/lysShub/relraw"
)

// Segment include PxySeg and MgrSeg, identify by SessionID; MgrSeg is a TCP
// packet, PxySeg indicate a transport layer packet, with header.
//
// SessionID   Payload(tcp/udp packet)
// [0, 2)      [2, n)
type Segment struct {
	p *relraw.Packet
}

func NewSegment(n int) *Segment {
	return &Segment{
		p: relraw.NewPacket(0, n),
	}
}

func ToSegment(p *relraw.Packet) *Segment {
	p.SetHead(HdrSize)
	return &Segment{p: p}
}

func FromData(data []byte) *Segment {
	p := relraw.NewPacket(64, len(data)+HdrSize)

	copy(p.Data()[HdrSize:], data)
	return &Segment{p: p}
}

const CtrSegID uint16 = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
	HdrSize   = idOffset2
)

func (s *Segment) Packet() *relraw.Packet {
	return s.p
}

// ID get session id
func (s Segment) ID() uint16 {
	b := s.p.Data()
	return binary.BigEndian.Uint16(b[idOffset1:idOffset2])
}

// SetID set session id
func (s Segment) SetID(id uint16) {
	b := s.p.Data()
	binary.BigEndian.PutUint16(b[idOffset1:idOffset2], id)
}

// Payload TCP/UPD packet
func (s Segment) Payload() []byte {
	return s.p.Data()[idOffset2:]
}
