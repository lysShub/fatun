package internal

import "errors"

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative control.proto

func Ge(e *Err) error {
	if e == nil || e.Err == "" {
		return nil
	}
	return errors.New(e.Err)
}

func Eg(e error) *Err {
	if e == nil {
		return &Err{}
	}
	return &Err{Err: e.Error()}
}
