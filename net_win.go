//go:build windows
// +build windows

package itun

import (
	"net"
	"strings"

	"golang.org/x/sys/windows"
)

func GetLocalIP() net.IP {
	if conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(114, 114, 114, 114), Port: 80}); err != nil {
		return net.ParseIP("127.0.0.1")
	} else {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP
	}
}

func GetSubMask(ip net.IP) (m net.IPMask) {
	if ip == nil {
		ip = GetLocalIP()
	}

	var ai = &windows.IpAdapterInfo{}

	var ol uint32 = 0
	err := windows.GetAdaptersInfo(ai, &ol)
	if err == windows.ERROR_BUFFER_OVERFLOW {
		if err = windows.GetAdaptersInfo(ai, &ol); err != nil {
			return ip.DefaultMask()
		}

		ip := GetLocalIP()
		if m = iteratorAdapterInfos(ip, ai); m != nil {
			return
		}
	}
	return ip.DefaultMask()
}

func iteratorAdapterInfos(ip net.IP, ai *windows.IpAdapterInfo) net.IPMask {
	if ai == nil {
		return nil
	} else {
		if m := iteratorIpAddress(ip, ai.IpAddressList); m != nil {
			return m
		}
	}

	if ai.Next != nil {
		return iteratorAdapterInfos(ip, ai.Next)
	}
	return nil
}

func iteratorIpAddress(ip net.IP, i windows.IpAddrString) net.IPMask {
	ii := ((_iaddr)(i.IpAddress.String)).IP()
	if ip.Equal(ii) {
		m := (_iaddr)(i.IpMask.String)
		if mip := m.IP(); mip == nil {
			return nil
		} else {
			if mip.To4() != nil { // IPv4
				return net.CIDRMask(m.ones(), 32)
			} else {
				return net.CIDRMask(m.ones(), 128)
			}
		}

	}

	if i.Next != nil {
		return iteratorIpAddress(ip, *i.Next)
	} else {
		return nil
	}
}

type _iaddr [16]byte

func (a _iaddr) IP() net.IP {
	for i := 0; i < 16; i++ {
		if a[i] == 0 {
			return net.ParseIP(string(a[:i]))
		}
	}
	return nil
}

func (a _iaddr) ones() int {
	var n int
	s := a.IP().String()
	//  1 3 7 15 31 63 127 255
	n += 8 * strings.Count(s, "255")
	// s = strings.ReplaceAll(s, "255", "")
	n += 7 * strings.Count(s, "127")
	s = strings.ReplaceAll(s, "127", "")
	n += 6 * strings.Count(s, "63")
	s = strings.ReplaceAll(s, "63", "")
	n += 5 * strings.Count(s, "31")
	s = strings.ReplaceAll(s, "31", "")
	n += 4 * strings.Count(s, "15")
	// s = strings.ReplaceAll(s, "15", "")
	n += 3 * strings.Count(s, "7")
	s = strings.ReplaceAll(s, "7", "")
	n += 2 * strings.Count(s, "3")
	s = strings.ReplaceAll(s, "3", "")
	n += strings.Count(s, "1")

	return n
}
