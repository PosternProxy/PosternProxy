package caddy

import (
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// BuildStreamServer generates a Caddy Layer4 server entry for a stream host.
func BuildStreamServer(h *models.StreamHost) *L4Server {
	var listen []string
	// dialNetwork prefixes the upstream address so the L4 proxy forwards on the
	// right transport. Without it a UDP listener would dial the upstream over TCP.
	dialNetwork := ""
	switch h.Protocol {
	case "udp":
		listen = []string{fmt.Sprintf("udp/:%d", h.IncomingPort)}
		dialNetwork = "udp/"
	case "tcp+udp":
		listen = []string{
			fmt.Sprintf(":%d", h.IncomingPort),
			fmt.Sprintf("udp/:%d", h.IncomingPort),
		}
	default:
		listen = []string{fmt.Sprintf(":%d", h.IncomingPort)}
	}

	upstream := L4Upstream{
		Dial: []string{fmt.Sprintf("%s%s:%d", dialNetwork, h.ForwardHost, h.ForwardPort)},
	}

	proxyHandler := L4Handler{
		Handler:   "proxy",
		Upstreams: []L4Upstream{upstream},
	}

	route := &L4Route{
		Handle: []L4Handler{proxyHandler},
	}

	// Add SNI matcher for TLS-based routing
	if h.TCPSNIMatch != nil && *h.TCPSNIMatch != "" {
		route.Match = []L4Matcher{{
			TLS: &L4TLSMatcher{SNI: []string{*h.TCPSNIMatch}},
		}}
	}

	return &L4Server{
		Listen: listen,
		Routes: []*L4Route{route},
	}
}

// StreamServerKey returns the key used in apps.layer4.servers for this stream host.
func StreamServerKey(h *models.StreamHost) string {
	return fmt.Sprintf("stream_%d", h.ID)
}
