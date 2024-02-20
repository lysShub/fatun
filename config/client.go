package config

type Client struct {
	Common

	Capture Capture
}

type Capture interface {
	Capture(ip []byte) (int, error)
	Inject(ip []byte) error
}
