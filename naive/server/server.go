package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"time"

	"golang.org/x/time/rate"
)

const (
	// Difficulty for the proof of work. The client needs to find a nonce such that
	// the SHA256 hash has at least this number of leading zero bits.
	// You need to adjust this to your needs. The higher the number, the harder the PoW.
	difficulty = 16
	// serverAddr is the port at which the server will be listening
	serverAddr        = ":8080"
	maxConn           = 100 // Maximum number of concurrent connections
	rateLimit         = 1   // Allow 1 operation per second
	burstLimit        = 5   // Allow bursts of up to 5 operations
	connectionTimeout = 10 * time.Second
)

// quotes is a hardcoded slice of wisdom quotes.
var quotes = []string{
	"Knowing yourself is the beginning of all wisdom. - Aristotle",
	"The only true wisdom is in knowing you know nothing. - Socrates",
	"Wisdom is not a product of schooling but of the lifelong attempt to acquire it. - Albert Einstein",
}

var (
	limiter = rate.NewLimiter(rate.Limit(rateLimit), burstLimit)
	sem     = make(chan struct{}, maxConn)
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := serveServer(ctx, serverAddr)
	if err != nil {
		log.Panicln(err)
	}
}

func serveServer(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("error listening: %w", err)
	}
	defer ln.Close()
	fmt.Println("Listening on", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %s", err)
			continue
		}

		if limiter.Allow() {
			sem <- struct{}{} // Acquire a token from the semaphore
			go handleConnection(ctx, conn)
		} else {
			fmt.Fprint(conn, "Rate limit exceeded.")
			conn.Close()
		}
	}
}

func handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		<-sem // Release the token when done
	}()

	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()

	fmt.Println("Connection established from:", conn.RemoteAddr().String())

	// Close connection after function return
	defer conn.Close()

	// Set deadline for connection
	conn.SetDeadline(time.Now().Add(connectionTimeout))

	challenge := generateChallenge()
	fmt.Fprintf(conn, "Challenge: %x\n", challenge)
	fmt.Fprintf(conn, "Difficulty: %d\n", difficulty)

	// Read response with a scanner to avoid blocking indefinitely
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		response := scanner.Text() // Read the client's response

		if isValidProof(challenge, response) {
			// Send a quote
			_, err := fmt.Fprintln(conn, getRandomQuote())
			if err != nil {
				fmt.Printf("Error sending quote: %v", err)
			}
			return
		} else {
			fmt.Fprintln(conn, "Invalid proof of work. Please try again.")
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading from connection: %v", err)
	}
}

// generateChallenge provides a cryptographic random challenge.
func generateChallenge() []byte {
	b := make([]byte, 32) // 256-bit challenge
	_, err := rand.Read(b)
	if err != nil {
		panic("Error generating challenge: " + err.Error())
	}
	return b
}

// isValidProof checks whether the client's response along with challenge has the right
// amount of leading zero bits.
func isValidProof(challenge []byte, response string) bool {
	// Convert response to byte slice
	respBytes, err := hex.DecodeString(response)
	if err != nil {
		fmt.Println("Invalid response:", err)
		return false
	}

	// Combine challenge and response and hash
	h := sha256.New()
	h.Write(challenge)
	h.Write(respBytes)
	hash := h.Sum(nil)

	// Check leading zero bits (difficulty)
	for i := uint(0); i < difficulty/8; i++ {
		if hash[i] != 0 {
			return false
		}
	}

	extraBits := difficulty % 8

	// Check the bits of the last partial byte if needed
	mask := byte(1<<(8-extraBits)) - 1
	return hash[difficulty/8]&mask == 0
}

// getRandomQuote selects a random quote from the quotes slice.
func getRandomQuote() string {
	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(quotes))))
	return quotes[idx.Int64()]
}
