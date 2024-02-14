package segment

import (
	"encoding/binary"
)

// Segment include PxySeg and MgrSeg, identify by SessionID; MgrSeg is a TCP
// packet, PxySeg indicate a transport layer packet, with header.
//
// SessionID   TCP/UDP packet
// [0, 2)      [2, n)
type Segment []byte

const MgrSegID uint16 = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
)

// ID get session id
func (s Segment) ID() uint16 {
	return binary.BigEndian.Uint16(s[idOffset1:idOffset2])
}

// SetID set session id
func (s Segment) SetID(id uint16) {
	binary.BigEndian.PutUint16(s[idOffset1:idOffset2], id)
}

// Payload TCP/UPD packet
func (s Segment) Payload() []byte {
	return s[idOffset2:]
}