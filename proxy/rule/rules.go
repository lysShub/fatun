package rule

import (
	"errors"
	"strings"
	"sync"

	"github.com/lysShub/go-divert"
)

type Rule interface {
	Close() error
	// TODO:
}

type rules struct {
	baseRule string
	ruleMap  map[string]Rule

	ch chan string

	m *sync.RWMutex
}

func NewRules(baseRule string) *rules {
	if baseRule == "" {
		baseRule = "!loopback"
	}

	return &rules{
		baseRule: baseRule,
		ruleMap:  map[string]Rule{},
		ch:       make(chan string, 16),
		m:        &sync.RWMutex{},
	}
}

func (r *rules) Proxyer() <-chan string {
	return r.ch
}

func (r *rules) AddRule(rule string) (err error) {
	if rule, err = r.formatRule(rule); err != nil {
		return err
	}

	r.m.RLock()
	if _, has := r.ruleMap[rule]; has {
		r.m.RUnlock()
		return nil
	}
	r.m.RUnlock()

	r.m.Lock()
	defer r.m.Unlock()
	if rule == BuiltinRule {
		rr, err := newBuiltinRule(r.baseRule, r.ch)
		if err != nil {
			return err
		}
		r.ruleMap[rule] = rr
	} else {
		rr, err := newRule(r.baseRule, rule, r.ch)
		if err != nil {
			return err
		}
		r.ruleMap[rule] = rr
	}
	return nil
}

// AddBuiltinRule add the builtin rule
//
//	builtin rule:  monitor tcp conn, will be proxy when resend SYN packet
func (r *rules) AddBuiltinRule() error { return r.AddRule(BuiltinRule) }

func (r *rules) DelRule(rule string) error {
	var err error
	if rule, err = r.formatRule(rule); err != nil {
		return err
	}

	r.m.Lock()
	rr, has := r.ruleMap[rule]
	r.m.Unlock()
	if !has {
		return errors.New("rule not found")
	} else {
		if err := rr.Close(); err != nil {
			return err
		}
		delete(r.ruleMap, rule)
	}
	return nil
}

func (r *rules) List() []string {
	var rules []string
	r.m.RLock()
	for f := range r.ruleMap {
		rules = append(rules, f)
	}
	r.m.Unlock()
	return rules
}

func (r *rules) formatRule(rule string) (string, error) {
	if rule == "" || strings.ToLower(rule) == BuiltinRule {
		return BuiltinRule, nil
	} else {
		r, err := divert.WinDivertHelperFormatFilter(rule, divert.LAYER_FLOW)
		if err != nil {
			return "", err
		}
		return r, nil
	}
}
