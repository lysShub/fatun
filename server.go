package fatun

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/lysShub/fatun/checksum"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/links"
	"github.com/lysShub/fatun/links/maps"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const DefaultPort = 443

type Sender interface {
	Recv(ip *packet.Packet) error
	Send(ip *packet.Packet) error
	Close() error
}

type Server struct {
	// Logger Warn/Error logger
	Logger *slog.Logger

	Listener conn.Listener

	// links manager, notice not call Cleanup()
	Links links.LinksManager

	Senders []Sender

	peer     conn.Peer
	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr errorx.CloseErr
}

func NewServer[P conn.Peer](opts ...func(*Server)) (*Server, error) {
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
		// addr := net.JoinHostPort("", strconv.Itoa(DefaultPort))
		// s.Listener, err = fatcp.Listen[P](addr, &fatcp.Config{}) // todo: 默认udp那个
		if err != nil {
			return nil, s.close(err)
		}
	}
	if s.Links == nil {
		s.Links = maps.NewLinkManager(time.Second*30, s.Listener.Addr().Addr())
	}

	if len(s.Senders) == 0 {
		s.Senders, err = NewDefaultSender(s.Listener.Addr())
		if err != nil {
			return s, s.close(err)
		}
	}

	return s, nil
}

func (s *Server) Serve() (err error) {
	for _, e := range s.Senders {
		go s.recvService(e)
	}
	return s.acceptService()
}

func (s *Server) close(cause error) error {
	if cause != nil {
		s.Logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		s.Logger.Info("server close", errorx.Trace(nil))
	}

	return s.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)

		if s.cancel != nil {
			s.cancel()
		}
		if len(s.Senders) > 0 {
			for _, e := range s.Senders {
				errs = append(errs, e.Close())
			}
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

func (s *Server) serveConn(conn conn.Conn) (_ error) {
	var (
		client = conn.RemoteAddr()
		pkt    = packet.Make(0, s.Listener.MTU())
		t      header.Transport
		peer   = s.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)
	defer func() {
		conn.Close()
		s.Logger.Info("close connect", slog.String("client", client.String()))
	}()

	for {
		err := conn.Recv(peer, pkt.Sets(0, 0xffff))
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
			Process: netip.AddrPortFrom(conn.RemoteAddr().Addr(), t.SourcePort()),
			Proto:   peer.Protocol(),
			Server:  netip.AddrPortFrom(conn.Peer(), t.DestinationPort()),
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

		if err := s.Senders[0].Send(ip); err != nil {
			return s.close(errors.WithStack(err))
		}
	}
}

func (s *Server) recvService(sender Sender) (_ error) {
	var (
		ip   = packet.Make(64, s.Listener.MTU())
		peer = s.peer.Builtin().Reset(0, netip.IPv4Unspecified())
	)
	for {
		err := sender.Recv(ip.Sets(64, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				if debug.Debug() && errors.Is(err, io.ErrShortBuffer) &&
					header.IPVersion(ip.SetHead(64).Bytes()) == 4 {

					// todo: temporary
					ip := header.IPv4(ip.Bytes())
					println("short buff", "ip4 total length:", ip.TotalLength(),
						"src", ip.SourceAddress().String(), "dst", ip.DestinationAddress().String(), "proto", ip.Protocol())
				}

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
		if err := conn.Send(peer, ip); err != nil {
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
