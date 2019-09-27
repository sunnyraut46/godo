package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/digitalocean/godo/test/e2e/framework/server"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:3000"
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s\n", addr)
	}

	e2eServer := server.New(os.Getenv("TEMPLATES_PATH"))
	server := &http.Server{
		Handler: e2eServer,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		log.Println("shutting down")

		{
			// Give one second for running requests to complete
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			err := server.Shutdown(ctx)
			if err != nil {
				log.Printf("failed to shutdown: %s\n", err)
			}
			cancel()
			close(c)
		}
	}()

	log.Printf("serving on %s\n", addr)
	err = server.Serve(l)
	log.Printf("serving ended: %s\n", err)
}
