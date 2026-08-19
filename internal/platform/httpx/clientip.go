package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the client address used for rate limiting. X-Forwarded-For and
// X-Real-IP are honored only when RemoteAddr is a trusted proxy; otherwise they
// are ignored so a direct client cannot pick an arbitrary bucket.
func ClientIP(r *http.Request, trusted []TrustedNetwork) string {
	remote := remoteHost(r.RemoteAddr)
	if !isTrusted(remote, trusted) {
		return remote
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		if ip := net.ParseIP(xrip); ip != nil {
			return ip.String()
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(parts[0])); ip != nil {
			return ip.String()
		}
	}
	return remote
}

// TrustedNetwork is a single IP or CIDR that may set forwarding headers.
type TrustedNetwork struct {
	net    net.IP
	prefix *net.IPNet
}

// ParseTrustedNetworks parses comma-separated IPs and CIDRs (e.g. 127.0.0.1,10.0.0.0/8).
func ParseTrustedNetworks(raw []string) ([]TrustedNetwork, error) {
	out := make([]TrustedNetwork, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			_, prefix, err := net.ParseCIDR(item)
			if err != nil {
				return nil, err
			}
			out = append(out, TrustedNetwork{prefix: prefix})
			continue
		}
		ip := net.ParseIP(item)
		if ip == nil {
			return nil, &net.ParseError{Type: "IP address", Text: item}
		}
		out = append(out, TrustedNetwork{net: ip})
	}
	return out, nil
}

func isTrusted(host string, trusted []TrustedNetwork) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, tn := range trusted {
		if tn.prefix != nil && tn.prefix.Contains(ip) {
			return true
		}
		if tn.net != nil && tn.net.Equal(ip) {
			return true
		}
	}
	return false
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
