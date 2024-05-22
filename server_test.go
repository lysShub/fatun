package fatun_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/peer"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {

	s, err := fatun.NewServer[peer.Default]()
	require.NoError(t, err)

	go func() {
		for {
			time.Sleep(time.Second * 30)
			ls := s.Links.Cleanup()
			for _, e := range ls {
				fmt.Println("close link", e.String())
			}
		}
	}()

	err = s.Serve()
	require.NoError(t, err)
}
