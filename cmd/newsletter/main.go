package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
		return fmt.Errorf("usage: newsletter <validate-config|generate-crontab|timezone|serve|check-agents>")
	}
	if args[0] == "serve" {
		return serveFeeds()
	}
	if args[0] == "check-agents" {
		return checkAgents(out, runAgentCheck)
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
		return fmt.Errorf("unknown command %q; available: validate-config, generate-crontab, timezone, serve, check-agents", args[0])
	}
	return err
}

func runAgentCheck(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}

func checkAgents(out io.Writer, run func(string, ...string) error) error {
	checks := []struct {
		name   string
		binary string
		args   []string
	}{
		{name: "Codex", binary: "codex", args: []string{"login", "status"}},
		{name: "Claude Code", binary: "claude", args: []string{"auth", "status"}},
	}
	var unavailable []string
	for _, check := range checks {
		if err := run(check.binary, check.args...); err != nil {
			if _, writeErr := fmt.Fprintf(out, "%s: not authenticated or unavailable\n", check.name); writeErr != nil {
				return writeErr
			}
			unavailable = append(unavailable, check.name)
			continue
		}
		if _, err := fmt.Fprintf(out, "%s: authenticated\n", check.name); err != nil {
			return err
		}
	}
	if len(unavailable) > 0 {
		return fmt.Errorf("agent checks failed: %s", strings.Join(unavailable, ", "))
	}
	return nil
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
