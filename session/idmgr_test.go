package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Id_Manager(t *testing.T) {
	t.Run("alloc-init-consecutive", func(t *testing.T) {
		var idmgr = &IdMgr{}

		var id ID
		for i := 0; i < 1024; i++ {
			nid, err := idmgr.Get()
			require.NoError(t, err)
			if i != 0 {
				require.Equal(t, id+1, nid)
			}
			id = nid
		}
	})

	t.Run("alloc-init-puted", func(t *testing.T) {
		var idmgr = &IdMgr{}

		for i := 0; i < 8; i++ {
			id, err := idmgr.Get()
			require.NoError(t, err)
			require.Zero(t, id)

			idmgr.Put(id)
		}
	})

	t.Run("not-exceed", func(t *testing.T) {
		var idmgr = &IdMgr{}

		for i := 0; i < 0xffff-1; i++ {
			id, err := idmgr.Get()
			require.NoError(t, err)
			require.NotEqual(t, CtrSessID, id)
		}

		id, err := idmgr.Get()
		require.Zero(t, id)
		require.Equal(t, ErrSessionExceed, err)
	})
}
