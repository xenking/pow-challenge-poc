package storage

import "context"

type Hashes interface {
	StoreHash(ctx context.Context, hash string) error
	IsExistsHash(ctx context.Context, hash string) (bool, error)
}

type Nonce interface {
	StoreNonce(ctx context.Context, nonce string) error
	IsExistsNonce(ctx context.Context, nonce string) (bool, error)
}

// Storage store and retrieve hashcash entries
type Storage interface {
	Hashes
	Nonce
}
