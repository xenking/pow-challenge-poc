package hashcash_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xenking/pow-challenge-poc/internal/hashcash"
)

var (
	validToken   = createValidTestToken()
	invalidToken = "blah"
	expiredToken = "1:20:040806:foo::65f460d0726f420d:13a6b8"
)

func createValidTestToken() string {
	hc, err := hashcash.New("12345")
	if err != nil {
		return ""
	}
	var gotProof bool
	var solution string
	for !gotProof {
		s, err := hc.Compute()
		if err == nil {
			solution = s
			gotProof = true
		}
	}

	return solution
}

func TestComputeHashcash(t *testing.T) {
	hc, err := hashcash.New("12345")
	if err != nil {
		t.Error(err)
	}
	var (
		gotProof bool
		solution string
	)

	for !gotProof {
		s, err := hc.Compute()
		if err != nil {
			if !errors.Is(err, hashcash.ErrSolutionFail) {
				t.Error(err)
			}
		} else {
			solution = s
			gotProof = true
		}
	}
	if !strings.HasPrefix(solution, "1:20:") {
		t.Error("bad/invalid hashcash token")
	}
	if !strings.Contains(solution, "12345") {
		t.Error("bad/invalid hashcash token")
	}
}

func TestVerifyHashcash(t *testing.T) {
	p := hashcash.NewParser(hashcash.DefaultParserConfig)
	nonce, hash, err := p.Parse(validToken)
	if err != nil {
		t.Error(err)
	}
	if nonce == "" || hash == "" {
		t.Error("hashcash token failed verification")
	}
}

func TestHashcashInvalidHeader(t *testing.T) {
	p := hashcash.NewParser(hashcash.DefaultParserConfig)
	_, _, err := p.Parse(invalidToken)
	if !errors.Is(err, hashcash.ErrInvalidHeader) {
		t.Error(err)
	}
}

func TestHashcashInvalidTimestamp(t *testing.T) {
	p := hashcash.NewParser(hashcash.DefaultParserConfig)

	_, _, err := p.Parse(expiredToken)
	if !errors.Is(err, hashcash.ErrExpiredTimestamp) {
		t.Error(err)
	}
}
