package hashcash

import (
	"time"
)

// options for a hashcash instance
type options struct {
	// Bits recommended default collision sizes are 20-bits
	bits int
}

const (
	// DefaultBits recommended default collision sizes are 20-bits
	DefaultBits = 20
)

type Option func(*options)

func WithBits(bits int) Option {
	return func(o *options) {
		o.bits = bits
	}
}

type ParserConfig struct {
	// Bits recommended default collision sizes are 20-bits
	Bits int
	// Expiry time before hashcash tokens are considered expired. Recommended
	// expiry time is 28 days
	Expired time.Time
	// Future hashcash in the future that should be rejected. Recommended
	// tolerance for clock skew is 48 hours
	Future time.Time
}

var DefaultParserConfig = &ParserConfig{
	Bits:    20,
	Future:  time.Now().AddDate(0, 0, 2),
	Expired: time.Now().AddDate(0, 0, -30),
}
