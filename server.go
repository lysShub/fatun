package fatun

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/checksum"
	"github.com/lysShub/fatun/links"
	"github.com/lysShub/fatun/links/maps"
	"github.com/lysShub/fatun/peer"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const DefaultPort = 443

type Sender interface {
	Recv(ctx context.Context, ip *packet.Packet) error
	Send(ctx context.Context, ip *packet.Packet) error
	Close() error
}

type Server struct {
	// Logger Warn/Error logger
	Logger *slog.Logger

	Listener fatcp.Listener

	// links manager, notice not call Cleanup()
	Links links.LinksManager

	Sender Sender

	peer     peer.Peer
	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr errorx.CloseErr
}

func NewServer[P peer.Peer](opts ...func(*Server)) (*Server, error) {
	var s = &Server{peer: *new(P)}
	s.srvCtx, s.cancel = context.WithCancel(context.Background())

	for _, opt := range opts {
		opt(s)
	}

	if s.Logger == nil {
		s.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	var err error
	if s.Listener == nil {
		addr := net.JoinHostPort("", strconv.Itoa(DefaultPort))
		s.Listener, err = fatcp.Listen[P](addr, &fatcp.Config{})
		if err != nil {
			return nil, s.close(err)
		}
	}
	if s.Links == nil {
		s.Links = maps.NewLinkManager(time.Second*30, s.Listener.Addr().Addr())
	}

	if s.Sender == nil {
		s.Sender, err = NewDefaultSender(s.Listener.Addr())
		if err != nil {
			return s, s.close(err)
		}
	}

	return s, nil
}

func (s *Server) Serve() (err error) {
	go s.recvService()
	return s.acceptService()
}

func (s *Server) close(cause error) error {
	return s.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)

		if s.cancel != nil {
			s.cancel()
		}
		if s.Sender != nil {
			errs = append(errs, s.Sender.Close())
		}
		if s.Links != nil {
			errs = append(errs, s.Links.Close())
		}
		if s.Listener != nil {
			errs = append(errs, s.Listener.Close())
		}
		return
	})
}

func (s *Server) acceptService() (_ error) {
	for {
		conn, err := s.Listener.AcceptCtx(s.srvCtx)
		if err != nil {
			if errorx.Temporary(err) {
				s.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return s.close(err)
			}
		}

		go s.serveConn(conn)
	}
}

func (s *Server) serveConn(conn fatcp.Conn) (_ error) {
	var (
		client = conn.RemoteAddr()
		pkt    = packet.Make(0, s.Listener.MTU())
		t      header.Transport
		peer   = s.peer.Make()
	)
	defer func() {
		conn.Close()
		s.Logger.Info("close connect", slog.String("client", client.String()))
	}()

	for {
		err := conn.Recv(s.srvCtx, peer, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				s.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				s.Logger.Error(err.Error(), errorx.Trace(err), slog.String("client", client.String()))
				return nil
			}
		}

		switch peer.Protocol() {
		case header.TCPProtocolNumber:
			t = header.TCP(pkt.Bytes())
		case header.UDPProtocolNumber:
			t = header.UDP(pkt.Bytes())
		default:
			s.Logger.Warn(fmt.Sprintf("not support protocol %d", peer.Protocol()), errorx.CallTrace())
			continue
		}

		up := links.Uplink{
			Process: netip.AddrPortFrom(conn.LocalAddr().Addr(), t.SourcePort()),
			Proto:   peer.Protocol(),
			Server:  netip.AddrPortFrom(peer.Peer(), t.DestinationPort()),
		}
		localPort, has := s.Links.Uplink(up)
		if !has {
			localPort, err = s.Links.Add(up, conn)
			if err != nil {
				s.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
		}

		down := links.Downlink{
			Server: up.Server,
			Proto:  up.Proto,
			Local:  netip.AddrPortFrom(s.Listener.Addr().Addr(), localPort),
		}
		ip := checksum.Server(pkt, down)

		if err := s.Sender.Send(s.srvCtx, ip); err != nil {
			return s.close(errors.WithStack(err))
		}
	}
}

func (s *Server) recvService() (_ error) {
	var (
		ip       = packet.Make(0, s.Listener.MTU())
		overhead = max(s.Listener.Overhead()-header.IPv4MinimumSize, 0)
		peer     = s.peer.Make()
	)
	for {
		err := s.Sender.Recv(s.srvCtx, ip.Sets(overhead, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				s.Logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return s.close(err)
			}
		}

		link, err := links.StripIP(ip)
		if err != nil {
			s.Logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}

		conn, port, has := s.Links.Downlink(link)
		if !has {
			// s.Logger.Warn("links manager not record", slog.String("downlin", link.String()))
			continue
		}
		peer.Reset(link.Proto, link.Server.Addr())
		switch peer.Protocol() {
		case header.TCPProtocolNumber:
			header.TCP(ip.Bytes()).SetDestinationPort(port)
		case header.UDPProtocolNumber:
			header.UDP(ip.Bytes()).SetDestinationPort(port)
		default:
			return errors.Errorf("not support protocol %d", peer.Protocol())
		}
		if err := conn.Send(s.srvCtx, peer, ip); err != nil {
			// todo: 如果是已经删除的downlinker, 应该从links中删除，对于tcp，还应该回复RST
			s.Logger.Warn(err.Error(), errorx.Trace(err))
		}
	}
}

// todo: optimzie
func ifaceByAddr(laddr netip.Addr) (*net.Interface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, i := range ifs {
		if i.Flags&net.FlagRunning == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, addr := range addrs {
			if e, ok := addr.(*net.IPNet); ok && e.IP.To4() != nil {
				if netip.AddrFrom4([4]byte(e.IP.To4())) == laddr {
					return &i, nil
				}
			}
		}
	}

	return nil, errors.Errorf("not found adapter %s mtu", laddr.String())
}
