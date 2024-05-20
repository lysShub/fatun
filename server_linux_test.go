//go:build linux
// +build linux

package fatun_test

import (
	"testing"
	"time"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/peer"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {

	s, err := fatun.NewServer[peer.Default]()
	require.NoError(t, err)

	err = s.Run()
	require.NoError(t, err)

	time.Sleep(time.Hour)
}
