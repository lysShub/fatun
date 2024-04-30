package proxyer

import (
	"github.com/lysShub/fatun/control"
	"github.com/lysShub/fatun/session"
)

type controlImpl Proxyer

type controlImplPtr = *controlImpl

var _ control.Handler = (controlImplPtr)(nil)

func (c *controlImpl) IPv6() bool {
	return true
}
func (c *controlImpl) InitConfig(cfg *control.Config) error {
	c.logger.Info("inited config")
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

// type serverImpl Proxyer
// type serverImplPrt = *serverImpl

// func (c *serverImpl) Downlink(pkt *packet.Packet, id session.ID) error {
// 	return (*Proxyer)(c).downlink(pkt, id)
// }
