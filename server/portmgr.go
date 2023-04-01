package server

var PortMgr = &portMgr{}

type portMgr struct {
	udp [65536]uint
	tcp [65536]uint
}

func (p *portMgr) Get(proto string) (port uint16, err error) {

}
