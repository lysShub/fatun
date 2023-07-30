package rule

import (
	"strings"

	"github.com/lysShub/go-divert"
)

type Rule interface {
	Close() error
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
