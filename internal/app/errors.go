package app

import "errors"

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrDuplicateHash  = errors.New("duplicate hash")
	ErrNonceNotFound  = errors.New("nonce not found")
)
