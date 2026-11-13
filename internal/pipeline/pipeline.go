// Copyright (c) 2026 Nicolás Correa
// SPDX-License-Identifier: MIT

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"newsletter-bot/internal/config"
)

type Story struct {
	Title     string `json:"title"`
	Source    string `json:"source"`
	URL       string `json:"url"`
	Published string `json:"published_at"`
	Summary   string `json:"summary"`
}

type Edition struct {
	Slug        string  `json:"slug"`
	Date        string  `json:"date"`
	Title       string  `json:"title"`
	ContentHTML string  `json:"content_html,omitempty"`
	Stories     []Story `json:"stories"`
}

type Request struct {
	Prompt  string
	Date    string
	History []RecentEdition
}

type RecentEdition struct {
	Date    string        `json:"date"`
	Title   string        `json:"title"`
	Stories []RecentStory `json:"stories"`
}

type RecentStory struct {
	Title  string `json:"title"`
	Source string `json:"source"`
	URL    string `json:"url"`
}

type Agent interface {
	Run(context.Context, Request) ([]byte, error)
}

func Lock(dataDir string) (func(), error) {
	return lockFile(dataDir, "feeds.lock", true)
}

func LockFeeds(dataDir string) (func(), error) {
	return lockFile(dataDir, "feeds.lock", false)
}

func LockNewsletter(dataDir, slug string) (func(), error) {
	if slug == "" || filepath.Base(slug) != slug {
		return nil, fmt.Errorf("invalid newsletter slug %q", slug)
	}
	return lockFile(dataDir, "newsletter-"+slug+".lock", true)
}

func lockFile(dataDir, name string, nonBlocking bool) (func(), error) {
	lockDir := filepath.Join(dataDir, "state", "locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(lockDir, name), os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open run lock: %w", err)
	}
	operation := syscall.LOCK_EX
	if nonBlocking {
		operation |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(lock.Fd()), operation); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}
	return func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); _ = lock.Close() }, nil
}

func Run(ctx context.Context, configuration config.Configuration, slug, dataDir string, now time.Time, agent Agent) error {
	var newsletter *config.Newsletter
	for i := range configuration.Newsletters {
		if configuration.Newsletters[i].Slug == slug {
			newsletter = &configuration.Newsletters[i]
			break
		}
	}
	if newsletter == nil {
		return fmt.Errorf("unknown newsletter %q", slug)
	}
	unlock, err := LockNewsletter(dataDir, newsletter.Slug)
	if err != nil {
		return err
	}
	defer unlock()

	location, err := time.LoadLocation(configuration.Defaults.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}
	date := now.In(location).Format("2006-01-02")
	historyDir := filepath.Join(dataDir, "history", newsletter.Slug)
	path := filepath.Join(historyDir, date+".json")
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("edition already exists for %s on %s", newsletter.Slug, date)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing edition: %w", err)
	}
	priorEditions, err := readEditions(historyDir, newsletter.Slug)
	if err != nil {
		return fmt.Errorf("read recent history: %w", err)
	}
	if len(priorEditions) > configuration.Defaults.HistoryContextEditions {
		priorEditions = priorEditions[:configuration.Defaults.HistoryContextEditions]
	}
	history := make([]RecentEdition, 0, len(priorEditions))
	seenURLs := make(map[string]struct{})
	for _, prior := range priorEditions {
		recent := RecentEdition{Date: prior.Date, Title: prior.Title, Stories: make([]RecentStory, 0, len(prior.Stories))}
		for _, story := range prior.Stories {
			normalized, err := normalizeURL(story.URL)
			if err != nil {
				return fmt.Errorf("invalid URL in edition %s: %w", prior.Date, err)
			}
			seenURLs[normalized] = struct{}{}
			recent.Stories = append(recent.Stories, RecentStory{Title: story.Title, Source: story.Source, URL: story.URL})
		}
		history = append(history, recent)
	}
	prompt, err := os.ReadFile(newsletter.PromptPath)
	if err != nil {
		return fmt.Errorf("read prompt: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(newsletter.TimeoutMinutes)*time.Minute)
	defer cancel()
	output, err := agent.Run(ctx, Request{Prompt: string(prompt), Date: date, History: history})
	if err != nil {
		return fmt.Errorf("run agent: %w", err)
	}
	var edition Edition
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&edition); err != nil {
		return fmt.Errorf("decode edition: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("decode edition: multiple JSON values")
	}
	if strings.TrimSpace(edition.Title) == "" || len(edition.Stories) == 0 {
		return fmt.Errorf("edition title and at least one story are required")
	}
	if strings.TrimSpace(edition.ContentHTML) == "" {
		return fmt.Errorf("edition content_html is required")
	}
	sanitized, err := sanitizeHTML(edition.ContentHTML)
	if err != nil {
		return fmt.Errorf("invalid content_html: %w", err)
	}
	edition.ContentHTML = sanitized
	for i, story := range edition.Stories {
		normalized, err := normalizeURL(story.URL)
		if strings.TrimSpace(story.Title) == "" || strings.TrimSpace(story.Source) == "" || err != nil || strings.TrimSpace(story.Summary) == "" {
			return fmt.Errorf("story %d has invalid required fields or URL", i+1)
		}
		if _, duplicate := seenURLs[normalized]; duplicate {
			return fmt.Errorf("story %d has duplicate story URL", i+1)
		}
		seenURLs[normalized] = struct{}{}
		published, err := time.Parse("2006-01-02", story.Published)
		if err != nil || published.Format("2006-01-02") != story.Published {
			return fmt.Errorf("story %d has invalid published_at date", i+1)
		}
	}
	edition.Slug, edition.Date = newsletter.Slug, date
	encoded, err := json.MarshalIndent(edition, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return err
	}
	if err := writeNewAtomic(path, encoded); err != nil {
		return fmt.Errorf("persist edition: %w", err)
	}
	unlockFeeds, err := LockFeeds(dataDir)
	if err != nil {
		return fmt.Errorf("acquire feed lock: %w", err)
	}
	defer unlockFeeds()
	if err := GenerateFeeds(configuration.Newsletters, dataDir, configuration.Defaults.FeedRetentionEditions); err != nil {
		return fmt.Errorf("edition saved at %s; regenerate feeds with newsletter generate-feeds: %w", path, err)
	}
	return nil
}

func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("URL must be absolute and contain no user information")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("URL host is required")
	}
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment, u.RawFragment = "", ""
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func LatestEdition(slug, dataDir string) (Edition, bool, error) {
	editions, err := readEditions(filepath.Join(dataDir, "history", slug), slug)
	if err != nil {
		return Edition{}, false, err
	}
	if len(editions) == 0 {
		return Edition{}, false, nil
	}
	return editions[0], true, nil
}

