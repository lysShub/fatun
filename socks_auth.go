package itun

import "net"

type Auth byte

func (a Auth) Auth(conn net.Conn) error {
	if f, ok := methodAuth[byte(a)]; ok {
		return f(conn)
	}

	return nil
}

const (
	NoAuth             Auth = 0x00
	GSSAPI             Auth = 0x01
	UsrPwd             Auth = 0x02
	ChallengeHandshake Auth = 0x03
	_                  Auth = 0x04
	ChallengeResponse  Auth = 0x05
	TLS                Auth = 0x06
	NDS                Auth = 0x07
	MultiAuth          Auth = 0x08
	JSONBlock          Auth = 0x09
	NULL               Auth = 0xff
)

type Auths [256]Auth

func (a Auths) Auth(conn net.Conn) error {
	for _, auth := range a {
		if err := auth.Auth(conn); err != nil {
			return err
		} else if auth == NoAuth {
			return nil
		}
	}
	return nil
}

func (a Auths) Adds(auth Auth, fn func(net.Conn) error) {
	for i := 0; i < 256; i++ {
		if a[i] == NoAuth || a[i] == auth {
			a[i] = auth
			methodAuth[byte(auth)] = fn
			return
		}
	}
}

func (a Auths) Include(auths ...byte) Auth {
	for _, auth := range AcceptAuths {
		for _, a := range auths {
			if Auth(a) == auth {
				return auth
			}
		}
	}
	return NULL
}

var AcceptAuths Auths

func init() {
	AcceptAuths.Adds(NoAuth, func(conn net.Conn) error { return nil })
}

var methodAuth = map[byte]func(conn net.Conn) error{}
