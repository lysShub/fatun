package maps

import (
	"fmt"
	"itun/pack"
	"net/netip"
)

/*
	代理映射：
		代理映射只要满足五元组即可, 即一个连接的唯一性由五元组决定。
			上行映射：
			map[{
				srcAddrPort,
				proto,
				dstAddrPort
			}]locPort

			下行映射：map[{
				dstAddrPort,
				proto,
				locPort
			}]srcAddrPort

		由于本机IP是固定，所以可以locAddr用locPort代替。

*/

type Map struct {
	up   *upMap
	down *downMap
}

func NewMap(laddr netip.Addr) (*Map, error) {
	var r = &Map{}
	var err error
	r.up, err = newUpMap(laddr)
	if err != nil {
		return nil, err
	}
	r.down = newDownMap()
	return r, nil
}

func (m *Map) UpGetUDP(src, dst netip.AddrPort) (locPort uint16, newLocPort bool, err error) {
	defer func() {
		if err == nil {
			m.down.Reg(dst, pack.UDP, locPort, src)
		}
	}()
	return m.up.GetUDP(src, dst)
}

func (m *Map) UpGetTCP(src, dst netip.AddrPort) (locPort uint16, newLocPort bool, err error) {
	defer func() {
		if err == nil {
			m.down.Reg(dst, pack.TCP, locPort, src)
		}
	}()
	return m.up.GetTCP(src, dst)
}

func (m *Map) Rel(src, dst netip.AddrPort, proto pack.Proto) (err error) {
	locPort := uint16(0)
	switch proto {
	case pack.TCP:
		locPort, _, err = m.up.rawGetTCP(src, dst)
	case pack.UDP:
		locPort, _, err = m.up.rawGetUDP(src, dst)
	default:
		panic(fmt.Errorf("not support proto %s", proto))
	}
	defer func() {
		if err == nil {
			m.down.Del(src, locPort, proto)
		}
	}()

	return m.up.Rel(src, dst, proto)
}

func (m *Map) DownGetUDP(dst netip.AddrPort, locPort uint16) (src netip.AddrPort, has bool) {
	return m.down.GetUDP(dst, locPort)
}

func (m *Map) DownGetTCP(dst netip.AddrPort, locPort uint16) (src netip.AddrPort, has bool) {
	return m.down.GetTCP(dst, locPort)
}

func (m *Map) Clsoe() error {
	return nil
}
