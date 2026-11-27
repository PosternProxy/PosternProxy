package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	NTLMErrExtraUpstreams = "NTLM passthrough requires a single upstream; remove extra upstreams or disable it"
	NTLMErrLocations      = "NTLM passthrough cannot be combined with path locations"
	NTLMErrTLSPassthrough = "NTLM passthrough cannot be combined with TLS passthrough"
	NTLMErrAdvanced       = "NTLM passthrough cannot be combined with advanced route JSON"
	NTLMErrHeaders        = "NTLM passthrough cannot change the Authorization or WWW-Authenticate headers"
	NTLMErrErrorPage      = "NTLM passthrough cannot use a custom 401 error page"
	NTLMErrJSON           = "NTLM passthrough settings must be valid JSON"
)

// NTLMPassthrough is the normalized slice of a host that the passthrough
// checks read. Empty JSON containers are allowed.
type NTLMPassthrough struct {
	Enabled        bool
	TLSPassthrough bool
	AdvancedConfig string
	Locations      json.RawMessage
	ExtraUpstreams json.RawMessage
	CustomHeaders  json.RawMessage
	ErrorPages     json.RawMessage
}

var errNTLMShape = errors.New("ntlm json shape")

// ValidateNTLMPassthrough returns nil when the flag is off. When it is on,
// conflicting or malformed settings are rejected and the caller must not
// write the row.
func ValidateNTLMPassthrough(in NTLMPassthrough) error {
	if !in.Enabled {
		return nil
	}
	if err := rejectNTLMArray(in.ExtraUpstreams, NTLMErrExtraUpstreams); err != nil {
		return err
	}
	if err := rejectNTLMArray(in.Locations, NTLMErrLocations); err != nil {
		return err
	}
	if in.TLSPassthrough {
		return errors.New(NTLMErrTLSPassthrough)
	}
	switch strings.TrimSpace(in.AdvancedConfig) {
	case "", "{}":
	default:
		return errors.New(NTLMErrAdvanced)
	}
	if err := rejectNTLMHeaders(in.CustomHeaders); err != nil {
		return err
	}
	return rejectNTLMErrorPages(in.ErrorPages)
}

func rejectNTLMArray(raw json.RawMessage, semantic string) error {
	n, err := jsonArrayLen(raw)
	if errors.Is(err, errNTLMShape) || (err == nil && n > 0) {
		return errors.New(semantic)
	}
	return err
}

func jsonArrayLen(raw json.RawMessage) (int, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return 0, nil
	}
	if !json.Valid(trimmed) {
		return 0, errors.New(NTLMErrJSON)
	}
	if trimmed[0] != '[' {
		return 0, errNTLMShape
	}
	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return 0, errors.New(NTLMErrJSON)
	}
	return len(items), nil
}

func rejectNTLMHeaders(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if !json.Valid(trimmed) || trimmed[0] != '[' {
		return errors.New(NTLMErrJSON)
	}
	var rules []CustomHeaderRule
	if err := json.Unmarshal(trimmed, &rules); err != nil {
		return errors.New(NTLMErrJSON)
	}
	for _, rule := range rules {
		if ntlmHeaderForbidden(rule) {
			return errors.New(NTLMErrHeaders)
		}
	}
	return nil
}

func ntlmHeaderForbidden(rule CustomHeaderRule) bool {
	name := rule.Name
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, "{}\r\n:") {
		return true
	}
	switch rule.Operation {
	case "set", "add":
		canon := http.CanonicalHeaderKey(name)
		return (rule.Direction == "request" && canon == "Authorization") ||
			(rule.Direction == "response" && canon == "Www-Authenticate")
	case "delete":
		return ntlmDeleteMatches(rule.Direction, name)
	default:
		return false
	}
}

// ntlmDeleteMatches follows Caddy's header delete switch: "*" deletes every
// field, a leading "*" is a suffix, a trailing "*" is a prefix, both are a
// substring, and any other name is an exact canonical delete.
func ntlmDeleteMatches(direction, name string) bool {
	var protected string
	switch direction {
	case "request":
		protected = "authorization"
	case "response":
		protected = "www-authenticate"
	default:
		return false
	}
	field := strings.ToLower(name)
	switch {
	case field == "*":
		return true
	case strings.HasPrefix(field, "*") && strings.HasSuffix(field, "*") && len(field) >= 2:
		return strings.Contains(protected, field[1:len(field)-1])
	case strings.HasPrefix(field, "*"):
		return strings.HasSuffix(protected, field[1:])
	case strings.HasSuffix(field, "*"):
		return strings.HasPrefix(protected, strings.TrimSuffix(field, "*"))
	default:
		return http.CanonicalHeaderKey(field) == http.CanonicalHeaderKey(protected)
	}
}

func rejectNTLMErrorPages(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if !json.Valid(trimmed) {
		return errors.New(NTLMErrJSON)
	}
	if trimmed[0] != '{' {
		return errors.New(NTLMErrErrorPage)
	}
	var pages map[string]string
	if err := json.Unmarshal(trimmed, &pages); err != nil {
		return errors.New(NTLMErrJSON)
	}
	for code, body := range pages {
		if body == "" {
			continue
		}
		status := 0
		if _, err := fmt.Sscanf(code, "%d", &status); err != nil {
			continue
		}
		if status == 401 {
			return errors.New(NTLMErrErrorPage)
		}
	}
	return nil
}
