package fatun_test

import (
	"testing"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/peer"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {

	s, err := fatun.NewServer[peer.Default]()
	require.NoError(t, err)

	err = s.Serve()
	require.NoError(t, err)
}
