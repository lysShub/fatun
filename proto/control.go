package proto

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// ctrSeg终是客户端发给服务器, 然服务器回复一个ctrSeg，客户端收到回复
// 后才能进入下一轮
type ctrSeg struct {
	header  [3]byte // {CtrCode[0] payload[1:2]}
	payload []byte
}

type CtrCode uint8

const (
	_               CtrCode = iota
	RegisterSession         // 注册一个代理连接
	RemoveSession           // 释放一个代理连接, 客户端通过超时主动释放（服务器也会有超时释放）
	Security                // 基于tls交换PxySeg的密钥
	PackLoss                // 报告PxySeg丢包率
	Start                   // 开始代理PxySeg
	RequireReg              // 直接发送了未注册的PxySeg
)

func (s *ctrSeg) CtrCode() CtrCode {
	return CtrCode(s.header[0])
}

func (s *ctrSeg) setCtrCode(code CtrCode) {
	s.header[0] = byte(code)
}

func (s *ctrSeg) payloadLength() uint16 {
	return binary.BigEndian.Uint16(s.header[1:])
}

func setBytesBuf(b []byte, n int) []byte {
	for cap(b) < n {
		b = b[:cap(b)]
		b = append(b, 0)
	}
	return b[:n]
}

func (s *ctrSeg) setPayloadLength(n uint16) {
	binary.BigEndian.PutUint16(s.header[1:], n)
	s.payload = setBytesBuf(s.payload, int(n))
}

func (s *ctrSeg) ReadFrom(conn net.Conn) error {
	if _, err := io.ReadFull(conn, s.header[:]); err != nil {
		return err
	}

	s.payload = setBytesBuf(s.payload, int(s.payloadLength()))

	_, err := io.ReadFull(conn, s.payload)
	return err
}

func (s *ctrSeg) WriteTo(conn net.Conn) error {
	if _, err := conn.Write(s.header[:]); err != nil {
		return err
	}
	if n := s.payloadLength(); n > 0 {
		if _, err := conn.Write(s.payload[:n]); err != nil {
			return err
		}
	}
	return nil
}

func (s *ctrSeg) assert(exp CtrCode) {
	if act := s.CtrCode(); act != exp {
		panic(fmt.Sprintf("can't parse control segment %d to %d", act, exp))
	}
}

func (s *ctrSeg) enStart() error {
	s.setCtrCode(Start)
	s.setPayloadLength(0)
	return nil
}

func (s *ctrSeg) deStart() error {
	s.assert(Start)
	if s.payloadLength() != 0 {
		return fmt.Errorf("invalid Register control segment format")
	}
	return nil
}

func (s *ctrSeg) enRegistrSessionReq(se Session) error {
	s.setCtrCode(RegisterSession)

	b, err := se.decode()
	if err != nil {
		return err
	}
	s.setPayloadLength(uint16(len(b)))
	copy(s.payload[0:], b)

	return nil
}

func (s *ctrSeg) enRegistrSessionResp(id uint16, msg string) {
	s.setCtrCode(RegisterSession)

	n := min(len(msg)+2, 0xffff-1)
	s.setPayloadLength(uint16(n))

	binary.BigEndian.PutUint16(s.payload, id)
	copy(s.payload[2:], []byte(msg))
}

func (s *ctrSeg) deRegisterSessionReq() (Session, error) {
	s.assert(RegisterSession)

	var se Session
	return se, se.encode(s.payload)
}

func (s *ctrSeg) deRegisterSessionResp() (uint16, error) {
	s.assert(RegisterSession)

	if len(s.payload) < 2 {
		return 0, fmt.Errorf("invalid Register Session response segment format")
	}

	id := binary.BigEndian.Uint16(s.payload)
	if msg := s.payload[2:]; id == 0 || len(msg) > 0 {
		return id, errors.New(string(msg))
	}
	return id, nil
}

func (s *ctrSeg) enSecurity() {
	s.setCtrCode(Security)
	s.setPayloadLength(0)
}

