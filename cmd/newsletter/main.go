package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"newsletter-bot/internal/config"
	"newsletter-bot/internal/feedserver"
)

func main() {
	configDir := os.Getenv("NEWSLETTER_CONFIG_DIR")
	if configDir == "" {
		configDir = "config"
	}
	if err := run(os.Args[1:], os.Stdout, configDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer, configDir string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: newsletter <validate-config|generate-crontab|timezone|serve>")
	}
	if args[0] == "serve" {
		return serveFeeds()
	}
	loaded, err := config.Load(configDir)
	if err != nil {
		return err
	}
	switch args[0] {
	case "validate-config":
		_, err = fmt.Fprintf(out, "valid: %d newsletter configurations\n", len(loaded.Newsletters))
	case "generate-crontab":
		_, err = io.WriteString(out, loaded.RenderCrontab())
	case "timezone":
		_, err = fmt.Fprintln(out, loaded.Defaults.Timezone)
	default:
		return fmt.Errorf("unknown command %q; available: validate-config, generate-crontab, timezone, serve", args[0])
	}
	return err
}

func serveFeeds() error {
	directory := os.Getenv("FEED_DIR")
	if directory == "" {
		directory = "/data/public"
	}
	address := os.Getenv("FEED_LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}

	handler, err := feedserver.New(directory)
	if err != nil {
		return fmt.Errorf("open feed directory: %w", err)
	}
	defer handler.Close()

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
