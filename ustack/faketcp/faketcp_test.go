package faketcp_test

import (
	"testing"

	"github.com/lysShub/itun/ustack/faketcp"
	"github.com/lysShub/relraw"
	"github.com/lysShub/relraw/test"
	"github.com/stretchr/testify/require"
)

func Test_FakeTCP(t *testing.T) {
	var pseudoSum1 uint16 = 1111

	var f = faketcp.NewFakeTCP(
		19986, 8080, 1234, 5678, &pseudoSum1,
	)

	var data = make([]byte, 16)

	var p = relraw.ToPacket(0, data)

	f.SendAttach(p)
	require.Equal(t, 16+20, p.Len())
	test.ValidTCP(t, p.Data(), pseudoSum1)
	require.True(t, faketcp.IsFakeTCP(p.Data()))

	f.RecvStrip(p)
	require.False(t, faketcp.IsFakeTCP(p.Data()))
	require.Equal(t, 16, p.Len())
}
