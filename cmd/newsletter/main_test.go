package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunValidateConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(configDir, "defaults.json"):              `{"timezone":"Etc/UTC","agent":"claude","timeout_minutes":30}`,
		filepath.Join(configDir, "newsletters", "sample.json"): `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["saturday"],"time":"12:00"}}`,
		filepath.Join(root, "prompts", "sample.md"):            "test prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := run([]string{"validate-config"}, &output, configDir); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "valid: 1 newsletter configurations\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunRejectsUnsupportedCommand(t *testing.T) {
	err := run([]string{"run", "sample"}, &bytes.Buffer{}, "unused")
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("run error = %v, want usage error", err)
	}
}

func TestRunPrintsConfiguredTimezone(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(configDir, "defaults.json"):              `{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":30}`,
		filepath.Join(configDir, "newsletters", "sample.json"): `{"slug":"sample","enabled":false,"prompt":"prompts/sample.md","schedule":{"days":["saturday"],"time":"12:00"}}`,
		filepath.Join(root, "prompts", "sample.md"):            "test prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := run([]string{"timezone"}, &output, configDir); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "Etc/UTC\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
