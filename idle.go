package itun

import (
	"sync/atomic"
	"time"
)

type Idle struct {
	cnt    uint8
	curCnt uint8

	rec atomic.Uint32
}

func NewIdle(dur time.Duration) (idle *Idle, ticker *time.Ticker) {
	const (
		min = 3
		max = 64
	)

	period := dur / min
	var cnt int
	if period > time.Second*5 {
		period = time.Second * 5
		cnt = int(dur/period) + 1
		if cnt > max {
			cnt = max
			period = dur / time.Duration(cnt)
		}
	}
	idle = &Idle{
		cnt: uint8(cnt),
	}
	ticker = time.NewTicker(period)
	return idle, ticker
}

func (i *Idle) Clone() *Idle {
	return &Idle{
		cnt: i.cnt,
	}
}

func (i *Idle) Idled() bool {
	if i.rec.Load() == 0 {
		i.curCnt++
	} else {
		i.rec.Store(0)
		i.curCnt = 0
	}
	return i.curCnt > i.cnt
}

func (i *Idle) Action() {
	i.rec.Add(1)
}
