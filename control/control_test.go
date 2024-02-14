package control

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/lysShub/itun/control/internal"
	"github.com/stretchr/testify/require"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

func init() {
	grpclog.SetLoggerV2(NewZapLogger())
	grpc.EnableTracing = false
}

type server struct {
	internal.UnimplementedControlServer
}

func (s *server) Crypto(ctx context.Context, crypto *internal.Bool) (*internal.Null, error) {
	fmt.Println("Crypto: ", crypto.Value)
	return &internal.Null{}, nil
}

func (s *server) IPv6(ctx context.Context, ipv6 *internal.Null) (*internal.Bool, error) {
	fmt.Println("ipv6")
	return &internal.Bool{Value: false}, nil
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
			internal.RegisterControlServer(s, &server{})

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

			_, err = client.Crypto(context.Background(), &internal.Bool{Value: true})
			if err != nil {
				panic(err)
			}

			ipv6, err := client.IPv6(context.Background(), &internal.Null{})
			if err != nil {
				panic(err)
			}
			fmt.Println("IPv6: ", ipv6.GetValue())

		}
	})

	t.Run("b", func(t *testing.T) {

		{
			var b = &internal.Bool{Value: false}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Value)

		}

		{
			var b = &internal.Bool{Value: true}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Value)
		}

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
}
