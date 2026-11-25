package caddy

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image/png"
	"io"
	"regexp"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestDefaultPagesEmbedThemeLogos(t *testing.T) {
	tests := []struct {
		name string
		body string
		copy string
	}{
		{"proxy-502", defaultProxyErrorPage(502, "v1.2.3"), "Bad Gateway"},
		{"proxy-503", defaultProxyErrorPage(503, "v1.2.3"), "Service Unavailable"},
		{"parked-200", defaultParkedPage(200, "v1.2.3"), "This domain is parked"},
		{"parked-404", defaultParkedPage(404, "v1.2.3"), "Page not available"},
		{"parked-410", defaultParkedPage(410, "v1.2.3"), "This domain is gone"},
		{"parked-503", defaultParkedPage(503, "v1.2.3"), "Temporarily unavailable"},
		{"parked-fallback", defaultParkedPage(451, "v1.2.3"), "This domain is not available"},
	}
	darkMedia := regexp.MustCompile(`@media[^{]*\(\s*prefers-color-scheme\s*:\s*dark\s*\)`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range []string{tt.copy, "v1.2.3", "{http.request.host}"} {
				if !strings.Contains(tt.body, want) {
					t.Errorf("page missing %q", want)
				}
			}
			if !darkMedia.MatchString(tt.body) {
				t.Error("page does not automatically follow the system dark preference")
			}
			assertEmbeddedPageLogos(t, tt.body)
		})
	}
}

func assertEmbeddedPageLogos(t *testing.T, body string) {
	t.Helper()
	tokenizer := nethtml.NewTokenizer(strings.NewReader(body))
	variants := make(map[string]bool)
	accessible := false
	for {
		tokenType := tokenizer.Next()
		if tokenType == nethtml.ErrorToken {
			if err := tokenizer.Err(); err != io.EOF {
				t.Fatalf("read page HTML: %v", err)
			}
			break
		}
		if tokenType != nethtml.StartTagToken && tokenType != nethtml.SelfClosingTagToken {
			continue
		}
		tag := tokenizer.Token()
		if tag.Data != "img" && tag.Data != "source" && tag.Data != "script" && tag.Data != "link" {
			continue
		}
		for _, attr := range tag.Attr {
			if tag.Data == "img" && attr.Key == "alt" && strings.Contains(attr.Val, "PosternProxy") {
				accessible = true
			}
			if attr.Key != "src" && attr.Key != "srcset" && attr.Key != "href" {
				continue
			}
			if attr.Val == "" {
				continue
			}
			const prefix = "data:image/png;base64,"
			if !strings.HasPrefix(attr.Val, prefix) {
				t.Errorf("%s %s requires a separate request: %.100q", tag.Data, attr.Key, attr.Val)
				continue
			}
			if tag.Data != "img" && tag.Data != "source" {
				continue
			}
			encoded := strings.Fields(strings.TrimPrefix(attr.Val, prefix))[0]
			if variants[encoded] {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Errorf("decode embedded logo: %v", err)
				continue
			}
			logo, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Errorf("embedded logo is not a valid PNG: %v", err)
				continue
			}
			if logo.Bounds().Dx() < 64 || logo.Bounds().Dy() < 64 {
				t.Error("embedded logo is too small for the page header")
			}
			variants[encoded] = true
		}
	}
	if len(variants) < 2 {
		t.Errorf("page has %d distinct embedded PNG logos; need light and dark variants", len(variants))
	}
	if !accessible {
		t.Error("logo has no accessible PosternProxy image description")
	}
}

func TestDefaultPageBrandingPreservesEscapingAndPlaceholders(t *testing.T) {
	version := `v1<script>alert("version")</script>&`
	for _, render := range []struct {
		name string
		page func(int, string) string
	}{
		{"proxy", defaultProxyErrorPage},
		{"parked", defaultParkedPage},
	} {
		t.Run(render.name, func(t *testing.T) {
			body := render.page(503, version)
			if strings.Contains(body, version) || !strings.Contains(body, html.EscapeString(version)) {
				t.Error("version is not HTML-escaped")
			}
			if !strings.Contains(body, "{http.request.host}") {
				t.Error("lost Caddy host placeholder")
			}
			if render.name == "proxy" && !strings.Contains(body, "{http.request.uuid}") {
				t.Error("lost Caddy request ID placeholder")
			}
			if !strings.Contains(render.page(503, " "), "unknown") {
				t.Error("blank version no longer falls back to unknown")
			}
			if strings.Contains(body, "%!") {
				t.Error("page contains a formatting error")
			}
			if render.name == "parked" {
				for _, status := range []int{404, 410, 503, 451} {
					if !strings.Contains(render.page(status, "v1"), fmt.Sprintf(`class="code">%d`, status)) {
						t.Errorf("parked page lost status %d", status)
					}
				}
			}
		})
	}
}
