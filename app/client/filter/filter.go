package filter

// Hitter validate the session is hit rule.
type Hitter interface {
	Hit(ip []byte) (bool, error)
}

type ErrNotRecord struct{}

func (ErrNotRecord) Error() string { return "filter not record" }

type Filter interface {
	Hitter

	EnableDefault() error
	DisableDefault() error

	AddProcess(process string) error
	DelProcess(process string) error
}

func New() Filter {
	return newFilter()
}
