package itun

import (
	"math"
	"sync/atomic"
	"time"
)

type Keepalive struct {
	cnt    uint16
	curCnt uint16

	rec atomic.Uint32
}

func NewKeepalive(idle time.Duration) (ka *Keepalive, ticker *time.Ticker) {
	const (
		maxDur = time.Second * 5
		minCnt = 5
	)

	var period time.Duration
	var cnt int
	if idle > maxDur*minCnt {
		period = maxDur
		cnt = int(math.Round(float64(idle) / float64(maxDur)))
	} else {
		period = idle / minCnt
		cnt = minCnt
	}

	if cnt > 0xffff {
		cnt = 0xffff
		period = idle / 0xffff
	}

	return &Keepalive{
		cnt: uint16(cnt),
	}, time.NewTicker(period)
}

func (k *Keepalive) Idled() bool {
	if k.rec.Load() == 0 {
		k.curCnt++
	} else {
		k.rec.Store(0)
		k.curCnt = 0
	}
	return k.curCnt > k.cnt
}

func (k *Keepalive) Action() {
	k.rec.Add(1)
}
