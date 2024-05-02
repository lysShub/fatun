package proxyer

import (
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/session"
	"github.com/lysShub/sockit/packet"
)

type controlImpl Proxyer

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

type serverImpl Proxyer

func (s *serverImpl) Downlink(pkt *packet.Packet, id session.ID) error {
	err := s.conn.Send(s.srvCtx, pkt, id)
	return err
}

func (s *serverImpl) DecSession() {
	(*Proxyer)(s).decSession()
}
