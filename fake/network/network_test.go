package network

import (
	"context"
	"io"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

func Test_Custom_TCP_Flag(t *testing.T) {
	var buildTCP = func(fin bool) header.TCP {
		const PseudoChecksum = 0

		var tcphdr = make(header.TCP, header.TCPMinimumSize)
		f := header.TCPFlagAck
		if fin {
			f |= header.TCPFlagFin
		}
		tcphdr.Encode(&header.TCPFields{
			SrcPort:    19986,
			DstPort:    8080,
			SeqNum:     rand.Uint32(),
			AckNum:     rand.Uint32(),
			DataOffset: header.TCPMinimumSize,
			Flags:      f,
			WindowSize: uint16(rand.Uint32()),
			Checksum:   0,
		})
		tcphdr.SetChecksum(^checksum.Checksum(tcphdr, PseudoChecksum))
		return tcphdr
	}
	var validChecksum = func(tcphdr header.TCP) bool {
		act := ^tcphdr.Checksum()

		tcphdr.SetChecksum(0)
		exp := tcphdr.CalculateChecksum(0)
		ok := act == exp

		tcphdr.SetChecksum(^act)
		return ok
	}

	t.Run("not-need-update-checksum", func(t *testing.T) {
		tcphdr := buildTCP(true)
		require.True(t, validChecksum(tcphdr))

		EnCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.True(t, isCustomFIN(tcphdr))
		require.False(t, tcphdr.Flags().Contains(header.TCPFlagFin))

		DeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.True(t, tcphdr.Flags().Contains(header.TCPFlagFin))
	})

	t.Run("encode-not-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		require.True(t, validChecksum(tcphdr))

		EnCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.False(t, isCustomFIN(tcphdr))
	})

	t.Run("decode-not-custom-fin", func(t *testing.T) {
		tcphdr := buildTCP(false)
		require.True(t, validChecksum(tcphdr))

		DeCustomFIN(tcphdr)
		require.True(t, validChecksum(tcphdr))
		require.False(t, isCustomFIN(tcphdr))
	})

	t.Run("checksum-delta", func(t *testing.T) {
		var old = []byte{0b10, 0b101}
		var new = []byte{0b11, 0b100}

		act := checksum.Checksum(old, 0)
		act = checksum.Combine(act, checksumDelta)

		exp := checksum.Checksum(new, 0)

		require.Equal(t, act, exp)
	})
}

func Test_Custom_Network_Stack(t *testing.T) {

	var cconn, sconn net.Conn
	var stacks []*stack.Stack

	{ // user stack
		var (
			links []*channel.Endpoint
			nics  = []tcpip.NICID{123, 321}
			caddr = tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{10, 0, 1, 1}), Port: 19986}
			saddr = tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{10, 0, 2, 1}), Port: 8080}
		)

		for i, nic := range nics {
			s := stack.New(stack.Options{
				NetworkProtocols:   []stack.NetworkProtocolFactory{NewProtocol},
				TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
				// HandleLocal:        true,
			})
			l := channel.New(4, uint32(1500), "")
			require.Nil(t, s.CreateNIC(nic, l))
			s.AddProtocolAddress(nic, tcpip.ProtocolAddress{
				Protocol: header.IPv4ProtocolNumber,
				AddressWithPrefix: func() tcpip.AddressWithPrefix {
					if i == 0 {
						return caddr.Addr.WithPrefix()
					}
					return saddr.Addr.WithPrefix()
				}(),
			}, stack.AddressProperties{})
			s.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nic}})
			// for !l.IsAttached() {
			// 	runtime.Gosched()
			// }

			stacks = append(stacks, s)
			links = append(links, l)
		}

		var linkUnicom = func(a, b *channel.Endpoint) {
			for {
				pkb := a.ReadContext(context.Background())
				if pkb.IsNil() {
					continue
				} else {
					iphdr := header.IPv4(pkb.ToView().AsSlice())
					tcphdr := header.TCP(iphdr.Payload())
					require.False(t, tcphdr.Flags().Contains(header.TCPFlagFin))
				}

				pkb2 := stack.NewPacketBuffer(stack.PacketBufferOptions{
					Payload: buffer.MakeWithData(pkb.ToView().AsSlice()),
				})
				pkb.DecRef()

				b.InjectInbound(ipv4.ProtocolNumber, pkb2)

			}
		}
		go linkUnicom(links[0], links[1])
		go linkUnicom(links[1], links[0])

		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			var err error
			cconn, err = gonet.DialTCPWithBind(
				context.Background(),
				stacks[0], caddr, saddr,
				ipv4.ProtocolNumber,
			)
			require.NoError(t, err)
			wg.Done()
		}()
		l, err := gonet.ListenTCP(
			stacks[1], saddr,
			ipv4.ProtocolNumber,
		)
		require.NoError(t, err)

		sconn, err = l.Accept()
		require.NoError(t, err)

		wg.Wait()
	}

	go io.Copy(sconn, sconn)

	msg := []byte("hello world")
	n, err := cconn.Write(msg)
	require.NoError(t, err)
	require.Equal(t, len(msg), n)

	var b = make([]byte, 64)
	n, err = cconn.Read(b)
	require.NoError(t, err)
	require.Equal(t, msg, b[:n])

	require.NoError(t, cconn.Close())
	time.Sleep(time.Second)

	ss := stacks[0].Stats()

	t.Log(ss)
}
