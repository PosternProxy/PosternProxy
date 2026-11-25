package caddy

import (
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

// A UDP stream host must forward to its upstream over UDP, not TCP. Dialing the
// upstream as plain "host:port" would use TCP and silently break UDP forwarding.
func TestBuildStreamServerUDPDialsUDPUpstream(t *testing.T) {
	s := BuildStreamServer(&models.StreamHost{
		Protocol: "udp", IncomingPort: 5353, ForwardHost: "10.0.0.5", ForwardPort: 53,
	})
	if len(s.Listen) != 1 || s.Listen[0] != "udp/:5353" {
		t.Fatalf("udp listener = %v, want [udp/:5353]", s.Listen)
	}
	dial := s.Routes[0].Handle[0].Upstreams[0].Dial[0]
	if dial != "udp/10.0.0.5:53" {
		t.Fatalf("udp upstream dial = %q, want udp/10.0.0.5:53", dial)
	}
}

func TestBuildStreamServerTCPDialsTCPUpstream(t *testing.T) {
	s := BuildStreamServer(&models.StreamHost{
		Protocol: "tcp", IncomingPort: 9000, ForwardHost: "10.0.0.5", ForwardPort: 900,
	})
	if len(s.Listen) != 1 || s.Listen[0] != ":9000" {
		t.Fatalf("tcp listener = %v, want [:9000]", s.Listen)
	}
	dial := s.Routes[0].Handle[0].Upstreams[0].Dial[0]
	if dial != "10.0.0.5:900" {
		t.Fatalf("tcp upstream dial = %q, want 10.0.0.5:900", dial)
	}
}
