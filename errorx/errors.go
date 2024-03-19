package errorx

import "strings"

type joinErr struct {
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
	return &joinErr{
		parent: parent,
		err:    err,
	}
}

func (e *joinErr) Error() string {
	var s strings.Builder

	s.WriteString(e.parent.Error())
	s.WriteRune(':')
	s.WriteString(e.err.Error())

	return s.String()
}

func (e *joinErr) Unwrap() error {
	return &unwrapedLinkErr{parent: e.parent, error: e.err}
}

type unwrapedLinkErr struct {
	parent error
	error
}

func (e *unwrapedLinkErr) Unwrap() error {
	return e.parent
}

func IsTemporary(err error) bool {
	_, ok := err.(interface{ Temporary() bool })
	return ok
}

type temporaryErr struct {
	err error
}

func Temporary(err error) error {
	return &temporaryErr{err: err}
}
func (t *temporaryErr) Error() string   { return t.err.Error() }
func (t *temporaryErr) Unwrap() error   { return t.err }
func (t *temporaryErr) Temporary() bool { return true }
