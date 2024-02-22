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

const CtrSegID uint16 = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
	HdrSize   = idOffset2
)

func NewSegment(n int) *Segment {
	return &Segment{
		p: relraw.NewPacket(0, n),
	}
}

func ToSegment(p *relraw.Packet) *Segment {
	return &Segment{p: p}
}

func FromData(data []byte) *Segment {
	p := relraw.NewPacket(64, len(data)+HdrSize)

	copy(p.Data()[HdrSize:], data)
	return &Segment{p: p}
}

// ID get session id
func (s *Segment) ID() uint16 {
	b := s.p.Data()
	return binary.BigEndian.Uint16(b[idOffset1:idOffset2])
}

// SetID set session id
func (s *Segment) SetID(id uint16) {
	s.p.AllocHead(HdrSize)

	s.SetHead(s.Head() - HdrSize)
	b := s.p.Data()
	binary.BigEndian.PutUint16(b[idOffset1:idOffset2], id)
}

// Payload TCP/UPD packet
func (s *Segment) Payload() []byte {
	return s.p.Data()[idOffset2:]
}

func (s *Segment) Packet() *relraw.Packet {
	return s.p
}

//	func (seg *Segment) AllocHead(head int) bool {
//		return seg.p.AllocHead(head)
//	}
func (seg *Segment) AllocTail(tail int) bool {
	return seg.p.AllocTail(tail)
}
func (seg *Segment) Attach(b []byte) {
	seg.p.Attach(b)
}
func (seg *Segment) Data() []byte {
	return seg.p.Data()
}
func (seg *Segment) Head() int {
	return seg.p.Head()
}
func (seg *Segment) Len() int {
	return seg.p.Len()
}
func (seg *Segment) SetHead(head int) {
	seg.p.SetHead(head)
}
func (seg *Segment) SetLen(n int) {
	seg.p.SetLen(n)
}
func (seg *Segment) Sets(head int, n int) {
	seg.p.Sets(head, n)
}
func (seg *Segment) Tail() int {
	return seg.p.Tail()
}
