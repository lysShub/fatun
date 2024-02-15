package segment

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/require"
)

type A struct {
	A []byte
}

func Test_Xxxx(t *testing.T) {

	t.Run("a", func(t *testing.T) {

		var a = A{
			A: make([]byte, 512),
		}

		var buf = &bytes.Buffer{}
		enc := gob.NewEncoder(buf)

		require.NoError(t, enc.Encode(a))

		t.Log(len(buf.Bytes()))
	})

}
