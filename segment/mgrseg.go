package segment

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	internal "github.com/lysShub/itun/segment/internal2"
	"google.golang.org/protobuf/proto"
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
	Crypto                // 是否加密
	IPv6                  // 是否支持IPv6
	EndConfig             // 结束mgr初始化配置
	AddTCP                // 新增tcp代理
	DelTCP                // 删除tcp代理
	AddUDP                // 新增udp代理
	DelUDP                // 删除udp代理
	PackLoss              // 请求Server报告上行丢包率
	Ping                  // ping请求
	enum_end
)

func (st MgrSegType) Validate() bool {
	return enum_start < st &&
		st < enum_end
}

func (st MgrSegType) IsConfig() bool {
	return st.Validate() && st <= EndConfig
}

type MgrClient struct {
	sync.RWMutex

	conn net.Conn
	seg  MgrSeg
}

// 1. 序列化req
// 2. 发送
// 3. 接收
// 4. 反序列化后返回
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
		return nil, ErrInvalidMgrType(t)
	}
	mc.seg.setPayloadLen(uint16(len(req)))
	mc.seg = append(mc.seg, req...)

	if n, err := mc.conn.Write(mc.seg); err != nil {
		return nil, err
	} else if n != len(mc.seg) {
		panic("impossible")
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

func (mc *MgrClient) IPv6() (ipv6 bool, err error) {
	p, err := proto.Marshal(&internal.IPv6Req{})
	if err != nil {
		return false, err
	}

	b, err := mc.do(IPv6, p)
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

	b, err := mc.do(AddTCP, p)
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

	b, err := mc.do(DelTCP, p)
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

	b, err := mc.do(AddUDP, p)
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

	b, err := mc.do(DelUDP, p)
	if err != nil {
		return err
	}

	var resp = &internal.DelUDPResp{}
	return errors.Join(proto.Unmarshal(b, resp), strErr(resp.Error))
}

// todo: 尝试grpc

type mgrServer struct {
	conn net.Conn
	srv  MgrServer
	seg  MgrSeg
}

type MgrServer interface {
	Crypto(crypto bool)
	IPv6() bool
	EndConfig()
	AddTCP(addr netip.AddrPort) (uint16, error)
	DelTCP(addr netip.AddrPort) error
	AddUDP(addr netip.AddrPort) (uint16, error)
	DelUDP(addr netip.AddrPort) error
	PackLoss() float32
	Ping()
}

func MgrServe(ctx context.Context, conn net.Conn, srv mgrServer) {
	var s = &mgrServer{
		conn: conn,
	}

	for {
		if err := s.readMsg(); err != nil {
			panic(err)
		}

		switch t := s.seg.Type(); t {

		}
	}
}

func (ms *mgrServer) readMsg() (err error) {
	var req = &internal.CryptoReq{}
	proto.Unmarshal(ms.seg.payload(), req)

	ms.srv.Crypto(req.Crypto)

	var resp = &internal.CryptoResp{}
	proto.Marshal(resp)

	return nil
}

func (ms *mgrServer) crypto() (err error) {
	ms.seg, err = ReadMgrMsg(ms.conn, ms.seg)
	if err != nil {
		return err
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
