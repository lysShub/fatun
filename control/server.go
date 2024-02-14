package control

import (
	"net"
	"sync"
)

type listenerWrap struct {
	wg sync.WaitGroup
	net.Conn
}

func newListenerWrap(conn net.Conn) *listenerWrap {
	return &listenerWrap{Conn: conn}
}

var _ net.Listener = (*listenerWrap)(nil)

func (l *listenerWrap) Accept() (net.Conn, error) {
	l.wg.Wait()
	defer l.wg.Add(1)
	return l.Conn, nil
}
func (l *listenerWrap) Close() error   { return nil }
func (l *listenerWrap) Addr() net.Addr { return l.Conn.LocalAddr() }

// func Serve(conn net.Conn, cfg *config.Server) {
// 	s := grpc.NewServer() // grpc.UnaryInterceptor(nil)
// 	internal.RegisterControlServer(s, &server{})

// 	if err := s.Serve(newListenerWrap(conn)); err != nil {
// 		log.Fatalf("failed to serve: %v", err)
// 	}

// }

// type server struct {
// 	internal.UnimplementedControlServer
// 	cfg *config.Server
// }

// func (s *server) Crypto(ctx context.Context, crypto *internal.Bool) (*internal.Null, error) {
// 	s.cfg.Crypto = crypto.GetValue()

// 	return &internal.Null{}, nil
// }
