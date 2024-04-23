package sconn_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/lysShub/fatun/sconn"
	"github.com/stretchr/testify/require"
)

func Test_PrevPackets(t *testing.T) {
	t.Run("1", func(t *testing.T) {
		var pps = sconn.PrevPackets{
			[]byte("hello"),
			[]byte("world"),
		}
		defer func() { os.Remove("a.pps") }()

		err := pps.Marshal("a.pps")
		require.NoError(t, err)

		var pps2 sconn.PrevPackets
		err = pps2.Unmarshal("a.pps")
		require.NoError(t, err)

		for i, e := range pps2 {
			require.Equal(t, pps[i], e)
		}
	})

	t.Run("2", func(t *testing.T) {
		var pps = sconn.PrevPackets{
			make([]byte, 1460),
			make([]byte, 345),
			make([]byte, 1460),
		}
		var r = rand.New(rand.NewSource(0))
		for i := range pps {
			r.Read(pps[i])
		}
		defer func() { os.Remove("a.pps") }()

		err := pps.Marshal("a.pps")
		require.NoError(t, err)

		var pps2 sconn.PrevPackets
		err = pps2.Unmarshal("a.pps")
		require.NoError(t, err)

		for i, e := range pps2 {
			require.Equal(t, pps[i], e)
		}
	})
}
