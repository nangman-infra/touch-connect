package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nangman-infra/touch-connect/tcctl/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		exitCode := 1
		if cliErr, ok := err.(cli.ExitError); ok {
			exitCode = cliErr.Code
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode)
	}
}
