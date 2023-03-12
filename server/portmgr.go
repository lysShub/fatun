package server

// 当一个Proxy-Server同时代理多个Client时,
// 可能造成端口冲突, 所以需要一个端口管理工具.
type PortMgr struct{}

func (p *PortMgr) Client(cPort uint16) (sPort uint16) {
	// TODO: 暂时只代理不会有多客户端
	return cPort
}

func (p *PortMgr) Server(sPort uint16) (cPort uint16) {
	return sPort
}
