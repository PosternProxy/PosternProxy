// Package nginx provides a best-effort parser for simple nginx configurations.
// It handles the common patterns used in reverse-proxy setups (server_name,
// proxy_pass, return 3xx) and skips or warns on constructs it cannot model.
package nginx

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ParsedProxyHost represents a proxy_pass server block.
type ParsedProxyHost struct {
	DomainNames   []string `json:"domain_names"`
	ForwardScheme string   `json:"forward_scheme"`
	ForwardHost   string   `json:"forward_host"`
	ForwardPort   int      `json:"forward_port"`
	SSLForced     bool     `json:"ssl_forced"`
}

// ParsedRedirectionHost represents a server block with a return 301/302.
type ParsedRedirectionHost struct {
	DomainNames   []string `json:"domain_names"`
	ForwardScheme string   `json:"forward_scheme"`
	ForwardDomain string   `json:"forward_domain"`
	ForwardPath   string   `json:"forward_path"`
	StatusCode    int      `json:"status_code"`
}

// ParseResult holds all successfully parsed entities and any warnings.
type ParseResult struct {
	ProxyHosts       []ParsedProxyHost       `json:"proxy_hosts"`
	RedirectionHosts []ParsedRedirectionHost `json:"redirection_hosts"`
	Warnings         []string                `json:"warnings"`
}

var (
	reServerName = regexp.MustCompile(`(?i)\bserver_name\s+([^;]+);`)
	reProxyPass  = regexp.MustCompile(`(?i)\bproxy_pass\s+([^\s;]+)\s*;`)
	reReturn     = regexp.MustCompile(`(?i)\breturn\s+(\d{3})\s+([^;]+);`)
	reListenSSL  = regexp.MustCompile(`(?i)\blisten\s+[^\n]*443`)
	reComment    = regexp.MustCompile(`#[^\n]*`)
)

// Parse extracts proxy hosts and redirection hosts from nginx config content.
func Parse(content []byte) (*ParseResult, error) {
	text := reComment.ReplaceAllString(string(content), "")
	result := &ParseResult{}

	blocks := extractServerBlocks(text)
	if len(blocks) == 0 {
		result.Warnings = append(result.Warnings, "No server { } blocks found in the config file.")
		return result, nil
	}

	for i, block := range blocks {
		label := fmt.Sprintf("server block #%d", i+1)
		warnings := parseBlock(block, label, result)
		result.Warnings = append(result.Warnings, warnings...)
	}

	return result, nil
}

// parseBlock processes one server { } block's interior and appends to result.
func parseBlock(block, label string, result *ParseResult) []string {
	var warnings []string

	// Extract server_name
	domains := extractDomains(block)
	if len(domains) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: no server_name found, skipping", label))
		return warnings
	}
	// Skip catch-all
	filtered := make([]string, 0, len(domains))
	for _, d := range domains {
		if d == "_" || d == "localhost" {
			warnings = append(warnings, fmt.Sprintf("%s: skipping catch-all/localhost server_name %q", label, d))
			continue
		}
		filtered = append(filtered, d)
	}
	if len(filtered) == 0 {
		return warnings
	}
	domains = filtered

	isSSL := reListenSSL.MatchString(block)

	// Check for proxy_pass
	if m := reProxyPass.FindStringSubmatch(block); m != nil {
		rawURL := strings.TrimSpace(m[1])
		scheme, host, port, err := parseUpstream(rawURL)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: could not parse proxy_pass %q: %v", label, rawURL, err))
			return warnings
		}
		result.ProxyHosts = append(result.ProxyHosts, ParsedProxyHost{
			DomainNames:   domains,
			ForwardScheme: scheme,
			ForwardHost:   host,
			ForwardPort:   port,
			SSLForced:     isSSL,
		})
		return warnings
	}

	// Check for return redirect
	if m := reReturn.FindStringSubmatch(block); m != nil {
		code, _ := strconv.Atoi(m[1])
		if code == 301 || code == 302 || code == 307 || code == 308 {
			target := strings.TrimSpace(m[2])
			scheme, domain, path := parseRedirectTarget(target)
			if domain == "" {
				warnings = append(warnings, fmt.Sprintf("%s: could not parse return target %q, skipping", label, target))
				return warnings
			}
			result.RedirectionHosts = append(result.RedirectionHosts, ParsedRedirectionHost{
				DomainNames:   domains,
				ForwardScheme: scheme,
				ForwardDomain: domain,
				ForwardPath:   path,
				StatusCode:    code,
			})
			return warnings
		}
	}

	warnings = append(warnings, fmt.Sprintf("%s: no proxy_pass or redirect found for domains %v, skipping", label, domains))
	return warnings
}

// extractServerBlocks returns the interior of each top-level server { } block.
func extractServerBlocks(text string) []string {
	var blocks []string
	for {
		idx := strings.Index(text, "server")
		if idx < 0 {
			break
		}
		// Skip 'server_name' and similar; look for 'server {' pattern
		rest := strings.TrimSpace(text[idx+len("server"):])
		if !strings.HasPrefix(rest, "{") {
			// It's server_name or other directive; advance past this token
			text = text[idx+1:]
			continue
		}
		// Find matching closing brace
		braceStart := strings.Index(text[idx:], "{") + idx
		end := findMatchingBrace(text, braceStart)
		if end < 0 {
			break
		}
		blocks = append(blocks, text[braceStart+1:end])
		text = text[end+1:]
	}
	return blocks
}

// findMatchingBrace returns the index of the closing brace matching the open
// brace at openIdx. Returns -1 if not found.
func findMatchingBrace(text string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractDomains parses space-separated domains from server_name directives.
func extractDomains(block string) []string {
	var domains []string
	for _, m := range reServerName.FindAllStringSubmatch(block, -1) {
		for _, d := range strings.Fields(m[1]) {
			d = strings.TrimRight(d, ";")
			if d != "" {
				domains = append(domains, d)
			}
		}
	}
	return deduplicate(domains)
}

// parseUpstream extracts scheme, host, and port from a proxy_pass URL.
func parseUpstream(rawURL string) (scheme, host string, port int, err error) {
	// Ensure it has a scheme for url.Parse
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	u, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", 0, parseErr
	}

	scheme = u.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}

	hostname := u.Hostname()
	portStr := u.Port()

	if portStr == "" {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid port %q", portStr)
		}
	}

	host = hostname
	return
}

// parseRedirectTarget extracts scheme, domain, and path from a redirect URL.
func parseRedirectTarget(target string) (scheme, domain, path string) {
	// Handle nginx variable placeholders like https://$host/path
	// Replace $host/$server_name with a placeholder
	target = strings.ReplaceAll(target, "$host", "PLACEHOLDER")
	target = strings.ReplaceAll(target, "$server_name", "PLACEHOLDER")

	if !strings.Contains(target, "://") {
		target = "https://" + target
	}

	u, err := url.Parse(target)
	if err != nil {
		return "", "", ""
	}

	scheme = u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	domain = u.Hostname()
	if domain == "PLACEHOLDER" {
		domain = "" // caller should use source domain
	}
	path = u.Path

	return
}

func deduplicate(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
