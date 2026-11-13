// Copyright (c) 2026 Nicolás Correa
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFeedURLConfig(t *testing.T, defaults, linux, explicit string) string {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "newsletters"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(configDir, "defaults.json"):             defaults,
		filepath.Join(configDir, "newsletters", "linux.json"): linux,
		filepath.Join(configDir, "newsletters", "tech.json"):  explicit,
		filepath.Join(root, "prompts", "linux.md"):            "prompt",
		filepath.Join(root, "prompts", "tech.md"):             "prompt",
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return configDir
}

func TestLoadDerivesFeedURLFromBaseAndSlug(t *testing.T) {
	configDir := writeFeedURLConfig(t,
		`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":5,"feed_base_url":"https://feeds.example.com/"}`,
		`{"slug":"linux","enabled":true,"prompt":"prompts/linux.md","schedule":{"days":["monday"],"time":"06:00"}}`,
		`{"slug":"tech","enabled":true,"prompt":"prompts/tech.md","site_url":"https://news.example.com/","schedule":{"days":["monday"],"time":"06:10"}}`,
	)
	loaded, err := Load(configDir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, newsletter := range loaded.Newsletters {
		got[newsletter.Slug] = newsletter.SiteURL
	}
	if got["linux"] != "https://feeds.example.com/linux.xml" {
		t.Fatalf("linux site url = %q, want derived feed URL", got["linux"])
	}
	if got["tech"] != "https://news.example.com/" {
		t.Fatalf("tech site url = %q, explicit site_url must win", got["tech"])
	}
}

func TestLoadRejectsInvalidFeedBaseURL(t *testing.T) {
	configDir := writeFeedURLConfig(t,
		`{"timezone":"Etc/UTC","agent":"codex","timeout_minutes":5,"feed_base_url":"feeds.example.com"}`,
		`{"slug":"linux","enabled":true,"prompt":"prompts/linux.md","schedule":{"days":["monday"],"time":"06:00"}}`,
		`{"slug":"tech","enabled":true,"prompt":"prompts/tech.md","schedule":{"days":["monday"],"time":"06:10"}}`,
	)
	if _, err := Load(configDir); err == nil {
		t.Fatal("Load() accepted a feed_base_url without scheme")
	}
}
