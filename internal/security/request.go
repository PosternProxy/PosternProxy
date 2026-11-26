package security

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type peerKey struct{}
type clientKey struct{}

func RequestIP(ctx context.Context) string { ip, _ := ctx.Value(clientKey{}).(string); return ip }

func ParseTrustedProxies(values []string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, value := range values {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("trusted proxy must be an IP CIDR: %w", err)
		}
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("use an IPv4 CIDR instead of an IPv4-mapped IPv6 CIDR")
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func ClientIdentity(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, err := netip.ParseAddrPort(r.RemoteAddr)
			var ip netip.Addr
			if err == nil {
				ip = peer.Addr()
			} else {
				ip, _ = netip.ParseAddr(r.RemoteAddr)
			}
			ip = ip.Unmap().WithZone("")
			actualPeer := ip.String()
			if ip.IsValid() && isTrusted(ip, trusted) {
				if forwarded, ok := forwardedClient(r.Header.Values("X-Forwarded-For"), ip, trusted); ok {
					ip = forwarded
				}
			}
			if ip.IsValid() {
				r.RemoteAddr = ip.String()
			} else {
				r.RemoteAddr = "unknown"
			}
			ctx := context.WithValue(r.Context(), peerKey{}, actualPeer)
			ctx = context.WithValue(ctx, clientKey{}, r.RemoteAddr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isTrusted(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func forwardedClient(headers []string, peer netip.Addr, trusted []netip.Prefix) (netip.Addr, bool) {
	if len(headers) == 0 {
		return peer, false
	}
	parts := strings.Split(strings.Join(headers, ","), ",")
	if len(parts) > 64 {
		return peer, false
	}
	chain := make([]netip.Addr, len(parts))
	for i, part := range parts {
		addr, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil || addr.Zone() != "" {
			return peer, false
		}
		chain[i] = addr.Unmap()
	}
	client := peer
	for i := len(chain) - 1; i >= 0 && isTrusted(client, trusted); i-- {
		client = chain[i]
	}
	return client, true
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			pattern := "unmatched"
			if route := chi.RouteContext(r.Context()); route != nil && route.RoutePattern() != "" {
				pattern = route.RoutePattern()
			}
			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
				if pattern == "/ws/agent" && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
					status = http.StatusSwitchingProtocols
				}
			}
			peer, _ := r.Context().Value(peerKey{}).(string)
			log.Info().Str("method", r.Method).Str("route", pattern).Str("client_ip", r.RemoteAddr).
				Str("peer_ip", peer).Int("status", status).Dur("duration", time.Since(started)).Msg("request completed")
		}()
		next.ServeHTTP(wrapped, r)
	})
}
