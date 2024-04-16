package itun

import (
	"math"
	"sync/atomic"
	"time"
)

var KeepaliveExceeded = keepaliveExceededError{}

type keepaliveExceededError struct{}

func (keepaliveExceededError) Error() string   { return "keepalive exceeded" }
func (keepaliveExceededError) Timeout() bool   { return true }
func (keepaliveExceededError) Temporary() bool { return true }

type Keepalive struct {
	count  uint16
	ticker *time.Ticker
}

func NewKeepalive(idle time.Duration) (ka *Keepalive) {
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
		count:  uint16(cnt),
		ticker: time.NewTicker(period),
	}
}

func (k *Keepalive) Ticker() <-chan time.Time {
	return k.ticker.C
}

func (k *Keepalive) Task() *Task {
	return &Task{}
}

type Task struct {
	countLimit   uint16
	currentCount uint16

	rec atomic.Uint32
}

func (t *Task) Idle() bool {
	if t.rec.Load() == 0 {
		t.currentCount++
	} else {
		t.rec.Store(0)
		t.currentCount = 0
	}
	return t.currentCount > t.countLimit
}

func (t *Task) Action() {
	t.rec.Add(1)
}
