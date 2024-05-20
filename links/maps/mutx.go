package maps

import (
	"net/netip"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatun/links"
	"github.com/lysShub/fatun/peer"
	"github.com/lysShub/fatun/ports"
)

// mutx by server ip address last byte
type mutxLinkManager struct {
	ap   *ports.Adapter
	mutx uint8
	mgrs []*linkManager
}

var _ links.LinksManager = (*mutxLinkManager)(nil)

func NewMutxLinkManager(mutx uint8, ttl time.Duration, addr netip.Addr) *mutxLinkManager {
	var m = &mutxLinkManager{
		ap:   ports.NewAdapter(addr),
		mutx: mutx,
		mgrs: make([]*linkManager, max(mutx, 4)),
	}
	for i := range m.mgrs {
		m.mgrs[i] = newLinkManager(m.ap, ttl)
	}
	return m
}

func (m *mutxLinkManager) Downlink(link links.Downlink) (conn fatcp.Conn[peer.Peer], clientPort uint16, has bool) {
	return m.get(link.Server.Addr()).Downlink(link)
}
func (m *mutxLinkManager) Add(link links.Uplink, conn fatcp.Conn[peer.Peer]) (localPort uint16, err error) {
	return m.get(link.Server.Addr()).Add(link, conn)
}
func (m *mutxLinkManager) Uplink(link links.Uplink) (localPort uint16, has bool) {
	return m.get(link.Server.Addr()).Uplink(link)
}
func (m *mutxLinkManager) Close() error {
	return m.ap.Close()
}

func (m *mutxLinkManager) get(server netip.Addr) *linkManager {
	s := server.AsSlice()
	return m.mgrs[s[len(s)-1]%m.mutx]
}
