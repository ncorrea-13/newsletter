package pipeline

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// The agent's content_html is untrusted. Only tags and attributes from this
// allowlist survive; any other tag, or a stray "<", rejects the whole edition.
var (
	htmlTagPattern  = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)((?:\s+[a-zA-Z-]+="[^"<>]*")*)\s*(/?)>`)
	htmlAttrPattern = regexp.MustCompile(`\s+([a-zA-Z-]+)="([^"<>]*)"`)
	htmlDigits      = regexp.MustCompile(`^[0-9]{1,4}$`)
)

type htmlTagRule struct {
	attrs []string
	void  bool
}

var allowedHTMLTags = map[string]htmlTagRule{
	"p":      {},
	"h1":     {},
	"h2":     {},
	"h3":     {},
	"ul":     {},
	"li":     {},
	"strong": {},
	"em":     {},
	"br":     {void: true},
	"a":      {attrs: []string{"href"}},
	"img":    {attrs: []string{"src", "alt", "width"}, void: true},
}

// sanitizeHTML returns the allowlisted HTML with attributes rebuilt, or an error.
func sanitizeHTML(input string) (string, error) {
	var out strings.Builder
	last := 0
	for _, loc := range htmlTagPattern.FindAllStringSubmatchIndex(input, -1) {
		text := input[last:loc[0]]
		if strings.Contains(text, "<") {
			return "", fmt.Errorf("content_html has a stray '<'")
		}
		out.WriteString(text)
		last = loc[1]

		closing := input[loc[2]:loc[3]] == "/"
		name := strings.ToLower(input[loc[4]:loc[5]])
		attrText := input[loc[6]:loc[7]]
		rule, ok := allowedHTMLTags[name]
		if !ok {
			return "", fmt.Errorf("content_html tag %q is not allowed", name)
		}
		if closing {
			if rule.void {
				return "", fmt.Errorf("content_html closing tag %q is not allowed", name)
			}
			out.WriteString("</" + name + ">")
			continue
		}
		var attrs []string
		for _, match := range htmlAttrPattern.FindAllStringSubmatch(attrText, -1) {
			attr, value := strings.ToLower(match[1]), match[2]
			if !contains(rule.attrs, attr) {
				continue // unknown attributes (onclick, style, ...) are dropped
			}
			switch attr {
			case "href", "src":
				if err := requireAbsoluteHTTP(value); err != nil {
					return "", err
				}
			case "width":
				if !htmlDigits.MatchString(value) {
					return "", fmt.Errorf("content_html width %q is not numeric", value)
				}
			}
			attrs = append(attrs, attr+`="`+value+`"`)
		}
		tag := "<" + name
		if len(attrs) > 0 {
			tag += " " + strings.Join(attrs, " ")
		}
		out.WriteString(tag + ">")
	}
	tail := input[last:]
	if strings.Contains(tail, "<") {
		return "", fmt.Errorf("content_html has a stray '<'")
	}
	out.WriteString(tail)
	return strings.TrimSpace(out.String()), nil
}

func requireAbsoluteHTTP(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("content_html URL %q must be absolute http(s)", raw)
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
