// Copyright (c) 2026 Nicolás Correa
// SPDX-License-Identifier: MIT

package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"newsletter-bot/internal/config"
)

const importSampleEmail = `<!doctype html><html><head><style>p{color:red}</style></head><body><div>` +
	`<p><strong>Miércoles 21 de octubre de 2026</strong></p><h1>Titular del día</h1><p>Apertura.</p>` +
	`<h2>🐧 Para entender</h2><h3>Thunderbird 160 corrige borrados</h3><p>Texto del destacado.</p>` +
	`<p><a href="https://linuxiac.com/nota/?utm_source=x&amp;a=1">Abrir enlace ›</a></p>` +
	`<h2>👁️ De un vistazo</h2><h3>🆕 Releases</h3><ul><li><strong>Wine 11.21</strong> mejora OpenGL. ` +
	`<a href="https://www.phoronix.com/news/wine">Abrir enlace ›</a></li></ul></div>` +
	`<script>alert(1)</script></body></html>`

func TestConvertEmailHTMLKeepsOnlyAllowlistedMarkup(t *testing.T) {
	body, err := sanitizeHTML(convertEmailHTML(importSampleEmail))
	if err != nil {
		t.Fatalf("converted body rejected by sanitizer: %v", err)
	}
	for _, forbidden := range []string{"<div", "<style", "<script", "<html", "alert(1)", "color:red"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("converted body still contains %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "<h1>Titular del día</h1>") {
		t.Fatalf("converted body lost the headline: %s", body)
	}
}

func TestEditionFromEmailHTMLExtractsStories(t *testing.T) {
	edition, err := editionFromEmailHTML(importSampleEmail, "tech", "2026-10-21", "Resumen tech del 21 de octubre")
	if err != nil {
		t.Fatalf("editionFromEmailHTML() error = %v", err)
	}
	if edition.Title != "Titular del día" {
		t.Fatalf("title = %q", edition.Title)
	}
	if len(edition.Stories) != 2 {
		t.Fatalf("stories = %d, want 2: %+v", len(edition.Stories), edition.Stories)
	}
	first := edition.Stories[0]
	if first.Title != "Thunderbird 160 corrige borrados" || first.Source != "linuxiac.com" || first.URL != "https://linuxiac.com/nota/?utm_source=x&a=1" {
		t.Fatalf("first story = %+v", first)
	}
	second := edition.Stories[1]
	if second.Title != "Wine 11.21" || !strings.HasPrefix(second.Summary, "Wine 11.21 mejora OpenGL") {
		t.Fatalf("second story = %+v", second)
	}
	if second.Published != "2026-10-21" {
		t.Fatalf("published = %q", second.Published)
	}
}

func TestEditionFromEmailHTMLReadsPlainTextSourceURL(t *testing.T) {
	body := `<h1>Sin enlaces HTML</h1><p>Crunchyroll sube precios en Argentina. Fuente: https://www.levelup.com/noticia/crunchyroll-sube.</p>`
	edition, err := editionFromEmailHTML(body, "animanga", "2026-10-23", "x")
	if err != nil {
		t.Fatalf("editionFromEmailHTML() error = %v", err)
	}
	if len(edition.Stories) != 1 || edition.Stories[0].URL != "https://www.levelup.com/noticia/crunchyroll-sube" {
		t.Fatalf("stories = %+v", edition.Stories)
	}
	if edition.Stories[0].Title == "" {
		t.Fatal("story without title")
	}
}

func TestEditionFromEmailHTMLRejectsEmailWithoutLinks(t *testing.T) {
	if _, err := editionFromEmailHTML(`<h1>Sin enlaces</h1><p>Solo texto.</p>`, "tech", "2026-10-21", "x"); err == nil {
		t.Fatal("edition without links was accepted")
	}
}

func TestImportHistoryPlansDryRunAndWritesOnce(t *testing.T) {
	root := t.TempDir()
	exportDir := filepath.Join(root, "export")
	if err := os.MkdirAll(filepath.Join(exportDir, "html"), 0o700); err != nil {
		t.Fatal(err)
	}
	single := `<h1>Uno</h1><p><a href="https://example.com/a">Abrir enlace ›</a></p>`
	double := `<h1>Dos</h1><h3>Historia A</h3><p>Resumen A.</p><p><a href="https://example.com/a">x</a></p>` +
		`<ul><li>Historia B. <a href="https://example.com/b">Abrir enlace ›</a></li></ul>`
	files := map[string]string{"html/one.html": single, "html/two.html": double, "html/test.html": double, "html/other.html": double}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(exportDir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index := []exportEntry{
		{MessageID: "one", Timestamp: "2026-10-21T10:40:00Z", Subject: "Resumen tech del 21 de octubre: uno", HTML: "html/one.html"},
		{MessageID: "two", Timestamp: "2026-10-21T11:00:00Z", Subject: "Resumen tech del 21 de octubre: dos", HTML: "html/two.html"},
		{MessageID: "test", Timestamp: "2026-10-22T10:40:00Z", Subject: "Resumen tech prueba", HTML: "html/test.html"},
		{MessageID: "other", Timestamp: "2026-10-22T10:40:00Z", Subject: "Hola", HTML: "html/other.html"},
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, "index.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := config.Configuration{
		Defaults:    config.Defaults{Timezone: "America/Argentina/Mendoza", FeedRetentionEditions: 30},
		Newsletters: []config.Newsletter{{Slug: "tech"}},
	}
	dataDir := filepath.Join(root, "data")

	plan, err := ImportHistory(configuration, exportDir, dataDir, true)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan, "\n")
	for _, expected := range []string{"would import tech 2026-10-21 (2 stories) from two", "skip test: test message", `skip other: unknown subject "Hola"`, "superseded"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("dry-run log missing %q:\n%s", expected, joined)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "history")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote history")
	}

	first, err := ImportHistory(configuration, exportDir, dataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(first, "\n"), "imported tech 2026-10-21 (2 stories)") {
		t.Fatalf("import log = %v", first)
	}
	stored, err := os.ReadFile(filepath.Join(dataDir, "history", "tech", "2026-10-21.json"))
	if err != nil {
		t.Fatal(err)
	}
	var edition Edition
	if err := json.Unmarshal(stored, &edition); err != nil || len(edition.Stories) != 2 || edition.ContentHTML == "" {
		t.Fatalf("stored edition = %s, error = %v", stored, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "public", "tech.xml")); err != nil {
		t.Fatalf("feeds not regenerated: %v", err)
	}

	second, err := ImportHistory(configuration, exportDir, dataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(second, "\n"), "edition already exists") {
		t.Fatalf("second import must not overwrite: %v", second)
	}
}
