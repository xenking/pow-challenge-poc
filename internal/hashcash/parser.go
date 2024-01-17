package hashcash

import (
	"strings"
	"time"
)

type Parser interface {
	Parse(header string) (nonce, hash string, err error)
}

// parser for hashcash tokens
type parser struct {
	// bits number of "partial pre-image" (zero) bits in the hashed code.
	bits int
	// expired expiry time for headers
	expired time.Time
	// future tolerance for clock skew
	future time.Time
}

func NewParser(cfg *ParserConfig) Parser {
	return &parser{
		bits:    cfg.Bits,
		future:  cfg.Future,
		expired: cfg.Expired,
	}
}

// Parse parses header and returns input nonce and hash string. If the header is not in a valid
// format, ErrInvalidHeader error is returned.
func (p *parser) Parse(header string) (nonce, hash string, err error) {
	fields := strings.Split(header, ":")
	if len(fields) != hashcashV1Length {
		return nonce, hash, ErrInvalidHeader
	}
	// fields: [version bits date nonce extension random counter]

	hash = sha1Hash(header)
	nonce = fields[3]

	wantZeros := p.bits / bitsPerHexChar

	// test 1 - zero count
	if !acceptableHeader(hash, zero, wantZeros) {
		return nonce, hash, ErrNoCollision
	}
	// test 2 - check token is not too far in the future or expired
	created, err := p.parseHashcashTime(fields[2])
	if err != nil {
		return nonce, hash, err
	}
	if created.After(p.future) || created.Before(p.expired) {
		return nonce, hash, ErrExpiredTimestamp
	}

	return nonce, hash, nil
}

// parseHashcashTime parses datetime in hashcash format
func (p *parser) parseHashcashTime(msgTime string) (date time.Time, err error) {
	// In a hashcash header the date parts year, month and day are mandatory but
	// hours, minutes and seconds are optional. So a valid date can be in format:
	//
	// - YYMMDD
	// - YYMMDDhhmm
	// - YYMMDDhhmmss
	//
	// Here we try find the format of the time, so it can be parsed.
	switch len(msgTime) {
	case 6:
		f := timeFormat[:6]
		date, err = time.Parse(f, msgTime)
	case 10:
		f := timeFormat[:10]
		date, err = time.Parse(f, msgTime)
	case 12:
		f := timeFormat[:12]
		date, err = time.Parse(f, msgTime)
	}
	return date, err
}