func (s *ctrSeg) deSecurity() error {
	s.assert(Security)
	if s.payloadLength() != 0 {
		return fmt.Errorf("invalid Security control segment format")
	}
	return nil
}

type ControlClient struct {
	conn net.Conn
	seg  ctrSeg
}

func NewControlClient(conn net.Conn) *ControlClient {
	return &ControlClient{
		conn: conn,
		seg: ctrSeg{
			payload: make([]byte, 0, 16),
		},
	}
}

func (c *ControlClient) Start() error {
	c.seg.enStart()
	if err := c.seg.WriteTo(c.conn); err != nil {
		return err
	}

	if err := c.seg.ReadFrom(c.conn); err != nil {
		return err
	}
	return c.seg.deStart()
}

func (c *ControlClient) RegisterSession(s Session) (sessId uint16, err error) {
	if err := c.seg.enRegistrSessionReq(s); err != nil {
		return 0, err
	}

	if err := c.seg.WriteTo(c.conn); err != nil {
		return 0, err
	}

	if err := c.seg.ReadFrom(c.conn); err != nil {
		return 0, err
	}
	return c.seg.deRegisterSessionResp()
}

type connWrapClose struct {
	net.Conn
}

func (c *connWrapClose) Close() error { return nil }

func (c *ControlClient) Security(cfg *tls.Config) (key [16]byte, err error) {
	c.seg.enSecurity()

	if err := c.seg.WriteTo(c.conn); err != nil {
		return key, err
	}

	{
		tconn := tls.Client(&connWrapClose{Conn: c.conn}, cfg)
		if err := tconn.Handshake(); err != nil {
			return key, err
		}

		if _, err := rand.Read(key[:]); err != nil {
			return key, err
		}

		if _, err := tconn.Write(key[:]); err != nil {
			return key, err
		}
		if err := tconn.Close(); err != nil {
			return key, err
		}
	}

	if err := c.seg.ReadFrom(c.conn); err != nil {
		return key, err
	}

	return key, c.seg.deSecurity()
}

type ControlServer struct {
	conn net.Conn
	seg  ctrSeg
}

func NewControlServer(conn net.Conn) *ControlServer {
	return &ControlServer{
		conn: conn,
		seg: ctrSeg{
			payload: make([]byte, 0, 16),
		},
	}
}

func (s *ControlServer) Next() (CtrCode, error) {
	if err := s.seg.ReadFrom(s.conn); err != nil {
		return 0, err
	}
	return s.seg.CtrCode(), nil
}

func (s *ControlServer) Start() error {
	if err := s.seg.deStart(); err != nil {
		return err
	}

	if err := s.seg.enStart(); err != nil {
		return err
	}
	return s.seg.WriteTo(s.conn)
}

func (s *ControlServer) Security(cfg *tls.Config) (key [16]byte, err error) {
	if err := s.seg.deSecurity(); err != nil {
		return key, err
	}

	{
		tconn := tls.Server(&connWrapClose{Conn: s.conn}, cfg)
		if err := tconn.Handshake(); err != nil {
			return key, err
		}

		if _, err := io.ReadFull(tconn, key[:]); err != nil {
			return key, err
		}

		if err := tconn.Close(); err != nil {
			return key, err
		}
	}

	s.seg.enSecurity()
	return key, s.seg.WriteTo(s.conn)
}

func (s *ControlServer) RegisterSession(smgr SessionMgr) (err error) {
	sess, err := s.seg.deRegisterSessionReq()
	if err != nil {
		s.seg.enRegistrSessionResp(0, err.Error())
		s.seg.WriteTo(s.conn)
		return err
	}

	id, err := smgr.Register(sess)
	if err != nil {
		s.seg.enRegistrSessionResp(0, err.Error())
		s.seg.WriteTo(s.conn)
		return err
	}

	s.seg.enRegistrSessionResp(id, "")
	return s.seg.WriteTo(s.conn)
}
