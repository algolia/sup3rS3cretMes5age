// Package main provides the entry point for the sup3rS3cretMes5age application,
// a secure self-destructing message service using HashiCorp Vault as a backend.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/algolia/sup3rS3cretMes5age/internal"
)

// version holds the application version string, injected at build time via ldflags.
var version = ""

func main() {
	versionFlag := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}
	// Load configuration
	conf := internal.LoadConfig()

	// Setup context first: it drives both the graceful shutdown and the
	// Vault token renewal goroutine started by NewVault.
	ctx, cancel := context.WithCancel(context.Background())

	// Create server with handlers; fail loudly if Vault is unreachable or
	// the token is invalid rather than serving errors to every request.
	store, err := internal.NewVault(ctx, "", conf.VaultPrefix, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Vault initialization failed: %v\n", err)
		os.Exit(1)
	}
	handlers := internal.NewSecretHandlers(store)
	server := internal.NewServer(conf, handlers)

	// Listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nShutting down gracefully...")

	// Cancel context to signal server to stop
	cancel()

	// Give server 10 seconds to finish existing requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Server stopped successfully")
}
