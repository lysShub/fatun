//go:build windows
// +build windows

package session

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun/app"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/errorx"
	"github.com/lysShub/sockit/helper/ipstack"
	"github.com/lysShub/sockit/packet"
	"github.com/lysShub/sockit/test"
	"github.com/lysShub/sockit/test/debug"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/net"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type sess struct {
	client Client
	id     session.ID
	cnt    atomic.Uint32

	inject     *divert.Handle
	injectAddr *divert.Address
	ipstack    *ipstack.IPStack

	srvCtx    context.Context
	srvCancel context.CancelFunc
	closeErr  atomic.Pointer[error]
}

func newSession(client Client, id session.ID, firstIP []byte) (Session, error) {
	var s = &sess{
		client: client,
		id:     id,

		injectAddr: &divert.Address{},
	}
	var err error

	sid := session.FromIP(firstIP)
	if idx, err := getIfidxByAddr(sid.Src.Addr()); err != nil {
		return nil, err
	} else {
		s.injectAddr.Network().IfIdx = idx
	}

	filter := fmt.Sprintf(
		"%s and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		session.ProtoStr(sid.Proto), sid.Src.Addr().String(), sid.Src.Port(), sid.Dst.Addr().String(), sid.Dst.Port(),
	)
	s.inject, err = divert.Open(filter, divert.Network, client.DivertPriority(), 0)
	if err != nil {
		return nil, err
	}
	s.ipstack, err = ipstack.New(sid.Src.Addr(), sid.Dst.Addr(), tcpip.TransportProtocolNumber(sid.Proto))
	if err != nil {
		return nil, err
	}

	s.srvCtx, s.srvCancel = context.WithCancel(context.Background())

	go s.uplinkService(firstIP)
	s.keepalive()
	return s, nil
}

func (s *sess) close(cause error) error {
	if s.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		s.srvCancel()

		if s.inject != nil {
			if err := s.inject.Close(); err != nil {
				cause = err
			}
		}

		if s.client != nil {
			s.client.Release(s.id)
		}

		if cause != nil {
			s.closeErr.Store(&cause)
			s.client.Logger().Warn("session close", cause.Error(), errorx.TraceAttr(cause))
		} else {
			s.client.Logger().Info("session close")
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *sess) uplinkService(initIP []byte) error {
	var pkt = packet.Make(64, 0, s.client.MTU()).Append(initIP)

	ipver := header.IPVersion(initIP)
	for {
		s.cnt.Add(1)

		switch ipver {
		case 4:
			pkt.SetHead(pkt.Head() + int(header.IPv4(pkt.Bytes()).HeaderLength()))
		case 6:
			pkt.SetHead(pkt.Head() + header.IPv6FixedHeaderSize)
		default:
			panic("")
		}
		if err := s.client.Uplink(pkt, session.ID(s.id)); err != nil {
			return s.close(err)
		}

		n, err := s.inject.RecvCtx(s.srvCtx, pkt.Sets(64, 0xffff).Bytes(), nil)
		if err != nil {
			return s.close(err)
		} else {
			pkt.SetData(n)
		}
	}
}

func (s *sess) Inject(pkt *packet.Packet) error {
	s.ipstack.AttachInbound(pkt)
	if debug.Debug() {
		test.ValidIP(test.T(), pkt.Bytes())
	}

	_, err := s.inject.Send(pkt.Bytes(), s.injectAddr)
	if err != nil {
		return s.close(err)
	}

	s.cnt.Add(1)
	return nil
}

func (s *sess) keepalive() {
	const magic uint32 = 0x23df83a0
	switch s.cnt.Load() {
	case magic:
		s.close(errors.WithStack(app.KeepaliveExceeded))
	default:
		s.cnt.Store(magic)
		time.AfterFunc(time.Minute, s.keepalive) // todo: from config
	}
}

func (s *sess) Close() error { return s.close(nil) }

func getIfidxByAddr(addr netip.Addr) (uint32, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return 0, errors.WithStack(err)
	}
	a := addr.String()
	for _, e := range ifs {
		for _, addr := range e.Addrs {
			if strings.HasPrefix(addr.Addr, a) {
				return uint32(e.Index), nil
			}
		}
	}
	return 0, errors.Errorf("not find adapter with address %s", a)
}
