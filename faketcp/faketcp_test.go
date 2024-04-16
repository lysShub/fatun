package faketcp_test

import (
	"testing"

	"github.com/lysShub/itun/faketcp"
	"github.com/lysShub/sockit/packet"

	"github.com/lysShub/sockit/test"
	"github.com/stretchr/testify/require"
)

func Test_FakeTCP(t *testing.T) {
	var pseudoSum1 uint16 = 1111

	var f = faketcp.New(
		19986, 8080, nil,
		faketcp.PseudoSum1(pseudoSum1),
	)

	var p = packet.Make(0, 16)

	f.AttachSend(p)
	require.Equal(t, 16+20, p.Data())
	test.ValidTCP(t, p.Bytes(), pseudoSum1)
	require.True(t, faketcp.Is(p.Bytes()))

	f.DetachRecv(p)
	require.False(t, faketcp.Is(p.Bytes()))
	require.Equal(t, 16, p.Data())
}
