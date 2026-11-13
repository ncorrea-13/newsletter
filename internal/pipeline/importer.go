package pipeline

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"newsletter-bot/internal/config"
)

// subjectSlugs maps the subject prefix of the old newsletter emails to a configured slug.
// Unknown subjects are reported and skipped, never guessed.
var subjectSlugs = []struct {
	prefix string
	slug   string
}{
	{"Resumen tech", "tech"},
	{"Resumen videojuegos", "videojuegos"},
	{"Resumen Linux", "linux"},
	{"Resumen policiales Mendoza", "policiales"},
	{"Resumen animanga", "animanga"},
	{"Resumen Cosmere RPG", "cosmere"},
	{"Resumen eventos IT Mendoza", "eventos-it"},
	{"Resumen economía práctica", "economia"},
}

type exportEntry struct {
	MessageID string `json:"message_id"`
	Timestamp string `json:"timestamp"`
	Subject   string `json:"subject"`
	HTML      string `json:"html"`
}

var (
	importDropBlocks   = regexp.MustCompile(`(?is)<(script|style|head|title)\b[^>]*>.*?</(?:script|style|head|title)\s*>`)
	importDropComments = regexp.MustCompile(`(?s)<!--.*?-->|<![^>]*>`)
	importTag          = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	importAttr         = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	importH1           = regexp.MustCompile(`(?is)<h1>(.*?)</h1>`)
	// Loose text and bare links count as blocks: layout tags are dropped, so their text is left outside <p>.
	importBlock     = regexp.MustCompile(`(?is)<h3>.*?</h3>|<li>.*?</li>|<p>.*?</p>|<a href="[^"]+">.*?</a>|[^<]+`)
	importLink      = regexp.MustCompile(`(?is)<a href="([^"]+)">(.*?)</a>`)
	importBareURL   = regexp.MustCompile(`https?://[^\s<>"']+`)
	importStrong    = regexp.MustCompile(`(?is)<strong>(.*?)</strong>`)
	importStripTags = regexp.MustCompile(`(?s)<[^>]*>`)
	importSpaces    = regexp.MustCompile(`\s+`)
)

