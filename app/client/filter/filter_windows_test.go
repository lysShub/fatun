package filter

import (
	"fmt"
	"testing"

	"github.com/lysShub/divert-go"
)

func TestClient(t *testing.T) {
	divert.MustLoad(divert.DLL, divert.Sys)
	defer divert.Release()

	var s = "udp and !ipv6 and event=CONNECT"
	d, err := divert.Open(s, divert.SOCKET, 0, divert.READ_ONLY|divert.SNIFF)
	if err != nil {
		panic(err)
	}

	var addr divert.Address

	for {

		_, err := d.Recv(nil, &addr)
		if err != nil {
			panic(err)
		}

		s := addr.Socket()

		_, op := addr.Event.String()
		fmt.Printf("%d %s %s --> %s \n", s.ProcessId, op, s.LocalAddr(), s.RemoteAddr())
	}

}
