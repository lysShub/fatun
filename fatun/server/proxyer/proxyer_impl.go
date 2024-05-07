package proxyer

import (
	"log/slog"

	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/netkit/packet"
)

type controlImpl Proxy

type controlImplPtr = *controlImpl

var _ control.Handler = (controlImplPtr)(nil)

func (c *controlImpl) IPv6() bool {
	return true
}
func (c *controlImpl) InitConfig(cfg *control.Config) error {
	return nil
}
func (c *controlImpl) AddSession(sess session.Session) (session.ID, error) {
	panic("")
}
func (c *controlImpl) DelSession(id session.ID) error {
	panic("")
}
func (c *controlImpl) PackLoss() float32 {
	return 0
}
func (c *controlImpl) Ping() {

}

type serverImpl Proxy

func (s *serverImpl) Downlink(pkt *packet.Packet, id session.ID) error {
	err := s.conn.Send(s.srvCtx, pkt, id)
	return err
}

func (s *serverImpl) DelSession(sess session.Session) {
	s.server.Logger().Info("del session", slog.String("session", sess.String()))

	(*Proxy)(s).decSession()
}

func (s *serverImpl) Closed() bool { return s.closeErr.Load() != nil }
