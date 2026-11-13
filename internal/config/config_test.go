package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, slug, body, prompt string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"newsletters", "../prompts"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "defaults.json"), []byte(`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":30}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "newsletters", slug+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if prompt != "" {
		if err := os.WriteFile(filepath.Join(root, "../prompts", slug+".md"), []byte(prompt), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadAppliesDefaultsAndMapsOnePromptPerNewsletter(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday","wednesday","friday"],"time":"06:05"}}`, "prompt")
	config, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Newsletters) != 1 {
		t.Fatalf("got %d newsletters, want 1", len(config.Newsletters))
	}
	newsletter := config.Newsletters[0]
	if newsletter.Agent != "codex" || newsletter.TimeoutMinutes != 30 {
		t.Fatalf("defaults not applied: %+v", newsletter)
	}
	if config.Defaults.HistoryContextEditions != 5 {
		t.Fatalf("history context editions = %d, want default 5", config.Defaults.HistoryContextEditions)
	}
	if config.Defaults.FeedRetentionEditions != 30 {
		t.Fatalf("feed retention editions = %d, want default 30", config.Defaults.FeedRetentionEditions)
	}
	if newsletter.PromptPath != filepath.Join(root, "..", "prompts", "sample.md") {
		t.Fatalf("unexpected prompt path: %q", newsletter.PromptPath)
	}
}

func TestLoadRejectsExcessiveHistoryContext(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"06:00"}}`, "prompt")
	if err := os.WriteFile(filepath.Join(root, "defaults.json"), []byte(`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":30,"history_context_editions":31}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "history_context_editions") {
		t.Fatalf("Load error = %v, want history context range error", err)
	}
}

func TestLoadRejectsExcessiveFeedRetention(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"06:00"}}`, "prompt")
	if err := os.WriteFile(filepath.Join(root, "defaults.json"), []byte(`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":30,"feed_retention_editions":201}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "feed_retention_editions") {
		t.Fatalf("Load error = %v, want feed retention range error", err)
	}
}

func TestLoadAcceptsAndRejectsModelValues(t *testing.T) {
	body := func(model string) string {
		return `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","model":"` + model + `","schedule":{"days":["monday"],"time":"06:00"}}`
	}
	root := writeFixture(t, "sample", body("gpt-5.5"), "prompt")
	if loaded, err := Load(root); err != nil || loaded.Newsletters[0].Model != "gpt-5.5" {
		t.Fatalf("valid model rejected: %v", err)
	}
	for _, model := range []string{"-x", "--sandbox", "bad model", strings.Repeat("a", 101)} {
		root := writeFixture(t, "sample", body(model), "prompt")
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "model must be") {
			t.Fatalf("model %q: Load error = %v, want model validation error", model, err)
		}
	}
}

func TestLoadRejectsInvalidFeedSiteURL(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","site_url":"javascript:alert(1)","schedule":{"days":["monday"],"time":"06:00"}}`, "prompt")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "must be an absolute HTTP(S) URL") {
		t.Fatalf("Load error = %v, want invalid site_url error", err)
	}
}

func TestLoadRejectsMissingPrompt(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"06:00"}}`, "")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("Load error = %v, want missing prompt error", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"06:00"},"shell":"unsafe"}`, "prompt")
	if _, err := Load(root); err == nil {
		t.Fatal("Load accepted an unknown field")
	}
}

func TestRenderCrontabUsesConfiguredDaysAndSkipsDisabledNewsletters(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday","wednesday","friday"],"time":"06:05"}}`, "prompt")
	if err := os.WriteFile(filepath.Join(root, "newsletters", "other.json"), []byte(`{"slug":"other","enabled":false,"prompt":"prompts/other.md","schedule":{"days":["tuesday"],"time":"10:00"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "..", "prompts", "other.md"), []byte("prompt"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got := config.RenderCrontab()
	want := "CRON_TZ=Etc/UTC\n5 6 * * 1,3,5 newsletter run sample\n"
	if got != want {
		t.Fatalf("crontab = %q, want %q", got, want)
	}
}

func TestLoadRejectsPromptPathThatDoesNotMatchSlug(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/../config/defaults.json","schedule":{"days":["monday"],"time":"06:00"}}`, "prompt")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "must be") {
		t.Fatalf("Load error = %v, want prompt path mismatch", err)
	}
}

func TestLoadRejectsUnmappedPrompt(t *testing.T) {
	root := writeFixture(t, "sample", `{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"06:00"}}`, "prompt")
	if err := os.WriteFile(filepath.Join(root, "..", "prompts", "orphan.md"), []byte("prompt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "no matching config") {
		t.Fatalf("Load error = %v, want unmapped prompt error", err)
	}
}
