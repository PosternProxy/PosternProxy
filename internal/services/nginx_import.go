package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/nginx"
)

// ImportSummary reports the results of a confirmed nginx import.
type ImportSummary struct {
	ProxyHostsCreated       int      `json:"proxy_hosts_created"`
	RedirectionHostsCreated int      `json:"redirection_hosts_created"`
	Errors                  []string `json:"errors,omitempty"`
}

// NginxImportService converts parsed nginx configs into PosternProxy entities.
type NginxImportService struct {
	proxyHosts       *ProxyHostService
	redirectionHosts *RedirectionHostService
}

func NewNginxImportService(ph *ProxyHostService, rh *RedirectionHostService) *NginxImportService {
	return &NginxImportService{proxyHosts: ph, redirectionHosts: rh}
}

// Preview parses nginx config content and returns a preview without creating anything.
func (s *NginxImportService) Preview(content []byte) (*nginx.ParseResult, error) {
	return nginx.Parse(content)
}

// Confirm creates PosternProxy entities from a ParseResult the user has approved.
func (s *NginxImportService) Confirm(ctx context.Context, callerID *int64, result *nginx.ParseResult, ip string) (*ImportSummary, error) {
	summary := &ImportSummary{}

	for _, ph := range result.ProxyHosts {
		dn, _ := json.Marshal(ph.DomainNames)
		input := models.CreateProxyHostInput{
			DomainNames:     json.RawMessage(dn),
			ForwardScheme:   ph.ForwardScheme,
			ForwardHost:     ph.ForwardHost,
			ForwardPort:     ph.ForwardPort,
			SSLForced:       ph.SSLForced,
			HTTP2Support:    true,
			Locations:       json.RawMessage("[]"),
			RateLimitWindow: "1m",
		}
		if _, err := s.proxyHosts.Create(ctx, callerID, input, ip); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("proxy %v: %v", ph.DomainNames, err))
		} else {
			summary.ProxyHostsCreated++
		}
	}

	for _, rh := range result.RedirectionHosts {
		dn, _ := json.Marshal(rh.DomainNames)
		statusCode := rh.StatusCode
		if statusCode == 0 {
			statusCode = 301
		}
		input := CreateRedirectionHostInput{
			DomainNames:       json.RawMessage(dn),
			ForwardScheme:     rh.ForwardScheme,
			ForwardDomainName: rh.ForwardDomain,
			ForwardPath:       rh.ForwardPath,
			StatusCode:        statusCode,
		}
		if _, err := s.redirectionHosts.Create(ctx, callerID, input, ip); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("redirect %v: %v", rh.DomainNames, err))
		} else {
			summary.RedirectionHostsCreated++
		}
	}

	return summary, nil
}
