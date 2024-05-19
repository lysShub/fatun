package fatun

type ErrRecvTooManyError struct{}

func (e ErrRecvTooManyError) Error() string {
	return "recv too many invalid packet"
}

type ErrKeepaliveExceeded struct{}

func (ErrKeepaliveExceeded) Error() string   { return "keepalive exceeded" }
func (ErrKeepaliveExceeded) Timeout() bool   { return true }
func (ErrKeepaliveExceeded) Temporary() bool { return true }

type ErrNotRecord struct{}

func (ErrNotRecord) Error() string   { return "not record" }
func (ErrNotRecord) Temporary() bool { return true }
