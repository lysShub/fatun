package rawtcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"itun/proto"
	"net"
	"sync"

	"github.com/lysShub/go-divert"
	"github.com/lysShub/go-divert/embed"
	"github.com/lysShub/relraw"
	rtcp "github.com/lysShub/relraw/tcp"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type proxyClient struct {
	mtu              uint16
	locAddr, srvAddr *net.TCPAddr
	NetworkProto     tcpip.NetworkProtocolNumber

	raw relraw.Raw

	cfg    *tls.Config
	secret [16]byte

	// CtrConn
	stack   *stack.Stack
	link    *channel.Endpoint
	ctrConn net.Conn
	ctr     *proto.ControlClient

	// PxyConn
	sessionMap      map[proto.Session]uint16
	sessionMapMu    sync.RWMutex
	ipstackUplink   relraw.IPStack
	ipstackDownlink relraw.IPStack

	sessionMgr SessionMgr
}

var dll *divert.DivertDLL

func Dial(srv *net.TCPAddr) (*proxyClient, error) {
	var err error
	dll, err = divert.LoadDivert(embed.DLL, embed.Sys)
	if err != nil {
		return nil, err
	}

	var p = &proxyClient{}

	p.raw, err = rtcp.NewRawWithDivert(nil, srv, dll)
	if err != nil {
		return nil, err
	}

	return p, p.connCtr(context.Background())
}

const nicid tcpip.NICID = 12345

func (p *proxyClient) newStack() error {
	opt := stack.Options{
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
		// HandleLocal:        true,
	}
	switch p.NetworkProto {
	case header.IPv4ProtocolNumber:
		opt.NetworkProtocols = []stack.NetworkProtocolFactory{ipv4.NewProtocol}
	case header.IPv6ProtocolNumber:
		opt.NetworkProtocols = []stack.NetworkProtocolFactory{ipv4.NewProtocol}
	default:
	}

	p.stack = stack.New(opt)
	p.link = channel.New(8, uint32(p.mtu), "")
	if err := p.stack.CreateNIC(nicid, p.link); err != nil {
		return errors.New(err.String())
	}
	p.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          p.NetworkProto,
		AddressWithPrefix: tcpip.AddrFromSlice(p.locAddr.IP).WithPrefix(),
	}, stack.AddressProperties{})
	p.stack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})

	return nil
}

func (p *proxyClient) ctrConnUplinkWorker() {
	for {
		pkb := p.link.ReadContext(context.Background())

		b := pkb.ToView().AsSlice()

		Segment(b).SetCtrseg()

		if _, err := p.raw.Write(b); err != nil {
			panic(err)
		}

		pkb.DecRef()
	}
}

func (p *proxyClient) rawConnDownlinkWorker() {
	var (
		b       = make([]byte, p.mtu)
		n       int
		err     error
		session proto.Session
	)
	for {
		n, err = p.raw.Read(b)
		if err != nil {
			panic(err)
		} else if n == 0 {
			fmt.Println("n==0") // 判断释放p close
		}

		seg := Segment(b[:n])
		if seg.IsPxyseg() {
			id, proto := seg.Pxyseg()
			session = p.sessionMgr.GetSession(id)
			if !session.Validate() {
				continue
			}

			switch proto {
			case header.TCPProtocolNumber:
				seg.ResetPxyseg()
				seg = p.ipstackDownlink.UpdateHeader(seg)

			case header.UDPProtocolNumber:
			default:
			}
		} else if seg.IsCtrseg() {
			// wait attach
			pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(b[:n])})
			p.link.InjectInbound(p.NetworkProto, pkb)
		} else {
			fmt.Println("不知道啥玩意")
		}
	}
}

func (p *proxyClient) initCtrConn(ctx context.Context) error {
	if p.cfg != nil {
		if key, err := p.ctr.Security(p.cfg); err != nil {
			return err
		} else {
			p.secret = key
		}
	}

	return p.ctr.Start()
}

// connCtr connect CtrConn
func (p *proxyClient) connCtr(ctx context.Context) (err error) {
	if err := p.newStack(); err != nil {
		return err
	}

	dialCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	//  uplink worker will work always
	go p.ctrConnUplinkWorker()

	// downlink worker must return after connCtr
	go p.rawConnDownlinkWorker()

	if p.ctrConn, err = gonet.DialTCPWithBind(
		dialCtx,
		p.stack,
		tcpip.FullAddress{
			NIC:  nicid,
			Addr: tcpip.AddrFromSlice(p.locAddr.IP),
			Port: uint16(p.locAddr.Port),
		},
		tcpip.FullAddress{
			NIC:  nicid,
			Addr: tcpip.AddrFromSlice(p.srvAddr.IP),
			Port: uint16(p.srvAddr.Port),
		},
		p.NetworkProto,
	); err != nil {
		cancel(fmt.Errorf("dial %s", err))
		return context.Cause(dialCtx)
	}

	if err := p.initCtrConn(dialCtx); err != nil {
		cancel(fmt.Errorf("initCtrConn %s", err))
		return context.Cause(dialCtx)
	}

	return context.Cause(dialCtx)
}

type WriteOpt uint8

const (
	_ WriteOpt = iota
	Ctrseg
	PxysegTCP
	PxysegUDP
)

// Write 发送Pxyseg数据包
func (p *proxyClient) Write(ip []byte) (int, error) {
	// todo: 判断init完成

	s := proto.GetSession(ip)
	if !s.Validate() {
		return 0, fmt.Errorf("get invalid session %s", s.String())
	}

	if s.TransportProtocol() == header.UDPProtocolNumber {
		ip = UDP2TCP(ip)
	}

	id := p.sessionMgr.GetID(s)
	if id == 0 {
		var err error
		id, err = p.ctr.RegisterSession(s)
		if err != nil {
			return 0, err
		}
		p.sessionMgr.Add(s, id)
	}

	ip = p.ipstackUplink.UpdateHeader(ip)

	Segment(ip).SetPxyseg(id, s.TransportProtocol())

	return p.raw.Write(ip)
}
