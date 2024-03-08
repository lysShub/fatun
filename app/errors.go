package app

import (
	"strings"
)

type linkErr struct {
	parent error
	err    error
}

func Join(parent error, err error) error {
	if parent == nil {
		return err
	}
	if err == nil {
		return parent
	}
	return &linkErr{
		parent: parent,
		err:    err,
	}
}

func (e *linkErr) Error() string {
	var s strings.Builder

	s.WriteString(e.parent.Error())
	s.WriteRune(':')
	s.WriteString(e.err.Error())

	return s.String()
}

func (e *linkErr) Unwrap() error {
	return &unwrapedLinkErr{parent: e.parent, error: e.err}
}

type unwrapedLinkErr struct {
	parent error
	error
}

func (e *unwrapedLinkErr) Unwrap() error {
	return e.parent
}
