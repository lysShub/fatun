package config

import (
	"github.com/lysShub/itun/sconn"
)

type Common struct {
	Sconn sconn.Config

	MTU uint16
}
