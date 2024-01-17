package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/xenking/pow-challenge-poc/internal/hashcash"
	"github.com/xenking/pow-challenge-poc/internal/server"
	"github.com/xenking/pow-challenge-poc/internal/storage"
)

type RequestType int8

const (
	RequestTypeUnknown RequestType = iota
	RequestTypeChallenge
	RequestTypeService
)

func New(storage storage.Storage, quotesPath string) (*App, error) {
	quoteStorage, err := NewQuoteStorage(quotesPath)
	if err != nil {
		return nil, err
	}

	return &App{
		challengeStorage: storage,
		parser:           hashcash.NewParser(hashcash.DefaultParserConfig),
		quotes:           quoteStorage,
	}, nil
}

type App struct {
	challengeStorage storage.Storage
	parser           hashcash.Parser
	quotes           *QuoteStorage
}

func (a *App) Handler(reqCtx *server.RequestContext) {
	ctx := reqCtx.Context()

	log.Println("received request:", reqCtx.String())

	body := reqCtx.Body()
	req, err := parseRequestBody(body)
	if err != nil {
		writeError(reqCtx, err)
	}

	switch req.Type {
	case RequestTypeChallenge:
		err = a.handleChallenge(ctx, reqCtx)
	case RequestTypeService:
		err = a.handleService(ctx, reqCtx, req)
	default:
		err = ErrInvalidRequest
	}

	if err != nil {
		writeError(reqCtx, err)
	}
}

func (a *App) handleChallenge(ctx context.Context, reqCtx *server.RequestContext) error {
	nonce, err := newNonce(32)
	if err != nil {
		return err
	}

	err = a.challengeStorage.StoreNonce(ctx, nonce)
	if err != nil {
		return err
	}
	log.Println("send challenge:", nonce)

	reqCtx.WriteResponseString(nonce)
	return nil
}

func (a *App) handleService(ctx context.Context, reqCtx *server.RequestContext, req *Request) error {
	err := a.validateChallenge(ctx, string(req.Payload))
	if err != nil {
		return err
	}

	quote := a.quotes.GetRandomQuote()

	log.Println("send quote:", quote.String())

	reqCtx.WriteResponseString(quote.String())

	return nil
}

func (a *App) validateChallenge(ctx context.Context, payload string) error {
	log.Println("validate challenge:", payload)

	parsedNonce, hash, err := a.parser.Parse(payload)
	if err != nil {
		return err
	}

	ok, err := a.challengeStorage.IsExistsHash(ctx, hash)
	if err != nil {
		return err
	}
	if ok {
		return ErrDuplicateHash
	}

	err = a.challengeStorage.StoreHash(ctx, hash)
	if err != nil {
		return err
	}

	ok, err = a.challengeStorage.IsExistsNonce(ctx, parsedNonce)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNonceNotFound
	}

	log.Println("challenge validated:", parsedNonce)

	return nil
}

type Request struct {
	Type    RequestType
	Payload []byte
}

func parseRequestBody(body []byte) (*Request, error) {
	if len(body) == 0 {
		return nil, ErrInvalidRequest
	}

	req := &Request{
		Type:    RequestType(body[0]),
		Payload: body[1:],
	}

	return req, nil
}

func writeError(ctx *server.RequestContext, err error) {
	ctx.Error(fmt.Sprintf("Server error: %v", err))
}

func newNonce(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return fmt.Sprintf("%x", b), err
}
