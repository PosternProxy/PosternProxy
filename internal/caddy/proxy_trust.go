package caddy

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/posternproxy/posternproxy/internal/security"
)

// ParseTrustedProxyPolicies reads controller-owned policies keyed by server ID.
// Static ranges avoid expanding trust when an external range source is unavailable.
func ParseTrustedProxyPolicies(raw string) (map[int64][]string, error) {
	policies := make(map[int64][]string)
	if strings.TrimSpace(raw) == "" {
		return policies, nil
	}
	d := json.NewDecoder(strings.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("Caddy trusted proxies must be a JSON object keyed by server ID")
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("read proxy policy key: %w", err)
		}
		id, err := strconv.ParseInt(key.(string), 10, 64)
		if err != nil || id < 1 || strconv.FormatInt(id, 10) != key.(string) {
			return nil, fmt.Errorf("proxy policy key must be a positive canonical server ID")
		}
		if _, exists := policies[id]; exists {
			return nil, fmt.Errorf("duplicate proxy policy for server %d", id)
		}
		var values []string
		if err := d.Decode(&values); err != nil {
			return nil, fmt.Errorf("read proxy ranges for server %d: %w", id, err)
		}
		ranges, err := validateProxyRanges(values)
		if err != nil {
			return nil, fmt.Errorf("proxy policy for server %d: %w", id, err)
		}
		policies[id] = ranges
	}
	if _, err := d.Token(); err != nil {
		return nil, fmt.Errorf("close proxy policies: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after proxy policies")
	}
	return policies, nil
}

func validateProxyRanges(values []string) ([]string, error) {
	prefixes, err := security.ParseTrustedProxies(values)
	if err != nil {
		return nil, err
	}
	ranges := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		if prefix.Bits() == 0 {
			return nil, fmt.Errorf("trusting all IP addresses is not permitted")
		}
		ranges = append(ranges, prefix.String())
	}
	return ranges, nil
}
