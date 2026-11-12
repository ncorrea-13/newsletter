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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"newsletter-bot/internal/agent"
	"newsletter-bot/internal/config"
	"newsletter-bot/internal/feedserver"
	"newsletter-bot/internal/pipeline"
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
	return runWithAgent(args, out, configDir, os.Getenv("NEWSLETTER_DATA_DIR"), nil)
}

func runWithAgent(args []string, out io.Writer, configDir, dataDir string, injectedAgent pipeline.Agent) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: newsletter <validate-config|generate-crontab|generate-feeds|run <slug>|timezone|status|serve|check-agents>")
	}
	if args[0] == "serve" {
		if len(args) != 1 {
			return fmt.Errorf("usage: newsletter serve")
		}
		return serveFeeds()
	}
	if args[0] == "check-agents" {
		if len(args) != 1 {
			return fmt.Errorf("usage: newsletter check-agents")
		}
		return checkAgents(out, runAgentCheck)
	}
	usages := map[string]string{
		"run":              "newsletter run <slug>",
		"generate-feeds":   "newsletter generate-feeds",
		"validate-config":  "newsletter validate-config",
		"generate-crontab": "newsletter generate-crontab",
		"timezone":         "newsletter timezone",
		"status":           "newsletter status",
	}
	usage, ok := usages[args[0]]
	if !ok {
		return fmt.Errorf("unknown command %q; available: validate-config, generate-crontab, generate-feeds, run, timezone, status, serve, check-agents", args[0])
	}
	if (args[0] == "run" && len(args) != 2) || (args[0] != "run" && len(args) != 1) {
		return fmt.Errorf("usage: %s", usage)
	}
	loaded, err := config.Load(configDir)
	if err != nil {
		return err
	}
	switch args[0] {
	case "run":
		runner := injectedAgent
		if runner == nil {
			runner, err = agent.ForNewsletter(loaded.Newsletters, args[1])
			if err != nil {
				return err
			}
		}
		if dataDir == "" {
			dataDir = filepath.Join(filepath.Dir(configDir), "data")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		err = pipeline.Run(ctx, loaded, args[1], dataDir, time.Now(), runner)
		if err == nil {
			_, err = fmt.Fprintf(out, "edition saved; feeds regenerated: %s\n", args[1])
		}
	case "generate-feeds":
		if dataDir == "" {
			dataDir = filepath.Join(filepath.Dir(configDir), "data")
		}
		unlock, err := pipeline.Lock(dataDir)
		if err != nil {
			return err
		}
		defer unlock()
		err = pipeline.GenerateFeeds(loaded.Newsletters, dataDir, loaded.Defaults.FeedRetentionEditions)
	case "validate-config":
		_, err = fmt.Fprintf(out, "valid: %d newsletter configurations\n", len(loaded.Newsletters))
	case "generate-crontab":
		_, err = io.WriteString(out, loaded.RenderCrontab())
	case "timezone":
		_, err = fmt.Fprintln(out, loaded.Defaults.Timezone)
	case "status":
		if dataDir == "" {
			dataDir = filepath.Join(filepath.Dir(configDir), "data")
		}
		for _, newsletter := range loaded.Newsletters {
			enabled := "disabled"
			if *newsletter.Enabled {
				enabled = "enabled"
			}
			latest := "none"
			edition, found, latestErr := pipeline.LatestEdition(newsletter.Slug, dataDir)
			if latestErr != nil {
				return latestErr
			}
			if found {
				latest = edition.Date
			}
			if _, err = fmt.Fprintf(out, "%s: %s, agent=%s, schedule=%s %s, latest=%s\n", newsletter.Slug, enabled, newsletter.Agent, strings.Join(newsletter.Schedule.Days, ","), newsletter.Schedule.Time, latest); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown command %q; available: validate-config, generate-crontab, generate-feeds, run, timezone, status, serve, check-agents", args[0])
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
