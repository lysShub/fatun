package pack

import (
	"net/netip"
	"unsafe"
)

const W = 4

type Proto = uint8

const (
	ICMP Proto = 1
	TCP        = 6
	UDP        = 17
)

var protoStr = [255]string{
	"HOPOPT", "ICMP", "IGMP", "GGP", "IPv4", "ST", "TCP", "CBT", "EGP", "IGP", "BBN-RCC-MON", "NVP-II", "PUP", "ARGUS", "EMCON", "XNET", "CHAOS", "UDP", "MUX", "DCN-MEAS", "HMP", "PRM", "XNS-IDP", "TRUNK-1", "TRUNK-2", "LEAF-1", "LEAF-2", "RDP", "IRTP", "ISO-TP4", "NETBLT", "MFE-NSP", "MERIT-INP", "DCCP", "3PC", "IDPR", "XTP", "DDP", "IDPR-CMTP", "TP++", "IL", "IPv6", "SDRP", "IPv6-Route", "IPv6-Frag", "IDRP", "RSVP", "GRE", "DSR", "BNA", "ESP", "AH", "I-NLSP", "SWIPE", "NARP", "MOBILE", "TLSP", "SKIP", "IPv6-ICMP", "IPv6-NoNxt", "IPv6-Opts", "CFTP", "SAT-EXPAK", "KRYPTOLAN", "RVD", "IPPC", "SAT-MON", "VISA", "IPCV", "CPNX", "CPHB", "WSN", "PVP", "BR-SAT-MON", "SUN-ND", "WB-MON", "WB-EXPAK", "ISO-IP", "VMTP", "SECURE-VMTP", "VINES", "TTP", "IPTM", "NSFNET-IGP", "DGP", "TCF", "EIGRP", "OSPFIGP", "Sprite-RPC", "LARP", "MTP", "AX.25", "IPIP", "MICP", "SCC-SP", "ETHERIP", "ENCAP", "GMTP", "IFMP", "PNNI", "PIM", "ARIS", "SCPS", "QNX", "A/N", "IPComp", "SNP", "Compaq-Peer", "IP", "IPX", "IPV6", "SCTP", "FC", "RSVP-E2E-IGNORE", "Mobility Header", "UDPLite", "MPLS-in-IP", "manet", "HIP", "Shim6", "WESP", "ROHC",
}

func Packe(b []byte, proto Proto, ip netip.Addr) []byte {
	n := len(b)
	if n+W < cap(b) {
		tb := make([]byte, n+W)
		copy(tb[0:], b[0:])
		b = tb
	} else {
		b = b[:n+W]
	}

	*(*[4]byte)(unsafe.Pointer(&b[n])) = netip.MustParseAddr(ip.String()).As4()
	return b
}

func Parse(b []byte) (n int, proto Proto, ip netip.Addr) {
	var _ = b[5]

	// n := len(b) - W
	// return *(*[4]byte)(unsafe.Pointer(&b[n]))

	return
}

func Checksum(d [20]byte) uint16 {
	var S uint32
	for i := 0; i < 20; i = i + 2 {
		S = S + uint32(d[i])<<8 + uint32(d[i+1])
		if S>>16 > 0 {
			S = S&0xffff + 1
		}
	}

	return uint16(65535) - uint16(S)
}
