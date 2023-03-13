package client

import (
	"crypto/rand"
	"fmt"
	"github.com/lysShub/go-divert"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestXxx(t *testing.T) {
	var b = []byte{0x08, 0x00, 0x00, 0x00, 0x00, 0x01, 0x10, 0xe2, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69}

	r := checkSum(b)

	t.Log(r)
}

func TestSetMap(t *testing.T) {

	// ping dest:
	// 114.114.114.114

	lip := getLocalIP()
	var e = dialEvent{
		pid:   3412,
		name:  "test.exe",
		laddr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{lip[0], lip[1], lip[2], lip[3]}), 3241),
		raddr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{114, 21, 8, 11}), 3241),
	}

	var p = &Client{
		serverIP: net.IPv4(114, 114, 114, 114),

		setedCh: make(chan [6]byte, 8),
		m:       &sync.Mutex{},
	}
	start1 := time.Now()
	err := p.setMap(e)
	require.NoError(t, err)

	start2 := time.Now()
	err = p.setMap(e)
	require.NoError(t, err)

	fmt.Println(time.Since(start1))
	fmt.Println(time.Since(start2))
}

func TestSend(t *testing.T) {
	h, err := divert.Open("!loopback", divert.LAYER_NETWORK, 111, 0)
	require.NoError(t, err)
	defer h.Close()

	addr := &divert.Address{}
	addr.Header.Layer = divert.LAYER_NETWORK
	addr.Header.Event = divert.EVENT_NETWORK_PACKET
	addr.Header.SetOutbound(true)
	var b = []byte{0x45, 0x00, 0x00, 0x3c, 0x43, 0xfc, 0x00, 0x00, 0x80, 0x01, 0x00, 0x00, 0xac, 0x1f, 0x01, 0xf4, 0x6e, 0xf2, 0x44, 0x42, 0x08, 0x00, 0x4d, 0x5a, 0x00, 0x01, 0x00, 0x01, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69}

	n, err := h.Send(b, addr)
	require.NoError(t, err)
	fmt.Println(n)
}

func Test_Send_Over_Size(t *testing.T) {

	var payload = make([]byte, 1000)
	rand.Read(payload)

	var b = &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID: 2321 & 0xffff, Seq: 1,
			Data: payload,
		},
	}
	icmpData, err := b.Marshal(nil)
	require.NoError(t, err)

	var ih = &ipv4.Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 20 + len(icmpData),
		ID:       2321,
		Flags:    ipv4.DontFragment,
		FragOff:  0,
		TTL:      128,
		Protocol: 1,
		Checksum: 0,
		Src:      getLocalIP(),
		Dst:      net.IPv4(8, 8, 8, 8),
	}
	ipHeader, err := ih.Marshal()
	require.NoError(t, err)

	ih.Checksum = int(checkSum(ipHeader))
	ipHeader, err = ih.Marshal()
	require.NoError(t, err)

	var data = append(ipHeader, icmpData...)

	addr := &divert.Address{}
	addr.Header.Layer = divert.LAYER_NETWORK
	addr.Header.Event = divert.EVENT_NETWORK_PACKET
	addr.Header.SetOutbound(true)

	h, err := divert.Open("false", divert.LAYER_NETWORK, 111, 0)
	require.NoError(t, err)

	_, err = h.Send(data, addr)
	require.NoError(t, err)
}

func Test_Read_TCP(t *testing.T) {
	h, err := divert.Open("!loopback and tcp and outbound", divert.LAYER_NETWORK, 111, 0)
	require.NoError(t, err)

	var da = make([]byte, 1536)
	for {
		n, _, err := h.Recv(da)
		require.NoError(t, err)

		var ih = &ipv4.Header{}

		err = ih.Parse(da[:n])
		require.NoError(t, err)

		t.Log(time.Now(), ih.Dst)

	}
}
