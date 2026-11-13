package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"newsletter-bot/internal/pipeline"
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
	err := run([]string{"run"}, &bytes.Buffer{}, "unused")
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("run error = %v, want usage error", err)
	}
}

func TestRunRejectsExtraArguments(t *testing.T) {
	for _, args := range [][]string{{"status", "extra"}, {"serve", "extra"}, {"check-agents", "extra"}, {"run", "a", "b"}} {
		err := run(args, &bytes.Buffer{}, "unused")
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("run(%q) error = %v, want usage error", args, err)
		}
	}
}

type cliFakeAgent struct{ t *testing.T }

func (agent cliFakeAgent) Run(_ context.Context, request pipeline.Request) ([]byte, error) {
	agent.t.Helper()
	if request.Prompt != "test prompt" || request.Date == "" {
		agent.t.Fatalf("agent request = %+v", request)
	}
	return []byte(`{"title":"Test edition","content_html":"<p>Cuerpo</p>","stories":[{"title":"Test story","source":"Example","url":"https://example.com/story","published_at":"2026-11-11","summary":"Test summary"}]}`), nil
}

func TestRunCommandPersistsFakeEditionAndPublishesFeeds(t *testing.T) {
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
		filepath.Join(configDir, "newsletters", "sample.json"): `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","model":"gpt-6-luna","schedule":{"days":["monday"],"time":"09:00"}}`,
		filepath.Join(root, "prompts", "sample.md"):            "test prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := filepath.Join(root, "data")
	var output bytes.Buffer
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	if err := runWithAgent([]string{"run", "sample"}, &output, configDir, dataDir, cliFakeAgent{t}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"event":"newsletter_run_started"`, `"slug":"sample"`, `"agent":"codex"`, `"model_requested":"gpt-6-luna"`, `"event":"newsletter_run_finished"`, `"status":"success"`, `"duration_ms":`} {
		if !strings.Contains(logs.String(), expected) {
			t.Errorf("container logs missing %q: %s", expected, logs.String())
		}
	}
	if got, want := output.String(), "edition saved; feeds regenerated: sample\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "public", "sample.xml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "public", "all.xml")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "history", "sample"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("history entries = %v, error = %v", entries, err)
	}
}

func TestGenerateFeedsRespectsRunLock(t *testing.T) {
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
		filepath.Join(configDir, "newsletters", "sample.json"): `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"09:00"}}`,
		filepath.Join(root, "prompts", "sample.md"):            "test prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := filepath.Join(root, "data")
	unlock, err := pipeline.Lock(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = runWithAgent([]string{"generate-feeds"}, &bytes.Buffer{}, configDir, dataDir, nil)
	if err == nil || !strings.Contains(err.Error(), "acquire run lock") {
		t.Fatalf("error = %v, want run lock contention", err)
	}
}

func TestCheckAgentsChecksBothWithoutPrintingCommandOutput(t *testing.T) {
	var output bytes.Buffer
	var calls []string
	err := checkAgents(&output, func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(calls, ","), "codex login status,claude auth status"; got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
	if got, want := output.String(), "Codex: authenticated\nClaude Code: authenticated\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCheckAgentsReportsFailuresWithoutLeakingCommandOutput(t *testing.T) {
	var output bytes.Buffer
	err := checkAgents(&output, func(string, ...string) error {
		return errors.New("provider output with sensitive details")
	})
	if err == nil || !strings.Contains(err.Error(), "Codex, Claude Code") {
		t.Fatalf("error = %v, want both agents reported", err)
	}
	if strings.Contains(output.String(), "sensitive") || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("provider output leaked: %q %v", output.String(), err)
	}
	if got, want := output.String(), "Codex: not authenticated or unavailable\nClaude Code: not authenticated or unavailable\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunStatusReportsConfigAndLatestEditionWithoutContent(t *testing.T) {
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
		filepath.Join(configDir, "newsletters", "sample.json"): `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"09:00"}}`,
		filepath.Join(root, "prompts", "sample.md"):            "test prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := filepath.Join(root, "data")
	history := filepath.Join(dataDir, "history", "sample")
	if err := os.MkdirAll(history, 0o700); err != nil {
		t.Fatal(err)
	}
	edition := `{"slug":"sample","date":"2026-11-11","title":"Private title","content_html":"<p>Cuerpo</p>","stories":[{"title":"Private story","source":"Example","url":"https://example.com/story","published_at":"2026-11-11","summary":"Private summary"}]}`
	if err := os.WriteFile(filepath.Join(history, "2026-11-11.json"), []byte(edition), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runWithAgent([]string{"status"}, &output, configDir, dataDir, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "sample: enabled, agent=codex, schedule=monday 09:00, latest=2026-11-11\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if strings.Contains(output.String(), "Private") {
		t.Fatalf("status leaked edition content: %q", output.String())
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
