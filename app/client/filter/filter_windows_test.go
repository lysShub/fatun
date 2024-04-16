package filter

import (
	"fmt"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
)

func Test_Filter(t *testing.T) {
	divert.Load(divert.DLL)
	defer divert.Release()

	f, _ := NewFilter()

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
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	var s = "udp and !ipv6 and event=CONNECT"
	d, err := divert.Open(s, divert.Socket, 0, divert.ReadOnly|divert.Sniff)
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

		fmt.Printf("%d %s %s --> %s \n", s.ProcessId, addr.Event.Op(), s.LocalAddr(), s.RemoteAddr())
	}

}
