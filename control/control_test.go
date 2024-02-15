package control

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/lysShub/itun/control/internal"
	"github.com/stretchr/testify/require"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type server1 struct {
	internal.UnimplementedControlServer
}

func (s *server1) Crypto(ctx context.Context, crypto *internal.Bool) (*internal.Null, error) {
	fmt.Println("Crypto: ", crypto.Val)
	return &internal.Null{}, nil
}

func (s *server1) IPv6(ctx context.Context, ipv6 *internal.Null) (*internal.Bool, error) {
	return &internal.Bool{Val: false}, nil
}

func Test_Wrap_Listener(t *testing.T) {
	var saddr = &net.TCPAddr{Port: 8080}

	t.Run("t", func(t *testing.T) {
		go func() {

			var conn net.Conn
			{
				l, err := net.ListenTCP("tcp", saddr)
				if err != nil {
					log.Fatalf("failed to listen: %v", err)
				}
				conn, err = l.AcceptTCP()
				require.NoError(t, err)
				require.NoError(t, l.Close())
			}

			s := grpc.NewServer()
			internal.RegisterControlServer(s, &server1{})

			if err := s.Serve(newListenerWrap(context.Background(), conn)); err != nil {
				fmt.Println("failed to serve: ", err)
			}
		}()

		time.Sleep(time.Second)
		{ // client

			tcp, err := net.Dial("tcp", saddr.String())
			require.NoError(t, err)

			opts := []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.FailOnNonTempDialError(true),
				grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {

					return tcp, nil
				}),
			}

			conn, err := grpc.Dial("", opts...)
			if err != nil {
				panic(err)
			}
			defer conn.Close()

			client := internal.NewControlClient(conn)

			_, err = client.Crypto(context.Background(), &internal.Bool{Val: true})
			if err != nil {
				panic(err)
			}

			ipv6, err := client.IPv6(context.Background(), &internal.Null{})
			if err != nil {
				fmt.Println(err, errors.Is(err, os.ErrClosed))
			}
			fmt.Println("IPv6: ", ipv6.GetVal())

			for i := 0; i < 20; i++ {
				_, err = client.Crypto(context.Background(), &internal.Bool{Val: true})
				if err != nil {
					panic(err)
				}
				fmt.Println("xx", i)
				time.Sleep(time.Second)
			}
		}

		time.Sleep(time.Second * 3)
	})

}

func Test_Control(t *testing.T) {
	var saddr = &net.TCPAddr{Port: 8080}

	t.Run("t", func(t *testing.T) {

		go func() {
			l, err := net.ListenTCP("tcp", saddr)
			if err != nil {
				log.Fatalf("failed to listen: %v", err)
			}
			s := grpc.NewServer()
			internal.RegisterControlServer(s, &server1{})

			if err := s.Serve(l); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()

		time.Sleep(time.Second)
		{ // client
			conn, err := grpc.Dial(saddr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				panic(err)
			}
			defer conn.Close()

			client := internal.NewControlClient(conn)

			_, err = client.Crypto(context.Background(), &internal.Bool{Val: true})
			if err != nil {
				panic(err)
			}

			err = conn.Close()
			require.NoError(t, err)

			ipv6, err := client.IPv6(context.Background(), &internal.Null{})
			if err != nil {
				fmt.Println(err, errors.Is(err, os.ErrClosed))
			}
			fmt.Println("IPv6: ", ipv6.GetVal())
		}

		time.Sleep(time.Second * 3)
	})

}

func Test_Xxx(t *testing.T) {
	// l := newListenerWrap(nil)

	// conn, err := l.Accept()
	// t.Log(conn, err)

	// {
	// 	conn, err := l.Accept()
	// 	t.Log(conn, err)
	// }

	t.Run("b", func(t *testing.T) {

		var err = internal.Err{Err: "xxx"}

		fmt.Println(err.String())

		return

		{
			var b = &internal.Bool{Val: false}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Val)

		}

		{
			var b = &internal.Bool{Val: true}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Val)
		}

	})
}
