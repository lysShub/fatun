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
	*relraw.Packet
}

const CtrSegID uint16 = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
)

// ID get session id
func (s Segment) ID() uint16 {
	return binary.BigEndian.Uint16(s.Data()[idOffset1:idOffset2])
}

// SetID set session id
func (s Segment) SetID(id uint16) {
	binary.BigEndian.PutUint16(s.Data()[idOffset1:idOffset2], id)
}

// Payload TCP/UPD packet
func (s Segment) Payload() []byte {
	return s.Data()[idOffset2:]
}
