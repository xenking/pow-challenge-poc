package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/xenking/pow-challenge-poc/internal/app"
	"github.com/xenking/pow-challenge-poc/internal/hashcash"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	serverAttr := os.Getenv("SERVER_ADDR")
	if serverAttr == "" {
		serverAttr = "localhost:8080"
	}

	err := getService(ctx, serverAttr)
	if err != nil {
		log.Panicln(err)
	}
}

func getService(ctx context.Context, addr string) error {
	challenge, err := getChallenge(ctx, addr)
	if err != nil {
		return err
	}

	log.Println("received challenge:", challenge)

	hc, err := hashcash.New(challenge)
	if err != nil {
		return err
	}

	hash, err := hc.Compute()
	if err != nil {
		return err
	}

	log.Println("computed hash:", hash)

	quote, err := getQuote(ctx, addr, hash)
	if err != nil {
		return err
	}

	log.Println("received quote:", quote)

	return nil
}

func getChallenge(ctx context.Context, addr string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	payload := []byte{byte(app.RequestTypeChallenge), '\n'}
	_, err = conn.Write(payload)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(conn)
	challenge, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read challenge: %w", err)
	}

	// Remove trailing newline
	challenge = challenge[:len(challenge)-1]

	return challenge, nil
}

func getQuote(ctx context.Context, addr, hash string) (*app.Quote, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	payload := []byte{byte(app.RequestTypeService)}
	payload = append(payload, hash...)
	payload = append(payload, '\n')

	_, err = conn.Write(payload)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	raw, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read quote: %w", err)
	}

	var quote *app.Quote
	err = json.Unmarshal(raw, &quote)
	if err != nil {
		return nil, fmt.Errorf("unmarshal quote: %w", err)
	}

	return quote, nil
}
