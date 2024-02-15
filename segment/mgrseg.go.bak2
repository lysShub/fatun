package segment

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/lysShub/itun/segment/internal"
	"google.golang.org/protobuf/proto"
)

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
	if typ.validate() {
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
	MgrSegPing      // ping请求

	enum_end
)

func (st MgrSegType) Validate() error {
	if !st.validate() {
		return ErrInvalidMgrType(st)
	}
	return nil
}

func (st MgrSegType) validate() bool {
	return enum_start < st &&
		st < enum_end
}

func (st MgrSegType) IsConfig() bool {
	return st.validate() && st <= MgrSegEndConfig
}

type MgrClient struct {
	sync.RWMutex
	conn net.Conn

	seg MgrSeg
}

func NewMgrClient(conn net.Conn) *MgrClient {
	return &MgrClient{conn: conn}
}

func (mc *MgrClient) do(t MgrSegType, req []byte) (resp []byte, err error) {
	mc.Lock()
	defer func() {
		mc.seg = mc.seg[:mgrSegMiniSize]
		mc.Unlock()
	}()

	// request
	if !mc.seg.setType(t) {
		return nil, fmt.Errorf("invalid manager segment type %s", t)
	}
	mc.seg.setPayloadLen(uint16(len(req)))
	mc.seg = append(mc.seg, req...)

	if n, err := mc.conn.Write(mc.seg); err != nil {
		return nil, err
	} else if n != len(mc.seg) {
		return nil, fmt.Errorf("impossible")
	}

	// response
	mc.seg, err = ReadMgrMsg(mc.conn, mc.seg)
	if err != nil {
		return nil, err
	}
	if mc.seg.Type() != t {
		return nil, fmt.Errorf("expect %s manager segment type, get %s", t, mc.seg.Type())
	}

	return mc.seg.payload(), nil
}

func (mc *MgrClient) Crypto(crypto bool) (err error) {
	p, err := proto.Marshal(&internal.CryptoReq{Crypto: crypto})
	if err != nil {
		return err
	}

	b, err := mc.do(MgrSegCrypto, p)
	if err != nil {
		return err
	}

	var resp = &internal.CryptoResp{}
	return proto.Unmarshal(b, resp)
}

func (mc *MgrClient) IPv6() (ipv6 bool, err error) {
	p, err := proto.Marshal(&internal.IPv6Req{})
	if err != nil {
		return false, err
	}

	b, err := mc.do(MgrSegIPv6, p)
	if err != nil {
		return false, err
	}

	var resp = &internal.IPv6Resp{}
	return resp.IPv6, proto.Unmarshal(b, resp)
}

func (mc *MgrClient) AddTCP(addr netip.AddrPort) (sessionId uint16, err error) {
	p, err := proto.Marshal(&internal.AddTCPReq{Addr: addr.String()})
	if err != nil {
		return 0, err
	}

	b, err := mc.do(MgrSegAddTCP, p)
	if err != nil {
		return 0, err
	}

	var resp = &internal.AddTCPResp{}
	err = proto.Unmarshal(b, resp)
	return uint16(resp.SessionID), errors.Join(err, strErr(resp.Error))
}

func (mc *MgrClient) DelTCP(addr netip.AddrPort) (err error) {
	p, err := proto.Marshal(&internal.DelTCPReq{Addr: addr.String()})
	if err != nil {
		return err
	}

	b, err := mc.do(MgrSegDelTCP, p)
	if err != nil {
		return err
	}

	var resp = &internal.DelTCPResp{}
	return errors.Join(proto.Unmarshal(b, resp), strErr(resp.Error))
}

func (mc *MgrClient) AddUDP(addr netip.AddrPort) (sessionId uint16, err error) {
	p, err := proto.Marshal(&internal.AddUDPReq{Addr: addr.String()})
	if err != nil {
		return 0, err
	}

	b, err := mc.do(MgrSegAddUDP, p)
	if err != nil {
		return 0, err
	}

	var resp = &internal.AddUDPResp{}
	err = proto.Unmarshal(b, resp)
	return uint16(resp.SessionID), errors.Join(err, strErr(resp.Error))
}

