package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/xenking/pow-challenge-poc/internal/app"
	"github.com/xenking/pow-challenge-poc/internal/server"
	"github.com/xenking/pow-challenge-poc/internal/storage/memory"
)

func main() {
	globalCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	storage := memory.New()

	application, err := app.New(storage, "./quotes.json")
	if err != nil {
		panic(err)
	}

	srv := &server.Server{
		Handler: application.Handler,
	}

	waitShutdownErr := make(chan error)

	go func() {
		<-globalCtx.Done()
		waitShutdownErr <- srv.Shutdown()
	}()

	serverAttr := os.Getenv("SERVER_ADDR")
	if serverAttr == "" {
		serverAttr = ":8080"
	}

	ln, err := net.Listen("tcp4", serverAttr)
	if err != nil {
		panic(err)
	}

	log.Println("listening on", serverAttr)

	_ = srv.ServeContext(globalCtx, ln)

	err = <-waitShutdownErr
	if err != nil {
		panic(err)
	}
}
