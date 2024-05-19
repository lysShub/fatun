package fatun_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/peer"
)

func TestXxx(t *testing.T) {

	c, err := fatun.NewClient[*peer.Default]()

	fmt.Println(c, err)
}