func (mc *MgrClient) DelUDP(addr netip.AddrPort) (err error) {
	p, err := proto.Marshal(&internal.DelUDPReq{Addr: addr.String()})
	if err != nil {
		return err
	}

	b, err := mc.do(MgrSegDelUDP, p)
	if err != nil {
		return err
	}

	var resp = &internal.DelUDPResp{}
	return errors.Join(proto.Unmarshal(b, resp), strErr(resp.Error))
}

type MgrServer struct {
	sync.RWMutex
	conn net.Conn
	seg  MgrSeg
}

func NewMgrServer(conn net.Conn) *MgrServer {
	return &MgrServer{
		conn: conn,
	}
}

func (ms *MgrServer) NextSeg() (typ MgrSegType, err error) {
	ms.seg, err = ReadMgrMsg(ms.conn, ms.seg)

	err = errors.Join(err, ms.seg.Type().Validate())
	return ms.seg.Type(), err
}

func (ms *MgrServer) Crypto(call func(crypto bool)) error {
	var req = &internal.CryptoReq{}
	if err := proto.Unmarshal(ms.seg.payload(), req); err != nil {
		return err
	}

	call(req.Crypto)

	var resp = &internal.CryptoResp{}
	b, err := proto.Marshal(resp)
	if err != nil {
		return err
	}

	ms.seg = append(ms.seg[mgrSegMiniSize:], b...)

	_, err = ms.conn.Write(ms.seg)
	return err
}

func (ms *MgrServer) IPv6(call func() bool) error {
	var req = &internal.IPv6Req{}
	if err := proto.Unmarshal(ms.seg.payload(), req); err != nil {
		return err
	}

	var resp = &internal.IPv6Resp{IPv6: call()}
	if b, err := proto.Marshal(resp); err != nil {
		return err
	} else {
		ms.seg = append(ms.seg[mgrSegMiniSize:], b...)
	}

	_, err := ms.conn.Write(ms.seg)
	return err
}

func (ms *MgrServer) AddTCP(call func(addr netip.AddrPort) (uint16, error)) error {
	var req = &internal.AddTCPReq{}
	if err := proto.Unmarshal(ms.seg.payload(), req); err != nil {
		return err
	}

	var resp = &internal.AddTCPResp{}

	addr, err := netip.ParseAddrPort(req.Addr)
	if err != nil {
		resp.Error = err.Error()
		return err
	}
	if id, err := call(addr); err != nil {
		resp.Error = err.Error()
		return err
	} else {
		resp.SessionID = uint32(id)
	}

	return nil
}

func ReadMgrMsg(conn net.Conn, b []byte) (MgrSeg, error) {
	if len(b) < mgrSegMiniSize {
		for i := 0; i < mgrSegMiniSize; i++ {
			b = append(b, 0)
		}
	}
	b = b[:mgrSegMiniSize]

	if _, err := io.ReadFull(conn, b); err != nil {
		return nil, err
	}

	n := int(MgrSeg(b).payloadLen())
	for len(b) < n+mgrSegMiniSize {
		b = append(b, 0)
		b = b[:cap(b)]
	}
	b = b[:n+mgrSegMiniSize]

	if _, err := io.ReadFull(conn, b[mgrSegMiniSize:]); err != nil {
		return nil, err
	}

	return b, nil
}

type ErrIncorrectMgrMsgFormat MgrSeg

func (e ErrIncorrectMgrMsgFormat) Error() string {
	return fmt.Sprintf(
		"incorrect manager segment format: Type=%s Payload=%s",
		MgrSeg(e).Type(), hex.EncodeToString(MgrSeg(e).payload()),
	)
}

type ErrInvalidMgrType MgrSegType

func (e ErrInvalidMgrType) Error() string {
	return fmt.Sprintf("invalid manager segment type %s", MgrSegType(e))
}

func strErr(s string) error {
	if s == "" {
		return nil
	} else {
		return errors.New(s)
	}
}
