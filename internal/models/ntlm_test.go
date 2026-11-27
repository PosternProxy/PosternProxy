package models

import (
	"encoding/json"
	"testing"
)

func TestNTLMErrorSentences(t *testing.T) {
	sentences := map[string]string{
		"extra":     "NTLM passthrough requires a single upstream; remove extra upstreams or disable it",
		"locations": "NTLM passthrough cannot be combined with path locations",
		"tls":       "NTLM passthrough cannot be combined with TLS passthrough",
		"advanced":  "NTLM passthrough cannot be combined with advanced route JSON",
		"headers":   "NTLM passthrough cannot change the Authorization or WWW-Authenticate headers",
		"page":      "NTLM passthrough cannot use a custom 401 error page",
		"json":      "NTLM passthrough settings must be valid JSON",
	}
	if NTLMErrExtraUpstreams != sentences["extra"] || NTLMErrLocations != sentences["locations"] ||
		NTLMErrTLSPassthrough != sentences["tls"] || NTLMErrAdvanced != sentences["advanced"] ||
		NTLMErrHeaders != sentences["headers"] || NTLMErrErrorPage != sentences["page"] ||
		NTLMErrJSON != sentences["json"] {
		t.Fatalf("sentence drift: %+v", sentences)
	}
}

func TestValidateNTLMPassthrough(t *testing.T) {
	headers := NTLMErrHeaders
	pages := NTLMErrErrorPage
	jsonErr := NTLMErrJSON
	tests := []struct {
		name string
		in   NTLMPassthrough
		want string
	}{
		{name: "flag off ignores conflicts", in: NTLMPassthrough{TLSPassthrough: true, Locations: json.RawMessage(`[{"path":"/"}]`), ExtraUpstreams: json.RawMessage(`[{}]`)}},
		{name: "omitted containers", in: NTLMPassthrough{Enabled: true}},
		{name: "empty containers", in: NTLMPassthrough{Enabled: true, Locations: json.RawMessage(`[]`), ExtraUpstreams: json.RawMessage(`[]`), CustomHeaders: json.RawMessage(`[]`), ErrorPages: json.RawMessage(`{}`)}},
		{name: "blank advanced", in: NTLMPassthrough{Enabled: true, AdvancedConfig: ""}},
		{name: "empty advanced object", in: NTLMPassthrough{Enabled: true, AdvancedConfig: "{}"}},
		{name: "whitespace advanced", in: NTLMPassthrough{Enabled: true, AdvancedConfig: "   "}},
		{name: "padded empty advanced object", in: NTLMPassthrough{Enabled: true, AdvancedConfig: " {} "}},
		{name: "spaced braces rejected", in: NTLMPassthrough{Enabled: true, AdvancedConfig: " { } "}, want: NTLMErrAdvanced},
		{name: "advanced object rejected", in: NTLMPassthrough{Enabled: true, AdvancedConfig: `{"match":[]}`}, want: NTLMErrAdvanced},
		{name: "extra object", in: NTLMPassthrough{Enabled: true, ExtraUpstreams: json.RawMessage(`[{}]`)}, want: NTLMErrExtraUpstreams},
		{name: "extra wrong type", in: NTLMPassthrough{Enabled: true, ExtraUpstreams: json.RawMessage(`{}`)}, want: NTLMErrExtraUpstreams},
		{name: "extra string", in: NTLMPassthrough{Enabled: true, ExtraUpstreams: json.RawMessage(`"foo"`)}, want: NTLMErrExtraUpstreams},
		{name: "extra null", in: NTLMPassthrough{Enabled: true, ExtraUpstreams: json.RawMessage(`null`)}, want: NTLMErrExtraUpstreams},
		{name: "extra malformed", in: NTLMPassthrough{Enabled: true, ExtraUpstreams: json.RawMessage(`{`)}, want: jsonErr},
		{name: "locations object", in: NTLMPassthrough{Enabled: true, Locations: json.RawMessage(`[{"path":"/"}]`)}, want: NTLMErrLocations},
		{name: "locations wrong type", in: NTLMPassthrough{Enabled: true, Locations: json.RawMessage(`{}`)}, want: NTLMErrLocations},
		{name: "locations null", in: NTLMPassthrough{Enabled: true, Locations: json.RawMessage(`null`)}, want: NTLMErrLocations},
		{name: "locations malformed", in: NTLMPassthrough{Enabled: true, Locations: json.RawMessage(`[`)}, want: jsonErr},
		{name: "tls passthrough", in: NTLMPassthrough{Enabled: true, TLSPassthrough: true}, want: NTLMErrTLSPassthrough},
		{name: "request delete authorization", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "Authorization")}, want: headers},
		{name: "request delete lower authorization", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "authorization")}, want: headers},
		{name: "request delete star", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "*")}, want: headers},
		{name: "request delete auth prefix", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "Auth*")}, want: headers},
		{name: "request delete authorization suffix", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "*ization")}, want: headers},
		{name: "request delete auth substring", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "*auth*")}, want: headers},
		{name: "response delete www-authenticate", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("response", "delete", "WWW-Authenticate")}, want: headers},
		{name: "response delete lower www-authenticate", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("response", "delete", "www-authenticate")}, want: headers},
		{name: "response delete www prefix", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("response", "delete", "www-*")}, want: headers},
		{name: "response delete authenticate suffix", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("response", "delete", "*authenticate")}, want: headers},
		{name: "request set authorization", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "set", "Authorization")}, want: headers},
		{name: "response add www-authenticate", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("response", "add", "www-authenticate")}, want: headers},
		{name: "placeholder name", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "set", "{http.request.header.Authorization}")}, want: headers},
		{name: "colon name", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "X-Request:Id")}, want: headers},
		{name: "request set star", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "set", "*")}},
		{name: "request delete other", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "X-Request-Id")}},
		{name: "empty header name", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "")}},
		{name: "middle star is exact", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("request", "delete", "a*b")}},
		{name: "wrong direction case", in: NTLMPassthrough{Enabled: true, CustomHeaders: headerRules("Request", "delete", "Authorization")}},
		{name: "headers null", in: NTLMPassthrough{Enabled: true, CustomHeaders: json.RawMessage(`null`)}, want: jsonErr},
		{name: "headers object", in: NTLMPassthrough{Enabled: true, CustomHeaders: json.RawMessage(`{}`)}, want: jsonErr},
		{name: "headers malformed", in: NTLMPassthrough{Enabled: true, CustomHeaders: json.RawMessage(`[`)}, want: jsonErr},
		{name: "headers non-object item", in: NTLMPassthrough{Enabled: true, CustomHeaders: json.RawMessage(`[1]`)}, want: jsonErr},
		{name: "error page 401", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"401":"<p>no</p>"}`)}, want: pages},
		{name: "error page 0401", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"0401":"<p>no</p>"}`)}, want: pages},
		{name: "error page 401abc", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"401abc":"<p>no</p>"}`)}, want: pages},
		{name: "error page +401", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"+401":"<p>no</p>"}`)}, want: pages},
		{name: "error page 401 empty body", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"401":""}`)}},
		{name: "error page 502", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"502":"<p>down</p>"}`)}},
		{name: "error pages array", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`[]`)}, want: pages},
		{name: "error pages null", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`null`)}, want: pages},
		{name: "error pages malformed", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{`)}, want: jsonErr},
		{name: "error page non-string", in: NTLMPassthrough{Enabled: true, ErrorPages: json.RawMessage(`{"401":1}`)}, want: jsonErr},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNTLMPassthrough(tc.in)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func headerRules(direction, operation, name string) json.RawMessage {
	raw, err := json.Marshal([]CustomHeaderRule{{Direction: direction, Operation: operation, Name: name}})
	if err != nil {
		panic(err)
	}
	return raw
}
