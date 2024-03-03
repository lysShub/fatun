package session

import (
	"encoding/binary"

	"github.com/lysShub/relraw"
)

// todo: session

// SessionID   Payload(tcp/udp packet)
// [0, 2)      [2, n)

type SessID uint16

func SetID(p *relraw.Packet, id SessID) {
	p.AllocHead(IDSize)
	p.SetHead(p.Head() - IDSize)

	b := p.Data()
	binary.BigEndian.PutUint16(b[idOffset1:idOffset2], uint16(id))
}

func GetID(p *relraw.Packet) SessID {
	b := p.Data()
	id := binary.BigEndian.Uint16(b[idOffset1:idOffset2])
	p.SetHead(p.Head() + IDSize)
	return SessID(id)
}

const CtrSessID SessID = 0xffff
const (
	idOffset1 = 0
	idOffset2 = 2
	IDSize    = idOffset2
)
