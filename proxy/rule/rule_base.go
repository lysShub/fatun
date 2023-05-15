package rule

import (
	"fmt"
	"itun/proxy/priority"
	"sync/atomic"

	"github.com/lysShub/go-divert"
)

type baseRuler struct {
	listener divert.Handle
	ch       chan string
	done     atomic.Bool
}

func newRule(baseRule, rule string, ch chan string) (Rule, error) {
	var err error
	var p = &baseRuler{ch: ch}

	var f = baseRule + " and " + rule
	p.listener, err = divert.Open(f, divert.LAYER_FLOW, priority.DefaultRulePriority, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		return nil, err
	}

	go p.do()
	return p, nil
}

func (p *baseRuler) do() {
	var err error

	var addr divert.Address
	for !p.done.Load() {
		_, addr, err = p.listener.Recv(nil)
		if err != nil {
			panic(err)
		}

		a := addr.Flow()
		var f = fmt.Sprintf("outbound and %s and localAddr=%s and remoteAddr=%s and localPort=%d and remotePort=%d", a.Protocol, a.LocalAddr(), a.RemoteAddr(), a.LocalPort, a.RemotePort)

		select {
		case p.ch <- f:
		default:
			fmt.Println("rules channel is full")
		}
	}
}

func (p *baseRuler) Close() error {
	p.done.Store(true)
	p.listener.Shutdown(divert.SHUTDOWN_RECV)
	return p.listener.Close()
}
