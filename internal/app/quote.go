package app

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"os"
)

type QuoteStorage struct {
	quotes []Quote
}

func NewQuoteStorage(path string) (*QuoteStorage, error) {
	var quotes []Quote

	err := loadQuotesFile(path, &quotes)
	if err != nil {
		return nil, err
	}

	return &QuoteStorage{
		quotes: quotes,
	}, err
}

func (s *QuoteStorage) GetRandomQuote() Quote {
	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(s.quotes))))
	return s.quotes[idx.Int64()]
}

func loadQuotesFile(path string, quotes *[]Quote) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&quotes)
}

type Quote struct {
	Quote  string `json:"quote"`
	Author string `json:"author"`
}

func (q Quote) String() string {
	data, _ := json.Marshal(q)
	return string(data)
}
