package caddy

import (
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// BuildStreamServers generates the Caddy Layer4 server(s) for a stream host,
// keyed by server name. "tcp+udp" yields two servers — one per transport — so
// each dials its upstream on the matching network. A single proxy upstream dial
// cannot serve both TCP and UDP, so a combined listener with one dial would
// forward UDP traffic over TCP.
func BuildStreamServers(h *models.StreamHost) map[string]*L4Server {
	key := StreamServerKey(h)
	switch h.Protocol {
	case "udp":
		return map[string]*L4Server{key: streamServer(h, "udp/:", "udp/", false)}
	case "tcp+udp":
		return map[string]*L4Server{
			key:          streamServer(h, ":", "", true),
			key + "_udp": streamServer(h, "udp/:", "udp/", false),
		}
	default:
		return map[string]*L4Server{key: streamServer(h, ":", "", true)}
	}
}

// streamServer builds one transport-specific L4 server. listenPrefix and
// dialNetwork select the transport (e.g. "udp/:" and "udp/"); tcp reports
// whether an optional TLS SNI matcher applies (SNI is a TCP/TLS concept, so it
// is never attached to a UDP listener).
func streamServer(h *models.StreamHost, listenPrefix, dialNetwork string, tcp bool) *L4Server {
	route := &L4Route{
		Handle: []L4Handler{{
			Handler:   "proxy",
			Upstreams: []L4Upstream{{Dial: []string{fmt.Sprintf("%s%s:%d", dialNetwork, h.ForwardHost, h.ForwardPort)}}},
		}},
	}
	if tcp && h.TCPSNIMatch != nil && *h.TCPSNIMatch != "" {
		route.Match = []L4Matcher{{TLS: &L4TLSMatcher{SNI: []string{*h.TCPSNIMatch}}}}
	}
	return &L4Server{
		Listen: []string{fmt.Sprintf("%s%d", listenPrefix, h.IncomingPort)},
		Routes: []*L4Route{route},
	}
}

// StreamServerKey returns the key used in apps.layer4.servers for this stream host.
func StreamServerKey(h *models.StreamHost) string {
	return fmt.Sprintf("stream_%d", h.ID)
}