// ImportHistory converts the exported newsletter emails into editions under dataDir.
// It never overwrites an existing edition. With dryRun it only reports the plan.
func ImportHistory(configuration config.Configuration, exportDir, dataDir string, dryRun bool) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(exportDir, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("read export index: %w", err)
	}
	var entries []exportEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode export index: %w", err)
	}
	location, err := time.LoadLocation(configuration.Defaults.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	configured := map[string]bool{}
	for _, newsletter := range configuration.Newsletters {
		configured[newsletter.Slug] = true
	}

	var log []string
	best := map[string]importCandidate{}
	for _, entry := range entries {
		slug, ok := slugForSubject(entry.Subject)
		switch {
		case !ok:
			log = append(log, fmt.Sprintf("skip %s: unknown subject %q", entry.MessageID, entry.Subject))
			continue
		case isTestSubject(entry.Subject):
			log = append(log, fmt.Sprintf("skip %s: test message", entry.MessageID))
			continue
		case !configured[slug]:
			log = append(log, fmt.Sprintf("skip %s: slug %q is not configured", entry.MessageID, slug))
			continue
		case !filepath.IsLocal(entry.HTML):
			log = append(log, fmt.Sprintf("skip %s: html path %q escapes the export", entry.MessageID, entry.HTML))
			continue
		}
		when, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			log = append(log, fmt.Sprintf("skip %s: bad timestamp %q", entry.MessageID, entry.Timestamp))
			continue
		}
		date := when.In(location).Format("2006-01-02")
		body, err := os.ReadFile(filepath.Join(exportDir, entry.HTML))
		if err != nil {
			log = append(log, fmt.Sprintf("skip %s: read html: %v", entry.MessageID, err))
			continue
		}
		edition, err := editionFromEmailHTML(string(body), slug, date, entry.Subject)
		if err != nil {
			log = append(log, fmt.Sprintf("skip %s (%s %s): %v", entry.MessageID, slug, date, err))
			continue
		}
		key := slug + " " + date
		candidate := importCandidate{edition: edition, messageID: entry.MessageID, when: when}
		previous, exists := best[key]
		switch {
		case !exists:
			best[key] = candidate
		case len(edition.Stories) > len(previous.edition.Stories),
			len(edition.Stories) == len(previous.edition.Stories) && when.After(previous.when):
			log = append(log, fmt.Sprintf("skip %s: superseded for %s by %s", previous.messageID, key, entry.MessageID))
			best[key] = candidate
		default:
			log = append(log, fmt.Sprintf("skip %s: duplicate for %s, kept %s", entry.MessageID, key, previous.messageID))
		}
	}

	keys := make([]string, 0, len(best))
	for key := range best {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	imported := 0
	for _, key := range keys {
		candidate := best[key]
		edition := candidate.edition
		if dryRun {
			log = append(log, fmt.Sprintf("would import %s (%d stories) from %s", key, len(edition.Stories), candidate.messageID))
			continue
		}
		written, err := writeImportedEdition(dataDir, edition)
		if err != nil {
			log = append(log, fmt.Sprintf("fail %s: %v", key, err))
			continue
		}
		if written {
			imported++
			log = append(log, fmt.Sprintf("imported %s (%d stories)", key, len(edition.Stories)))
		} else {
			log = append(log, fmt.Sprintf("skip %s: edition already exists", key))
		}
	}
	if !dryRun && imported > 0 {
		unlock, err := LockFeeds(dataDir)
		if err != nil {
			return log, fmt.Errorf("acquire feed lock: %w", err)
		}
		defer unlock()
		if err := GenerateFeeds(configuration.Newsletters, dataDir, configuration.Defaults.FeedRetentionEditions); err != nil {
			return log, fmt.Errorf("regenerate feeds: %w", err)
		}
		log = append(log, fmt.Sprintf("feeds regenerated with %d imported editions", imported))
	}
	return log, nil
}

type importCandidate struct {
	edition   Edition
	messageID string
	when      time.Time
}

func slugForSubject(subject string) (string, bool) {
	for _, mapping := range subjectSlugs {
		if strings.HasPrefix(subject, mapping.prefix) {
			return mapping.slug, true
		}
	}
	return "", false
}

func isTestSubject(subject string) bool {
	lower := strings.ToLower(subject)
	return strings.Contains(lower, "prueba") || strings.Contains(lower, "diagnóstico") || strings.Contains(lower, "diagnostico")
}

// writeImportedEdition stores the edition unless one already exists for that slug and date.
func writeImportedEdition(dataDir string, edition Edition) (bool, error) {
	historyDir := filepath.Join(dataDir, "history", edition.Slug)
	path := filepath.Join(historyDir, edition.Date+".json")
	if _, err := os.Lstat(path); err == nil {
		return false, nil
	}
	unlock, err := LockNewsletter(dataDir, edition.Slug)
	if err != nil {
		return false, err
	}
	defer unlock()
	encoded, err := json.MarshalIndent(edition, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return false, err
	}
	if err := writeNewAtomic(path, encoded); err != nil {
		return false, err
	}
	return true, nil
}

// editionFromEmailHTML builds an edition from one exported email body.
// The result passes the same content checks as a generated edition.
func editionFromEmailHTML(raw, slug, date, subject string) (Edition, error) {
	body, err := sanitizeHTML(convertEmailHTML(raw))
	if err != nil {
		return Edition{}, fmt.Errorf("content: %w", err)
	}
	title := ""
	if match := importH1.FindStringSubmatch(body); match != nil {
		title = plainText(match[1])
	}
	if title == "" {
		title = subject
	}
	stories := extractStories(body, date)
	if len(stories) == 0 {
		return Edition{}, fmt.Errorf("no stories with links")
	}
	return Edition{Slug: slug, Date: date, Title: title, ContentHTML: body, Stories: stories}, nil
}

