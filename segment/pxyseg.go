package segment

/*

// SessionID   Proto  Payload
// [0, 2)      [2]    [3,n)
type PxySeg Segment

const protoOffset = 2

func (ps PxySeg) Set(id uint16, proto tcpip.TransportProtocolNumber) bool {
	if id == MgrSegID {
		return false
	}
	Segment(ps).SetID(id)
	ps[protoOffset] = byte(proto)
	return true
}

func (ps PxySeg) SetTCP(id uint16) bool {
	return ps.Set(id, header.TCPProtocolNumber)
}

func (ps PxySeg) SetUDP(id uint16) bool {
	return ps.Set(id, header.UDPProtocolNumber)
}

func (ps PxySeg) Get() (id uint16, proto tcpip.TransportProtocolNumber) {
	id = Segment(ps).ID()
	if id == MgrSegID {
		return 0, 0
	}

	proto = tcpip.TransportProtocolNumber(ps[protoOffset])
	return
}

*/
