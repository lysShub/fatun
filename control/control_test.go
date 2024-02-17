package control

import (
	"fmt"
	"testing"

	"github.com/lysShub/itun/control/internal"
	"github.com/stretchr/testify/require"

	"google.golang.org/protobuf/proto"
)

func Test_Xxx(t *testing.T) {
	// l := newListenerWrap(nil)

	// conn, err := l.Accept()
	// t.Log(conn, err)

	// {
	// 	conn, err := l.Accept()
	// 	t.Log(conn, err)
	// }

	t.Run("b", func(t *testing.T) {

		var err = internal.Err{Err: "xxx"}

		fmt.Println(err.String())

		return

		{
			var b = &internal.Bool{Val: false}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Val)

		}

		{
			var b = &internal.Bool{Val: true}

			r, err := proto.Marshal(b)
			require.NoError(t, err)

			var recv = &internal.Bool{}
			err = proto.Unmarshal(r, recv)
			require.NoError(t, err)

			fmt.Println(r, recv.Val)
		}

	})
}
