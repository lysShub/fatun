package faketcp_test

import (
	"testing"

	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/relraw"
)

func Test_FakeTCP(t *testing.T) {

	var pseudoSum1 uint16 = 1111

	var f = faketcp.NewFakeTCP(
		19986, 8080, 1234, 5678, &pseudoSum1,
	)

	var data = make([]byte, 16)

	var p = relraw.ToPacket(0, data)

	f.SendAttach(p)

}
