package filter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
)

func Test_Filter(t *testing.T) {
	divert.Load(divert.DLL, divert.Sys)
	defer divert.Release()

	f, _ := NewFilter(cctx.WithContext(context.Background()))

	f.AddRule("chrome.exe", itun.TCP)

	time.Sleep(time.Hour)

	// err := f.AddRule("curl.exe", itun.TCP)
	// require.NoError(t, err)

	// ch := f.ProxyCh()

	// for {
	// 	s := <-ch
	// 	fmt.Println(s.String())
	// }
}

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