func GenerateFeeds(newsletters []config.Newsletter, dataDir string, retention int) error {
	if retention < 1 {
		retention = 30
	}
	var all []Edition
	for _, newsletter := range newsletters {
		editions, err := readEditions(filepath.Join(dataDir, "history", newsletter.Slug), newsletter.Slug)
		if err != nil {
			return fmt.Errorf("load %s history: %w", newsletter.Slug, err)
		}
		if len(editions) > retention {
			editions = editions[:retention]
		}
		title := newsletter.Title
		if title == "" {
			title = newsletter.Slug
		}
		description := newsletter.Description
		if description == "" {
			description = newsletter.Slug
		}
		if err := writeFeedAtomic(filepath.Join(dataDir, "public", newsletter.Slug+".xml"), feedMeta{Title: title, Description: description, Link: newsletter.SiteURL, GUIDPrefix: newsletter.Slug}, editions); err != nil {
			return fmt.Errorf("write %s feed: %w", newsletter.Slug, err)
		}
		all = append(all, editions...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Date > all[j].Date })
	if len(all) > retention {
		all = all[:retention]
	}
	if err := writeFeedAtomic(filepath.Join(dataDir, "public", "all.xml"), feedMeta{Title: "all", Description: "All editions", GUIDPrefix: "all"}, all); err != nil {
		return err
	}
	return pruneStaleFeeds(dataDir, newsletters)
}

func pruneStaleFeeds(dataDir string, newsletters []config.Newsletter) error {
	expected := map[string]bool{"all.xml": true}
	for _, newsletter := range newsletters {
		expected[newsletter.Slug+".xml"] = true
	}
	publicDir := filepath.Join(dataDir, "public")
	entries, err := os.ReadDir(publicDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".xml" || expected[entry.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(publicDir, entry.Name())); err != nil {
			return fmt.Errorf("remove stale feed %s: %w", entry.Name(), err)
		}
	}
	return nil
}

type feedMeta struct {
	Title       string
	Description string
	Link        string
	GUIDPrefix  string
}

func readEditions(directory, slug string) ([]Edition, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var editions []Edition
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("edition %q must not be a symlink", entry.Name())
		}
		date := strings.TrimSuffix(entry.Name(), ".json")
		parsedDate, err := time.Parse("2006-01-02", date)
		if err != nil || parsedDate.Format("2006-01-02") != date {
			return nil, fmt.Errorf("invalid edition filename %q", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		var edition Edition
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&edition); err != nil {
			return nil, fmt.Errorf("decode %s: %w", entry.Name(), err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("decode %s: trailing JSON data", entry.Name())
		}
		if edition.Slug != slug || edition.Date != date || edition.Title == "" || len(edition.Stories) == 0 {
			return nil, fmt.Errorf("edition metadata mismatch in %s", entry.Name())
		}
		editions = append(editions, edition)
	}
	sort.Slice(editions, func(i, j int) bool { return editions[i].Date > editions[j].Date })
	return editions, nil
}

func writeNewAtomic(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".edition-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Link(temp.Name(), path); err != nil {
		return err
	}
	return syncDir(path)
}

func syncDir(path string) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}
type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}
type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        guid   `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}
type guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func writeFeedAtomic(path string, meta feedMeta, editions []Edition) error {
	items := make([]item, 0, len(editions))
	for _, edition := range editions {
		date, err := time.Parse("2006-01-02", edition.Date)
		if err != nil {
			return err
		}
		description := edition.ContentHTML
		if description == "" {
			// Editions saved before content_html existed keep the plain summary format.
			var descriptions []string
			for _, story := range edition.Stories {
				descriptions = append(descriptions, story.Source+": "+story.Title+": "+story.Summary)
			}
			description = strings.Join(descriptions, "\n\n")
		}
		items = append(items, item{Title: edition.Title, Link: edition.Stories[0].URL, GUID: guid{Value: meta.GUIDPrefix + ":" + edition.Date}, Description: description, PubDate: date.UTC().Format(time.RFC1123Z)})
	}
	link := meta.Link
	if link == "" && len(items) > 0 {
		link = items[0].Link
	}
	feed := rss{Version: "2.0", Channel: channel{Title: meta.Title, Link: link, Description: meta.Description, Items: items}}
	content, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}
	content = append([]byte(xml.Header), content...)
	return replaceAtomic(path, content)
}

func replaceAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".feed-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return err
	}
	return syncDir(path)
}
