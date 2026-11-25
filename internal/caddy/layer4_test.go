package caddy

import (
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func dialOf(t *testing.T, s *L4Server) string {
	t.Helper()
	if s == nil || len(s.Routes) == 0 || len(s.Routes[0].Handle) == 0 ||
		len(s.Routes[0].Handle[0].Upstreams) == 0 || len(s.Routes[0].Handle[0].Upstreams[0].Dial) == 0 {
		t.Fatalf("server has no upstream dial: %+v", s)
	}
	return s.Routes[0].Handle[0].Upstreams[0].Dial[0]
}

// A UDP stream host must forward to its upstream over UDP, not TCP. Dialing the
// upstream as plain "host:port" would use TCP and silently break UDP forwarding.
func TestBuildStreamServersUDPDialsUDPUpstream(t *testing.T) {
	servers := BuildStreamServers(&models.StreamHost{
		ID: 3, Protocol: "udp", IncomingPort: 5353, ForwardHost: "10.0.0.5", ForwardPort: 53,
	})
	s := servers["stream_3"]
	if s == nil || len(s.Listen) != 1 || s.Listen[0] != "udp/:5353" {
		t.Fatalf("udp listener = %v, want [udp/:5353]", s)
	}
	if dial := dialOf(t, s); dial != "udp/10.0.0.5:53" {
		t.Fatalf("udp upstream dial = %q, want udp/10.0.0.5:53", dial)
	}
}

func TestBuildStreamServersTCPDialsTCPUpstream(t *testing.T) {
	servers := BuildStreamServers(&models.StreamHost{
		ID: 4, Protocol: "tcp", IncomingPort: 9000, ForwardHost: "10.0.0.5", ForwardPort: 900,
	})
	s := servers["stream_4"]
	if s == nil || len(s.Listen) != 1 || s.Listen[0] != ":9000" {
		t.Fatalf("tcp listener = %v, want [:9000]", s)
	}
	if dial := dialOf(t, s); dial != "10.0.0.5:900" {
		t.Fatalf("tcp upstream dial = %q, want 10.0.0.5:900", dial)
	}
}

// Combined mode must produce one server per transport, each dialing its own
// network, so UDP traffic is not forwarded over a TCP dial.
func TestBuildStreamServersCombinedSplitsPerTransport(t *testing.T) {
	servers := BuildStreamServers(&models.StreamHost{
		ID: 5, Protocol: "tcp+udp", IncomingPort: 7000, ForwardHost: "10.0.0.9", ForwardPort: 700,
	})
	tcp := servers["stream_5"]
	udp := servers["stream_5_udp"]
	if tcp == nil || udp == nil {
		t.Fatalf("combined mode must yield both tcp and udp servers, got %v", servers)
	}
	if tcp.Listen[0] != ":7000" || dialOf(t, tcp) != "10.0.0.9:700" {
		t.Fatalf("tcp server wrong: listen=%v dial=%q", tcp.Listen, dialOf(t, tcp))
	}
	if udp.Listen[0] != "udp/:7000" || dialOf(t, udp) != "udp/10.0.0.9:700" {
		t.Fatalf("udp server wrong: listen=%v dial=%q", udp.Listen, dialOf(t, udp))
	}
}
