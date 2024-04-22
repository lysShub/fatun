//go:build windows
// +build windows

package capture

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TLSCapByGoRequest(t *testing.T) {

	url := "https://dl.google.com/go/go1.20.4.linux-amd64.tar.gz"

	pps, err := CaptureTLSWithGolang(context.Background(), url, 1024*64, -(512))
	require.NoError(t, err)

	for _, e := range pps {
		fmt.Println(len(e))
	}
}
