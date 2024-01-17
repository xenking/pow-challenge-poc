package hashcash

import (
	"fmt"
	"time"
)

const (
	maxIterations    int    = 1 << 30        // Max iterations to find a solution
	bytesToRead      int    = 8              // Bytes to read for random token
	bitsPerHexChar   int    = 4              // Each hex character takes 4 bits
	zero             rune   = 48             // ASCII code for number zero
	hashcashV1Length int    = 7              // Number of items in a V1 hashcash header
	timeFormat       string = "060102150405" // YYMMDDhhmmss
)

// HashCash instance
type HashCash struct {
	// version hashcash format version, 1 (which supersedes version 0).
	version int
	// bits number of "partial pre-image" (zero) bits in the hashed code.
	bits int
	// created date The time that the message was sent.
	created time.Time
	// nonce random string being transmitted.
	nonce string
	// extension (optional; ignored in version 1).
	extension string
	// rand characters, encoded in base-64 format.
	rand string
	// counter (up to 2^20), encoded in base-64 format.
	counter int
}

// New creates a new HashCash instance
func New(nonce string, opts ...Option) (*HashCash, error) {
	if nonce == "" {
		return nil, ErrNonceEmpty
	}

	opt := &options{
		bits: DefaultBits,
	}

	for _, o := range opts {
		o(opt)
	}

	rand, err := randomBytes(bytesToRead)
	if err != nil {
		return nil, err
	}

	return &HashCash{
		version:   1,
		bits:      opt.bits,
		created:   time.Now(),
		nonce:     nonce,
		extension: "",
		rand:      base64EncodeBytes(rand),
		counter:   1,
	}, nil
}

// Compute a new hashcash header. If no solution can be found 'ErrSolutionFail'
// error is returned.
func (h *HashCash) Compute() (string, error) {
	// hex char: 0    0    0    0    0
	// binary  : 0000 0000 0000 0000 0000 = 4 bits per char = 20 bits total
	var (
		wantZeros = h.bits / bitsPerHexChar
		header    = h.createHeader()
		hash      = sha1Hash(header)
	)
	for !acceptableHeader(hash, zero, wantZeros) {
		h.counter++
		header = h.createHeader()
		hash = sha1Hash(header)
		if h.counter >= maxIterations {
			return "", ErrSolutionFail
		}
	}
	return header, nil
}

func (h *HashCash) createHeader() string {
	return fmt.Sprintf("%d:%d:%s:%s:%s:%s:%s", h.version,
		h.bits,
		h.created.Format(timeFormat),
		h.nonce,
		h.extension,
		h.rand,
		base64EncodeInt(h.counter))
}

// acceptableHeader determines if the string 'hash' is prefixed with 'n',
// 'char' characters.
func acceptableHeader(hash string, char rune, n int) bool {
	for _, val := range hash[:n] {
		if val != char {
			return false
		}
	}
	return true
}
