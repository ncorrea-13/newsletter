package main

import (
	"fmt"
	"io"
	"os"

	"newsletter-bot/internal/config"
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
		return fmt.Errorf("usage: newsletter <validate-config|generate-crontab|timezone>")
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
		return fmt.Errorf("unknown command %q; available: validate-config, generate-crontab, timezone", args[0])
	}
	return err
}
