//go:build linux
// +build linux

package server

import (
	"context"
	"crypto/tls"
	"itun"
	"net/netip"

	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/tcp/bpf"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	TLS *tls.Config
}

type Server struct {
	Config

	Addr netip.AddrPort
}

func ListenAndServer(ctx context.Context, addr string, cfg Config) error {
	a, err := netip.ParseAddrPort(addr)
	if err != nil {
		return err
	}

	l, err := bpf.ListenWithBPF(a)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rconn, err := l.Accept()
		if err != nil {
			return err
		}

		s := &Handler{
			Raw: rconn,
		}

		go s.Do(ctx)
	}
}

type Handler struct {
	Raw relraw.RawConn

	mgrRaw  *itun.TCPConn
	MgrConn *tls.Conn

	Srv *Server
}

func NewHandler(conn relraw.RawConn, cf Config) (*Handler, error) {

	return nil, nil
}

func (s *Handler) Do(ctx context.Context) error {
	go func() {
		var ip = header.IPv4(make([]byte, 1536))
		for {
			if n, err := s.Raw.Read(ip); err != nil {
				panic(err)
			} else if header.IPVersion(ip) != 4 {
				continue
			} else if n < header.IPv4MinimumSize+header.IPv6MinimumSize {
				continue
			}

			tcphdr := header.TCP(ip.Payload())

			if itun.IsMgrSeg(tcphdr) {
				if _, err := s.mgrRaw.InjectIP(ip); err != nil {
					panic(err)
				}
			} else {
				// 如果握手没完成, 需要报错
			}
		}
	}()

	if err := s.handshake(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Handler) handshake(ctx context.Context) error {
	// tls handshake
	s.MgrConn = tls.Client(s.mgrRaw, s.Srv.Config.TLS)

	// itun init config

	return nil
}
