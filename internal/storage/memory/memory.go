package memory

import (
	"context"

	"github.com/xenking/pow-challenge-poc/internal/storage"
)

func New() storage.Storage {
	return &memory{
		hashes: make(map[string]struct{}),
		nonce:  make(map[string]struct{}),
	}
}

type memory struct {
	hashes map[string]struct{}
	nonce  map[string]struct{}
}

func (m *memory) StoreHash(ctx context.Context, hash string) error {
	m.hashes[hash] = struct{}{}
	return nil
}

func (m *memory) IsExistsHash(ctx context.Context, hash string) (bool, error) {
	_, ok := m.hashes[hash]
	return ok, nil
}

func (m *memory) StoreNonce(ctx context.Context, nonce string) error {
	m.nonce[nonce] = struct{}{}
	return nil
}

func (m *memory) IsExistsNonce(ctx context.Context, nonce string) (bool, error) {
	_, ok := m.nonce[nonce]
	return ok, nil
}
