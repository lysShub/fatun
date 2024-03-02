package main

import (
	"context"

	"github.com/lysShub/itun/app/client"
)

// var server netip.AddrPort
// if a, err := net.ResolveTCPAddr("tcp", pxySrvAddr); err != nil {
// 	return nil, err
// } else {
// 	if a.Port == 0 {
// 		a.Port = itun.DefaultPort
// 	}
// 	addr, ok := netip.AddrFromSlice(a.IP)
// 	if !ok {
// 		return nil, pkge.Errorf("invalid proxy server address %s", pxySrvAddr)
// 	} else if addr.Is4In6() {
// 		addr = netip.AddrFrom4(addr.As4())
// 	}
// 	server = netip.AddrPortFrom(addr, uint16(a.Port))
// }

// var addrPort netip.AddrPort
// 	if a, err := net.ResolveTCPAddr("tcp", addr); err != nil {
// 		return err
// 	} else {
// 		if a.Port == 0 {
// 			a.Port = itun.DefaultPort
// 		}

// 		addr, ok := netip.AddrFromSlice(a.IP)
// 		if !ok {
// 			if len(a.IP) == 0 {
// 				addr = relraw.LocalAddr()
// 			} else {
// 				return pkge.Errorf("invalid address %s", a.IP)
// 			}
// 		} else if addr.Is4In6() {
// 			addr = netip.AddrFrom4(addr.As4())
// 		}
// 		addrPort = netip.AddrPortFrom(addr, uint16(a.Port))
// 	}

func main() {

	client.NewClient(context.Background(), nil, nil)

}
