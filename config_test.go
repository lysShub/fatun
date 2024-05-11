package fatun_test

import (
	"math/rand"
	"os"
	"testing"

	"github.com/lysShub/fatun"
	"github.com/stretchr/testify/require"
)

func Test_PrevPackets(t *testing.T) {
	t.Run("1", func(t *testing.T) {
		var pss = fatun.PrevSegmets{
			[]byte("hello"),
			[]byte("world"),
		}
		defer func() { os.Remove("a.pss") }()

		err := pss.Marshal("a.pss")
		require.NoError(t, err)

		var pss2 fatun.PrevSegmets
		err = pss2.Unmarshal("a.pss")
		require.NoError(t, err)

		for i, e := range pss2 {
			require.Equal(t, pss[i], e)
		}
	})

	t.Run("2", func(t *testing.T) {
		var pss = fatun.PrevSegmets{
			make([]byte, 1460),
			make([]byte, 345),
			make([]byte, 1460),
		}
		var r = rand.New(rand.NewSource(0))
		for i := range pss {
			r.Read(pss[i])
		}
		defer func() { os.Remove("a.pss") }()

		err := pss.Marshal("a.pss")
		require.NoError(t, err)

		var pss2 fatun.PrevSegmets
		err = pss2.Unmarshal("a.pss")
		require.NoError(t, err)

		for i, e := range pss2 {
			require.Equal(t, pss[i], e)
		}
	})
}
