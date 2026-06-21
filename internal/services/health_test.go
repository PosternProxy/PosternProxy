package services

import "testing"

func TestUpstreamStatus(t *testing.T) {
	tests := []struct {
		name        string
		numRequests int
		fails       int
		want        string
	}{
		{"recorded failures are unhealthy", 10, 2, UpstreamUnhealthy},
		{"failures dominate even with traffic", 0, 1, UpstreamUnhealthy},
		{"traffic with no failures is healthy", 5, 0, UpstreamHealthy},
		{"no traffic and no failures is unknown, not healthy", 0, 0, UpstreamUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := upstreamStatus(tt.numRequests, tt.fails); got != tt.want {
				t.Errorf("upstreamStatus(%d, %d) = %q, want %q", tt.numRequests, tt.fails, got, tt.want)
			}
		})
	}
}
