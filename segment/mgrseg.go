package segment

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/netip"
	"unsafe"
)

/*
	MgrSeg 总是由client发出req, proxy server 回复resp

	todo: 这个不对，MgrSeg 还是有tcp头
*/

// MgrSegType PayloadLen Payload
// [0]        [1,3)      [3,n)
type MgrSeg []byte

const (
	typeOffset    = 0
	lenOffset1    = 1
	lenOffset2    = 3
	payloadOffset = 3

	mgrSegMiniSize = lenOffset2
)

func (ms MgrSeg) setType(typ MgrSegType) bool {
	if typ.Validate() {
		Segment(ms).SetID(MgrSegID)
		ms[typ] = byte(typ)
		return true
	}
	return false
}

func (ms MgrSeg) Type() MgrSegType {
	if Segment(ms).ID() == MgrSegID {
		return MgrSegType(ms[typeOffset])
	}
	return 0
}

func (ms MgrSeg) payloadLen() uint16 {
	return binary.BigEndian.Uint16(ms[lenOffset1:lenOffset2])
}

func (ms MgrSeg) setPayloadLen(n uint16) {
	binary.BigEndian.PutUint16(ms[lenOffset1:lenOffset2], n)
}

func (ms MgrSeg) payload() []byte {
	return ms[payloadOffset : payloadOffset+ms.payloadLen()]
}

//go:generate stringer -output=mgrset_gen.go -trimprefix=MgrSeg -type=MgrSegType
type MgrSegType uint8

const (
	enum_start MgrSegType = iota

	MgrSegCrypto    // 是否加密
	MgrSegIPv6      // 是否支持IPv6
	MgrSegEndConfig // 结束mgr初始化配置
	MgrSegAddTCP    // 新增tcp代理
	MgrSegDelTCP    // 删除tcp代理
	MgrSegAddUDP    // 新增udp代理
	MgrSegDelUDP    // 删除udp代理
	MgrSegPackLoss  // 请求Server报告上行丢包率

	enum_end
)

func (st MgrSegType) Validate() bool {
	return enum_start < st &&
		st < enum_end
}

func (st MgrSegType) IsConfig() bool {
	return st.Validate() && st <= MgrSegEndConfig
}

// type MgrMsg struct {
// 	Type    MgrSegType
// 	Payload []byte // todo: not export
// }

func ReadMgrMsg(conn net.Conn) (MgrSeg, error) {
	var b = make(MgrSeg, mgrSegMiniSize)

	if _, err := io.ReadFull(conn, b); err != nil {
		return nil, err
	}

	n := int(b.payloadLen())
	for len(b) < n+mgrSegMiniSize {
		b = append(b, 0)
		b = b[:cap(b)]
	}
	b = b[:n+mgrSegMiniSize]

	if _, err := io.ReadFull(conn, b[mgrSegMiniSize:]); err != nil {
		return nil, err
	}

	// return &MgrMsg{
	// 	Type:    b.Type(),
	// 	Payload: b[mgrSegMiniSize:],
	// }, nil
	return b, nil
}

type ErrIncorrectMgrMsgFormat MgrSeg

func (e ErrIncorrectMgrMsgFormat) Error() string {
	return fmt.Sprintf(
		"incorrect manager segment format: Type=%s Payload=%s",
		MgrSeg(e).Type(), hex.EncodeToString(MgrSeg(e).payload()),
	)
}

func (ms MgrSeg) Crypto() (bool, error) {
	assert(ms, MgrSegCrypto)
	if ms.payloadLen() == 1 {
		return ms.payload()[0] > 0, nil
	}
	return false, ErrIncorrectMgrMsgFormat(ms)
}
func MgrCrypto(crypto bool) MgrSeg {
	var seg = make(MgrSeg, mgrSegMiniSize+1)
	if crypto {
		seg[mgrSegMiniSize] = 1
	}
	setType(seg, MgrSegCrypto)
	return seg
}

func (ms MgrSeg) IPv6() (bool, error) {
	assert(ms, MgrSegIPv6)
	if ms.payloadLen() == 1 {
		return ms.payload()[0] > 0, nil
	}
	return false, ErrIncorrectMgrMsgFormat(ms)
}
func MgrIPv6(ipv6 bool) MgrSeg {
	var seg = make(MgrSeg, mgrSegMiniSize+1)
	if ipv6 {
		seg[mgrSegMiniSize] = 1
	}
	setType(seg, MgrSegCrypto)
	return seg
}

func (ms MgrSeg) EndConfig() error {
	assert(ms, MgrSegEndConfig)
	if ms.payloadLen() == 0 {
		return nil
	}
	return ErrIncorrectMgrMsgFormat(ms)
}
func MgrEndConfig() MgrSeg {
	return setType(make(MgrSeg, mgrSegMiniSize), MgrSegEndConfig)
}

func (ms MgrSeg) AddTCP() (netip.AddrPort, error) {
	return ms.addrport(MgrSegAddTCP)
}
func MgrAddTCP(addr netip.AddrPort) MgrSeg {
	return setType(addrport(addr), MgrSegAddTCP)
}

func (ms MgrSeg) DelTCP() (netip.AddrPort, error) {
	return ms.addrport(MgrSegDelTCP)
}
func MgrDelTCP(addr netip.AddrPort) MgrSeg {
	return setType(addrport(addr), MgrSegDelTCP)
}

func (ms MgrSeg) AddUDP() (netip.AddrPort, error) {
	return ms.addrport(MgrSegAddUDP)
}
func MgrAddUDP(addr netip.AddrPort) MgrSeg {
	return setType(addrport(addr), MgrSegAddUDP)
}

func (ms MgrSeg) DelUDP(addr netip.AddrPort) (netip.AddrPort, error) {
	return ms.addrport(MgrSegDelUDP)
}
func MgrDelUDP(addr netip.AddrPort) MgrSeg {
	return setType(addrport(addr), MgrSegDelUDP)
}

func (ms MgrSeg) PackLoss() error {
	assert(ms, MgrSegPackLoss)
	if ms.payloadLen() == 0 {
		return nil
	}
	return ErrIncorrectMgrMsgFormat(ms)
}
func MgrPackLoss(pl float32) MgrSeg {
	var seg = make(MgrSeg, mgrSegMiniSize+4)
	binary.BigEndian.AppendUint32(seg[mgrSegMiniSize:mgrSegMiniSize+4], *(*uint32)(unsafe.Pointer(&pl)))
	return setType(seg, MgrSegPackLoss)
}

func (ms MgrSeg) addrport(t MgrSegType) (netip.AddrPort, error) {
	assert(ms, t)
	if ms.payloadLen() > 0 {
		return netip.ParseAddrPort(string(ms.payload()))
	}
	return netip.AddrPort{}, ErrIncorrectMgrMsgFormat(ms)
}
func addrport(addr netip.AddrPort) MgrSeg {
	p := addr.String()

	var seg = make(MgrSeg, mgrSegMiniSize+len(p))
	copy(seg[mgrSegMiniSize:], p)
	return seg
}

func setType(b MgrSeg, t MgrSegType) MgrSeg {
	if !b.setType(t) {
		panic(fmt.Sprintf("invalid manager segment type %d", t))
	}
	b.setPayloadLen(uint16(len(b) - mgrSegMiniSize))
	return b
}

func assert(seg MgrSeg, typ MgrSegType) {
	if !typ.Validate() {
		panic(fmt.Sprintf("invalid manager segment type %d", typ))
	} else if seg.Type() != typ {
		panic(fmt.Sprintf("expect %s manager segment type, get %s", typ, seg.Type()))
	}
}
