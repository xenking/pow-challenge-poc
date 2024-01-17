package hashcash

import "errors"

var (
	// ErrSolutionFail error cannot compute a solution
	ErrSolutionFail = errors.New("exceeded iterations failed to find solution")

	// ErrNonceEmpty error empty hashcash nonce
	ErrNonceEmpty = errors.New("empty hashcash nonce")

	// ErrInvalidHeader error invalid hashcash header format
	ErrInvalidHeader = errors.New("invalid hashcash header format")

	// ErrNoCollision error n 5 most significant hex digits (n most significant
	// bits are not 0.
	ErrNoCollision = errors.New("no collision most significant bits are not zero")

	// ErrExpiredTimestamp error futuristic and expired timestamps are rejected
	ErrExpiredTimestamp = errors.New("expired timestamp")

	// ErrInvalidTimestamp error invalid timestamp
	ErrInvalidTimestamp = errors.New("invalid timestamp")
)
