package pack

import (
	"unsafe"
)

// use for structure ipv4(tcp) header
type IPHdr [20]byte

func newIpHdr(localIP [4]byte) *IPHdr {
	var hdr IPHdr
	hdr[0] = 0x45                                   // version:4  hdrLen: 5(*4)
	hdr[1] = 0                                      // TOS:0
	hdr[2], hdr[3] = 0, 0                           // Total Length
	hdr[4], hdr[5] = 0, 0                           // ID:0
	hdr[6], hdr[7] = 0x20, 0                        // Flag: DF, Offset:0
	hdr[8] = 64                                     // TTL:64
	hdr[9] = 6                                      // Proto: 6(TCP)
	hdr[10], hdr[11] = 0, 0                         // Checksum
	hdr[12], hdr[13], hdr[14], hdr[15] = 0, 0, 0, 0 // source IP
	copy(hdr[16:], unsafe.Slice(&localIP[0], 4))    // dest IP

	return &hdr
}

func (i *IPHdr) SetTotalLen(n uint16) {
	*(*uint16)(unsafe.Pointer(&i[2])) = n
}

func (i *IPHdr) SetID(id uint16) {
	*(*uint16)(unsafe.Pointer(&i[4])) = id
}

func (i *IPHdr) SetSrcIP(ip [4]byte) {
	*(*[4]byte)(unsafe.Pointer(&i[12])) = ip
}

func (i *IPHdr) SetChecksum(sum uint16) {
	*(*uint16)(unsafe.Pointer(&i[10])) = sum
}
