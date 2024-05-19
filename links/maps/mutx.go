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
type mutxLinkManager[P peer.Peer] struct {
	ap    *ports.Adapter
	conns *connManager[P]
	mutx  uint8
	mgrs  []*linkManager[P]
}

var _ links.LinksManager[peer.Peer] = (*mutxLinkManager[peer.Peer])(nil)

func NewMutxLinkManager[P peer.Peer](mutx uint8, ttl time.Duration, addr netip.Addr) *mutxLinkManager[P] {
	var m = &mutxLinkManager[P]{
		ap:    ports.NewAdapter(addr),
		conns: newConnManager[P](),
		mutx:  mutx,
		mgrs:  make([]*linkManager[P], max(mutx, 4)),
	}
	for i := range m.mgrs {
		m.mgrs[i] = newLinkManager[P](m.ap, m.conns, ttl)
	}
	return m
}

func (m *mutxLinkManager[P]) Downlink(link links.Downlink) (conn fatcp.Conn[P], clientPort uint16, has bool) {
	return m.get(link.Server.Addr()).Downlink(link)
}
func (m *mutxLinkManager[P]) Add(link links.Uplink, conn fatcp.Conn[P]) (localPort uint16, err error) {
	return m.get(link.Server.Addr()).Add(link, conn)
}
func (m *mutxLinkManager[P]) Uplink(link links.Uplink) (localPort uint16, has bool) {
	return m.get(link.Server.Addr()).Uplink(link)
}
func (m *mutxLinkManager[P]) Close() error {
	return m.ap.Close()
}

func (m *mutxLinkManager[P]) get(server netip.Addr) *linkManager[P] {
	s := server.AsSlice()
	return m.mgrs[s[len(s)-1]%m.mutx]
}
