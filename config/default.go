package config

import "time"

var _ = Config{
	PrevPackets:      [][]byte{{1, 23}},
	HandShakeTimeout: time.Second,
}
