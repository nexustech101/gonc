// GoNC - A cross-platform netcat clone with native SSH tunneling.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gonc/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Execute(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gonc: %v\n", err)
		os.Exit(1)
	}
}
