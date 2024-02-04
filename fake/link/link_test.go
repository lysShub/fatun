package link_test

import (
	"context"
	"fmt"
	"io"
	"itun/fake/link"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

func Test_Custom_Network_Stack(t *testing.T) {

	var cconn, sconn net.Conn
	var links []*link.Endpoint

	{ // user stack
		var (
			stacks []*stack.Stack
			nics   = []tcpip.NICID{123, 321}
			caddr  = tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{10, 0, 1, 1}), Port: 19986}
			saddr  = tcpip.FullAddress{Addr: tcpip.AddrFrom4([4]byte{10, 0, 2, 1}), Port: 8080}
		)

		for i, nic := range nics {
			s := stack.New(stack.Options{
				NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
				TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
				// HandleLocal:        true,
			})
			l := link.New(4, uint32(1500))
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

			stacks = append(stacks, s)
			links = append(links, l)
		}

		var linkUnicom = func(a, b *link.Endpoint) {
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

	fmt.Println(links[0].SeqAck())
	fmt.Println(links[1].SeqAck())
}