// convertEmailHTML keeps the allowlisted tags and attributes of an email body and
// drops everything else, keeping the text of dropped layout tags.
func convertEmailHTML(raw string) string {
	s := importDropComments.ReplaceAllString(raw, "")
	s = importDropBlocks.ReplaceAllString(s, "")
	var out strings.Builder
	last := 0
	for _, loc := range importTag.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(strings.ReplaceAll(s[last:loc[0]], "<", "&lt;"))
		last = loc[1]
		name := strings.ToLower(s[loc[4]:loc[5]])
		rule, ok := allowedHTMLTags[name]
		if !ok {
			continue
		}
		if s[loc[2]:loc[3]] == "/" {
			if !rule.void {
				out.WriteString("</" + name + ">")
			}
			continue
		}
		var attrs strings.Builder
		for _, match := range importAttr.FindAllStringSubmatch(s[loc[6]:loc[7]], -1) {
			attr := strings.ToLower(match[1])
			value := match[2] + match[3] + match[4]
			if !contains(rule.attrs, attr) {
				continue
			}
			if (attr == "href" || attr == "src") && requireAbsoluteHTTP(value) != nil {
				continue
			}
			if attr == "width" && !htmlDigits.MatchString(value) {
				continue
			}
			value = strings.NewReplacer(`"`, "&quot;", "<", "&lt;", ">", "&gt;").Replace(value)
			attrs.WriteString(` ` + attr + `="` + value + `"`)
		}
		out.WriteString("<" + name + attrs.String() + ">")
	}
	out.WriteString(strings.ReplaceAll(s[last:], "<", "&lt;"))
	return out.String()
}

// extractStories takes the first new link of each list item or paragraph as a story.
// Titles come from the nearest h3 for paragraphs and from the bold text for list items.
func extractStories(body, date string) []Story {
	var stories []Story
	seen := map[string]bool{}
	heading := ""
	for _, block := range importBlock.FindAllString(body, -1) {
		lower := strings.ToLower(block)
		if strings.HasPrefix(lower, "<h3>") {
			heading = plainText(block)
			continue
		}
		var rawURL, anchor, summary, bare string
		if link := importLink.FindStringSubmatch(block); link != nil {
			rawURL = html.UnescapeString(link[1])
			anchor = plainText(link[2])
			summary = plainText(importLink.ReplaceAllString(block, ""))
		} else {
			// Some editions cite the source as plain text, for example "Fuente: https://...".
			bare = importBareURL.FindString(plainText(block))
			if bare == "" {
				continue
			}
			rawURL = strings.TrimRight(bare, ".,;:)")
			summary = strings.Replace(plainText(block), bare, "", 1)
		}
		key, err := normalizeURL(rawURL)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		summary = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(summary, "Abrir enlace", ""), "›", ""))
		var title string
		if strings.HasPrefix(lower, "<li>") {
			if strong := importStrong.FindStringSubmatch(block); strong != nil {
				title = plainText(strong[1])
			}
		} else {
			title = heading
		}
		if title == "" {
			title = anchor
		}
		if title == "" {
			title = shortTitle(summary)
		}
		if summary == "" {
			summary = title
		}
		if title == "" || summary == "" {
			continue
		}
		stories = append(stories, Story{Title: title, Source: hostOf(rawURL), URL: rawURL, Published: date, Summary: summary})
	}
	return stories
}

// shortTitle keeps the first 100 runes of a summary, cut at a word boundary.
func shortTitle(summary string) string {
	runes := []rune(summary)
	if len(runes) <= 100 {
		return summary
	}
	cut := string(runes[:100])
	if i := strings.LastIndex(cut, " "); i > 40 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}

func plainText(fragment string) string {
	text := strings.ReplaceAll(fragment, "<br>", " ")
	text = html.UnescapeString(importStripTags.ReplaceAllString(text, ""))
	return strings.TrimSpace(importSpaces.ReplaceAllString(text, " "))
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "desconocida"
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
