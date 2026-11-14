// Copyright (c) 2026 Nicolás Correa
// SPDX-License-Identifier: MIT

package pipeline

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLKeepsAllowlistedMarkup(t *testing.T) {
	input := `<h2>Hoy</h2><p><strong>Dato</strong> con <a href="https://example.com/nota">enlace</a>.<br></p><p><img src="https://example.com/img.jpg" alt="Portada" width="160"></p><ul><li>Uno</li></ul>`
	got, err := sanitizeHTML(input)
	if err != nil {
		t.Fatalf("sanitizeHTML() error = %v", err)
	}
	if got != input {
		t.Fatalf("sanitizeHTML() = %q, want unchanged %q", got, input)
	}
}

func TestSanitizeHTMLKeepsTables(t *testing.T) {
	input := `<table><thead><tr><th>Indicador</th><th>Valor</th></tr></thead><tbody><tr><td>IPC</td><td>2,1 % (INDEC)</td></tr></tbody></table>`
	got, err := sanitizeHTML(input)
	if err != nil {
		t.Fatalf("sanitizeHTML() error = %v", err)
	}
	if got != input {
		t.Fatalf("sanitizeHTML() = %q, want unchanged %q", got, input)
	}
}

func TestSanitizeHTMLDropsUnknownAttributes(t *testing.T) {
	got, err := sanitizeHTML(`<a href="https://example.com/x" onclick="steal()" style="color:red">x</a>`)
	if err != nil {
		t.Fatalf("sanitizeHTML() error = %v", err)
	}
	if strings.Contains(got, "onclick") || strings.Contains(got, "style") {
		t.Fatalf("unsafe attributes survived: %q", got)
	}
}

func TestSanitizeHTMLRejectsUnsafeInput(t *testing.T) {
	cases := map[string]string{
		"script tag":         `<p>hola</p><script>alert(1)</script>`,
		"iframe tag":         `<iframe src="https://example.com"></iframe>`,
		"javascript href":    `<a href="javascript:alert(1)">x</a>`,
		"relative image":     `<img src="/local.png">`,
		"non numeric width":  `<img src="https://example.com/i.png" width="100%">`,
		"stray less-than":    `<p>5 < 6</p>`,
		"html comment":       `<p>x</p><!-- hidden -->`,
		"closing void tag":   `<br></br>`,
		"unquoted attribute": `<a href=https://example.com/x>x</a>`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitizeHTML(input); err == nil {
				t.Fatalf("sanitizeHTML(%q) accepted unsafe input", input)
			}
		})
	}
}
