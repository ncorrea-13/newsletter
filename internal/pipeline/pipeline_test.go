package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"newsletter-bot/internal/config"
)

type fakeAgent struct {
	result []byte
	check  func(Request)
}

func (f fakeAgent) Run(_ context.Context, request Request) ([]byte, error) {
	if f.check != nil {
		f.check(request)
	}
	return f.result, nil
}

func TestRunPersistsEditionAndRegeneratesRSS(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(configDir, "defaults.json"), `{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":2}`)
	write(filepath.Join(configDir, "newsletters", "sample.json"), `{"slug":"sample","title":"Sample Weekly","description":"Sampled news","site_url":"https://news.example.com/","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"09:00"}}`)
	write(filepath.Join(root, "prompts", "sample.md"), "Write one story.")
	loaded, err := config.Load(configDir)
	if err != nil {
		t.Fatal(err)
	}
	historyDir := filepath.Join(root, "data", "history", "sample")
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	previous := Edition{Slug: "sample", Date: "2026-11-10", Title: "Previous", Stories: []Story{{Title: "Old story", Source: "Example", URL: "https://example.com/old?utm_source=feed", Published: "2026-11-10", Summary: "Already covered."}}}
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(historyDir, "2026-11-10.json"), previousJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	generated := []byte(`{"title":"Weekly","stories":[{"title":"Story","source":"Example","url":"https://example.com/story","published_at":"2026-11-11","summary":"A useful update."}]}`)
	dataDir := filepath.Join(root, "data")
	date := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	if err := Run(context.Background(), loaded, "sample", dataDir, date, fakeAgent{result: generated, check: func(request Request) {
		if request.Prompt != "Write one story." || request.Date != "2026-11-11" || len(request.History) != 1 || request.History[0].Stories[0].URL != previous.Stories[0].URL {
			t.Errorf("request = %+v", request)
		}
	}}); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(dataDir, "history", "sample", "2026-11-11.json"))
	if err != nil {
		t.Fatal(err)
	}
	var edition Edition
	if err := json.Unmarshal(stored, &edition); err != nil || edition.Title != "Weekly" {
		t.Fatalf("stored edition = %s, decode error = %v", stored, err)
	}
	feed, err := os.ReadFile(filepath.Join(dataDir, "public", "sample.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(feed), "Story") || !strings.Contains(string(feed), "https://example.com/story") {
		t.Fatalf("feed missing edition: %s", feed)
	}
	if strings.Count(string(feed), "<item>") != 2 || !strings.Contains(string(feed), "sample:2026-11-11") {
		t.Fatalf("RSS should contain both editions with stable GUIDs: %s", feed)
	}
	for _, expected := range []string{"<title>Sample Weekly</title>", "<description>Sampled news</description>", "<link>https://news.example.com/</link>"} {
		if !strings.Contains(string(feed), expected) {
			t.Errorf("feed metadata missing %q: %s", expected, feed)
		}
	}
	allFeed, err := os.ReadFile(filepath.Join(dataDir, "public", "all.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(allFeed), "<title>all</title>") || !strings.Contains(string(allFeed), "all:2026-11-11") {
		t.Fatalf("all feed metadata = %s", allFeed)
	}
	calls := 0
	err = Run(context.Background(), loaded, "sample", dataDir, date, fakeAgent{result: generated, check: func(Request) { calls++ }})
	if err == nil || calls != 0 {
		t.Fatalf("duplicate run error = %v, agent calls = %d; want early rejection", err, calls)
	}
	duplicate := []byte(`{"title":"Next","stories":[{"title":"Old story again","source":"Example","url":"https://EXAMPLE.com:443/old#source","published_at":"2026-11-10","summary":"Repeating the same story."}]}`)
	err = Run(context.Background(), loaded, "sample", dataDir, date.AddDate(0, 0, 1), fakeAgent{result: duplicate})
	if err == nil || !strings.Contains(err.Error(), "duplicate story URL") {
		t.Fatalf("repeated story URL error = %v, want duplicate URL rejection", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "history", "sample", "2026-11-12.json")); !os.IsNotExist(err) {
		t.Fatalf("duplicate story edition persisted: %v", err)
	}
}

func TestRunUsesKernelLockAndRecoversStaleLockFile(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "defaults.json"), []byte(`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "newsletters", "sample.json"), []byte(`{"slug":"sample","enabled":true,"prompt":"prompts/sample.md","schedule":{"days":["monday"],"time":"09:00"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prompts", "sample.md"), []byte("prompt"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configDir)
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	lockDir := filepath.Join(dataDir, "state", "locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, "run.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenFile(lockPath, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	generated := []byte(`{"title":"Weekly","stories":[{"title":"Story","source":"Example","url":"https://example.com/story","published_at":"2026-11-11","summary":"Summary"}]}`)
	date := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	calls := 0
	err = Run(context.Background(), loaded, "sample", dataDir, date, fakeAgent{result: generated, check: func(Request) { calls++ }})
	if err == nil || !strings.Contains(err.Error(), "acquire run lock") || calls != 0 {
		t.Fatalf("concurrent run error = %v, agent calls = %d", err, calls)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), loaded, "sample", dataDir, date, fakeAgent{result: generated}); err != nil {
		t.Fatalf("run with stale lock file should succeed: %v", err)
	}
}

func TestNormalizeURLIgnoresFragmentsTrackingAndHostCase(t *testing.T) {
	got, err := normalizeURL("HTTPS://Example.com:443/story?utm_campaign=mail&z=2&a=1#section")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.com/story?a=1&z=2"; got != want {
		t.Fatalf("normalized URL = %q, want %q", got, want)
	}
	for _, raw := range []string{"ftp://example.com/x", "https://user:pass@example.com/x", "/relative", "https:///x", "javascript:alert(1)"} {
		if _, err := normalizeURL(raw); err == nil {
			t.Errorf("normalizeURL(%q) accepted invalid URL", raw)
		}
	}
}

func TestRunDoesNotReplaceExistingEdition(t *testing.T) {
	// The atomic create path is exercised against an existing edition below.
	root := t.TempDir()
	path := filepath.Join(root, "edition.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeNewAtomic(path, []byte("replacement")); err == nil {
		t.Fatal("expected existing edition to be preserved")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "original" {
		t.Fatalf("edition = %q, error = %v", got, err)
	}
}

func TestGenerateFeedsRebuildsFromHistoryInDateOrder(t *testing.T) {
	root := t.TempDir()
	history := filepath.Join(root, "history", "sample")
	if err := os.MkdirAll(history, 0o700); err != nil {
		t.Fatal(err)
	}
	for date, title := range map[string]string{"2026-11-10": "Older", "2026-11-11": "Newer"} {
		edition, err := json.Marshal(Edition{Slug: "sample", Date: date, Title: title, Stories: []Story{{Title: title, Source: "Example", URL: "https://example.com/" + date, Published: date, Summary: "Summary"}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(history, date+".json"), edition, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := GenerateFeeds([]config.Newsletter{{Slug: "sample"}}, root, 30); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sample.xml", "all.xml"} {
		feed, err := os.ReadFile(filepath.Join(root, "public", name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Index(string(feed), "Newer") > strings.Index(string(feed), "Older") {
			t.Fatalf("%s not newest-first: %s", name, feed)
		}
		if strings.Count(string(feed), "<item>") != 2 {
			t.Fatalf("%s item count incorrect: %s", name, feed)
		}
	}
	if err := GenerateFeeds([]config.Newsletter{{Slug: "sample"}}, root, 1); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(root, "public", "sample.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(feed), "<item>") != 1 || !strings.Contains(string(feed), "Newer") {
		t.Fatalf("retention should keep only newest edition: %s", feed)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "removed.xml"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := GenerateFeeds([]config.Newsletter{{Slug: "sample"}}, root, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "removed.xml")); !os.IsNotExist(err) {
		t.Fatalf("stale feed was not removed: %v", err)
	}
}

func TestGenerateFeedsEscapesAgentText(t *testing.T) {
	root := t.TempDir()
	history := filepath.Join(root, "history", "sample")
	if err := os.MkdirAll(history, 0o700); err != nil {
		t.Fatal(err)
	}
	edition, err := json.Marshal(Edition{Slug: "sample", Date: "2026-11-11", Title: "A & <B>", Stories: []Story{{Title: "<script>alert(1)</script>", Source: "Example", URL: "https://example.com/story?a=1&b=2", Published: "2026-11-11", Summary: "x & y <z>"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(history, "2026-11-11.json"), edition, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := GenerateFeeds([]config.Newsletter{{Slug: "sample"}}, root, 30); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(root, "public", "sample.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"A &amp; &lt;B&gt;", "&lt;script&gt;alert(1)&lt;/script&gt;", "x &amp; y &lt;z&gt;"} {
		if !strings.Contains(string(feed), expected) {
			t.Errorf("feed missing escaped text %q: %s", expected, feed)
		}
	}
	if strings.Contains(string(feed), "<script>") {
		t.Fatalf("feed contains unescaped XML: %s", feed)
	}
}

func setupPipeline(t *testing.T, root, defaults string, slugs ...string) config.Configuration {
	t.Helper()
	configDir := filepath.Join(root, "config")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o700))
	must(os.MkdirAll(filepath.Join(root, "prompts"), 0o700))
	must(os.WriteFile(filepath.Join(configDir, "defaults.json"), []byte(defaults), 0o600))
	for _, slug := range slugs {
		must(os.WriteFile(filepath.Join(configDir, "newsletters", slug+".json"), []byte(`{"slug":"`+slug+`","enabled":true,"prompt":"prompts/`+slug+`.md","schedule":{"days":["monday"],"time":"09:00"}}`), 0o600))
		must(os.WriteFile(filepath.Join(root, "prompts", slug+".md"), []byte("prompt"), 0o600))
	}
	loaded, err := config.Load(configDir)
	must(err)
	return loaded
}

func writeEditionFile(t *testing.T, root, slug, date, title string) {
	t.Helper()
	directory := filepath.Join(root, "data", "history", slug)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	edition, err := json.Marshal(Edition{Slug: slug, Date: date, Title: title, Stories: []Story{{Title: title, Source: "Example", URL: "https://example.com/" + slug + "/" + date, Published: date, Summary: "Summary"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, date+".json"), edition, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunTruncatesHistoryContextToConfiguredLimit(t *testing.T) {
	root := t.TempDir()
	loaded := setupPipeline(t, root, `{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":2,"history_context_editions":2}`, "sample")
	for _, date := range []string{"2026-11-08", "2026-11-09", "2026-11-10"} {
		writeEditionFile(t, root, "sample", date, "Edition "+date)
	}
	generated := []byte(`{"title":"Weekly","stories":[{"title":"Story","source":"Example","url":"https://example.com/story","published_at":"2026-11-12","summary":"Summary"}]}`)
	var received Request
	err := Run(context.Background(), loaded, "sample", filepath.Join(root, "data"), time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC), fakeAgent{result: generated, check: func(request Request) { received = request }})
	if err != nil {
		t.Fatal(err)
	}
	if len(received.History) != 2 || received.History[0].Date != "2026-11-10" || received.History[1].Date != "2026-11-09" {
		t.Fatalf("history = %+v, want newest two editions", received.History)
	}
}

func TestGenerateFeedsMergesNewslettersAndCapsItems(t *testing.T) {
	root := t.TempDir()
	writeEditionFile(t, root, "aaa", "2026-11-10", "Older A")
	writeEditionFile(t, root, "bbb", "2026-11-11", "Newer B")
	newsletters := []config.Newsletter{{Slug: "aaa"}, {Slug: "bbb"}}
	dataDir := filepath.Join(root, "data")
	if err := GenerateFeeds(newsletters, dataDir, 30); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(dataDir, "public", "all.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(feed), "<item>") != 2 || strings.Index(string(feed), "Newer B") > strings.Index(string(feed), "Older A") {
		t.Fatalf("all feed should merge newsletters newest first: %s", feed)
	}
	if err := GenerateFeeds(newsletters, dataDir, 1); err != nil {
		t.Fatal(err)
	}
	feed, err = os.ReadFile(filepath.Join(dataDir, "public", "all.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(feed), "<item>") != 1 || !strings.Contains(string(feed), "Newer B") {
		t.Fatalf("all feed should respect item cap: %s", feed)
	}
}
