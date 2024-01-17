package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
)

const (
	ServerAddress = "localhost:8080"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := connect(ctx, ServerAddress)
	if err != nil {
		log.Panicln(err)
	}
}

func connect(ctx context.Context, addr string) error {
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error dialing: %v\n", err)
	}
	defer conn.Close()

	log.Println("Connected to server at", addr)

	reader := bufio.NewReader(conn)

	// Receive and parse the challenge from the server
	challengeStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Error reading challenge: %v\n", err)
	}

	var challenge []byte
	_, err = fmt.Sscanf(challengeStr, "Challenge: %x\n", &challenge)
	if err != nil {
		return fmt.Errorf("Error parsing challenge: %v\n", err)
	}

	// Receive and parse the difficulty from the server
	difficultyStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Error reading difficulty: %v\n", err)
	}

	var difficulty uint
	_, err = fmt.Sscanf(difficultyStr, "Difficulty: %d\n", &difficulty)
	if err != nil {
		return fmt.Errorf("Error parsing difficulty: %v\n", err)
	}

	log.Printf("Received Challenge: %x\n", challenge)
	log.Println("Received Difficulty:", difficulty)
	log.Println("Solving challenge...")

	// Solve the challenge with the provided difficulty
	nonce, err := solveChallenge(challenge, difficulty)
	if err != nil {
		return fmt.Errorf("Error solving challenge: %v\n", err)
	}

	log.Printf("Nonce: %x", nonce)

	// Send the nonce to the server
	_, err = fmt.Fprintf(conn, "%x\n", nonce)
	if err != nil {
		return fmt.Errorf("Error sending nonce: %v\n", err)
	}

	// Receive quote from server
	quote, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("Error reading quote: %v\n", err)
	}

	log.Printf("Quote: '%s'\n", quote)
	return nil
}

// solveChallenge tries to find a nonce that when hashed with the challenge,
// results in a hash with the sufficient number of leading zeroes (as defined by difficulty).
func solveChallenge(challenge []byte, difficulty uint) ([]byte, error) {
	var nonce [32]byte
	var hash [32]byte
	h := sha256.New()

	for {
		// Generate a random nonce
		_, err := rand.Read(nonce[:])
		if err != nil {
			return nil, err
		}

		// Compute the hash
		h.Reset()
		h.Write(challenge)
		h.Write(nonce[:])
		hash = [32]byte(h.Sum(nil))

		// Check if the hash meets the difficulty criteria
		if checkDifficulty(hash[:], difficulty) {
			return nonce[:], nil
		}
	}
}

// checkDifficulty checks if the computed hash meets the required difficulty of leading zeros.
// This function is the counterpart of isValidProof from the server but just checks against the hash.
func checkDifficulty(hash []byte, difficulty uint) bool {
	zeroBytes := difficulty / 8
	for i := uint(0); i < zeroBytes; i++ {
		if hash[i] != 0 {
			return false
		}
	}

	remainingBits := difficulty % 8
	if remainingBits == 0 {
		return true
	}

	mask := byte(^uint(0) >> remainingBits)
	return (hash[zeroBytes] & mask) == 0
}
