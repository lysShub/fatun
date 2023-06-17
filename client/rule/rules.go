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

type rule string

func (r rule) Format() (string, error) {
	if r == "" || strings.ToLower(string(r)) == BuiltinRule {
		return BuiltinRule, nil
	} else {
		r, err := divert.HelperFormatFilter(string(r), divert.LAYER_SOCKET)
		if err != nil {
			return "", err
		}
		return r, nil
	}
}

func (r rule) IsBuiltin() bool {
	return strings.ToLower(string(r)) == BuiltinRule
}

type rules struct {
	ruleMap map[string]Rule

	ch chan string

	m *sync.RWMutex
}

func NewRules() *rules {
	return &rules{
		ruleMap: map[string]Rule{},
		ch:      make(chan string, 16),
		m:       &sync.RWMutex{},
	}
}

func (rs *rules) Proxyer() <-chan string {
	return rs.ch
}

func (rs *rules) AddRule(r string) (err error) {
	if r, err = rule(r).Format(); err != nil {
		return err
	}

	rs.m.RLock()
	if _, has := rs.ruleMap[r]; has {
		rs.m.RUnlock()
		return nil
	}
	rs.m.RUnlock()

	rs.m.Lock()
	defer rs.m.Unlock()
	if r == BuiltinRule {
		rr, err := newBuiltinRule(rs.ch)
		if err != nil {
			return err
		}
		rs.ruleMap[r] = rr
	} else {
		rr, err := newRule(r, rs.ch)
		if err != nil {
			return err
		}
		rs.ruleMap[r] = rr
	}
	return nil
}

// AddBuiltinRule add the builtin rule
//
//	builtin rule:  monitor tcp conn, will be proxy when resend SYN packet
func (rs *rules) AddBuiltinRule() error { return rs.AddRule(BuiltinRule) }

func (rs *rules) DelRule(rule1 string) error {
	var err error
	if rule1, err = rule(rule1).Format(); err != nil {
		return err
	}

	rs.m.Lock()
	rr, has := rs.ruleMap[rule1]
	rs.m.Unlock()
	if !has {
		return errors.New("rule not found")
	} else {
		if err := rr.Close(); err != nil {
			return err
		}
		delete(rs.ruleMap, rule1)
	}
	return nil
}

func (rs *rules) List() []string {
	var rules []string
	rs.m.RLock()
	for f := range rs.ruleMap {
		rules = append(rules, f)
	}
	rs.m.Unlock()
	return rules
}
