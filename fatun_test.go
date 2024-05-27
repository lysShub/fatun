package fatun_test

import (
	"context"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/crypto"
)

var cfg = &fatcp.Config{
	Handshake: &fatcp.Sign{
		Sign: []byte("0123456789abcdef"),
		Parser: func(ctx context.Context, sign []byte) (crypto.Key, error) {
			return crypto.Key{9: 1}, nil
		},
	},
}
