package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func main() {
	serverURL := os.Getenv("TC_WORKER_SERVER_URL")
	messageRef := os.Getenv("TC_WORKER_MESSAGE_REF")
	if serverURL == "" {
		log.Fatal("TC_WORKER_SERVER_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if messageRef != "" {
		attemptRef, err := tcworker.RegisterAndProcess(ctx, serverURL, messageRef)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("completed attempt %s", attemptRef)
		return
	}
	options, err := tcworker.LoopOptionsFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if err := tcworker.RegisterAndRun(ctx, serverURL, options); err != nil {
		log.Fatal(err)
	}
}
