//go:build !windows
// +build !windows

package itun

func GetSubMask(ip net.IP) net.IPMask {

	return ip.DefaultMask()
}
