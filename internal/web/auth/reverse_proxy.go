// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"golang.org/x/exp/slices"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// getUserClientFunc is a function type that returns a Kubernetes client for a given user impersonation.
type getUserClientFunc func(user.Impersonation) (any, error)

// trustedProxySet contains the IP prefixes from which reverse-proxy
// authentication headers may be trusted.
type trustedProxySet struct {
	prefixes []netip.Prefix
}

// newReverseProxyMiddleware creates authentication middleware that obtains the
// user's identity from headers added by a trusted reverse proxy.
func newReverseProxyMiddleware(
	conf *fluxcdv1.WebConfigSpec,
	kubeClient *kubeclient.Client,
) (func(next http.Handler) http.Handler, error) {
	return newReverseProxyMiddlewareWithClientFactory(
		conf,
		func(imp user.Impersonation) (any, error) {
			return kubeClient.GetUserClientFromCache(imp)
		},
	)
}

func newReverseProxyMiddlewareWithClientFactory(
	conf *fluxcdv1.WebConfigSpec,
	getUserClient getUserClientFunc,
) (func(next http.Handler) http.Handler, error) {
	if conf == nil ||
		conf.Authentication == nil ||
		conf.Authentication.ReverseProxy == nil {
		return nil, fmt.Errorf("reverse proxy authentication is not configured")
	}

	cfg := conf.Authentication.ReverseProxy

	if strings.TrimSpace(cfg.Headers.Username) == "" {
		return nil, fmt.Errorf("reverse proxy username header must be configured")
	}

	trustedProxies, err := newTrustedProxySet(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("failed to configure trusted proxies: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peerIP, err := remoteIP(r.RemoteAddr)
			if err != nil {
				http.Error(
					w,
					"invalid reverse proxy address",
					http.StatusUnauthorized,
				)
				return
			}

			if !trustedProxies.Contains(peerIP) {
				http.Error(
					w,
					"request did not originate from a trusted reverse proxy",
					http.StatusUnauthorized,
				)
				return
			}

			username := strings.TrimSpace(
				r.Header.Get(cfg.Headers.Username),
			)
			if username == "" {
				http.Error(
					w,
					"authenticated username header is missing",
					http.StatusUnauthorized,
				)
				return
			}

			name := username
			if cfg.Headers.Name != "" {
				if headerName := strings.TrimSpace(
					r.Header.Get(cfg.Headers.Name),
				); headerName != "" {
					name = headerName
				}
			}

			groups := parseHeaderList(
				r.Header.Values(cfg.Headers.Groups),
				cfg.Groups,
			)

			if len(groups) == 0 {
				groups = slices.Clone(cfg.DefaultGroups)
			}

			details := user.Details{
				Profile: user.Profile{
					Name: name,
				},
				Impersonation: user.Impersonation{
					Username: username,
					Groups:   groups,
				},
				Provider: map[string]any{
					"type": fluxcdv1.AuthenticationTypeReverseProxy,
				},
			}

			if err := details.Impersonation.SanitizeAndValidate(); err != nil {
				http.Error(
					w,
					"invalid authenticated identity",
					http.StatusUnauthorized,
				)
				return
			}

			client, err := getUserClient(details.Impersonation)
			if err != nil {
				http.Error(
					w,
					"failed to initialize Kubernetes client",
					http.StatusInternalServerError,
				)
				return
			}

			SetReverseProxyAuthProviderCookie(w)

			ctx := user.StoreSession(
				r.Context(),
				details,
				client,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// newTrustedProxySet parses trusted proxy IP addresses and CIDR ranges.
//
// Both of these are accepted:
//
//   - 10.42.1.12
//   - 10.42.0.0/16
//   - fd7a:115c:a1e0::1
//   - fd7a:115c:a1e0::/48
func newTrustedProxySet(values []string) (*trustedProxySet, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one trusted proxy must be configured")
	}

	prefixes := make([]netip.Prefix, 0, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("trusted proxy at index %d is empty", i)
		}

		prefix, err := parseIPOrPrefix(value)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid trusted proxy %q: %w",
				value,
				err,
			)
		}

		prefixes = append(prefixes, prefix.Masked())
	}

	return &trustedProxySet{
		prefixes: prefixes,
	}, nil
}

// parseIPOrPrefix accepts either an individual IP address or a CIDR prefix.
func parseIPOrPrefix(value string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix, nil
	}

	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf(
			"expected an IP address or CIDR prefix: %w",
			err,
		)
	}

	bits := 128
	if addr.Is4() {
		bits = 32
	}

	return netip.PrefixFrom(addr, bits), nil
}

// Contains reports whether an IP address belongs to one of the configured
// trusted proxy prefixes.
func (s *trustedProxySet) Contains(addr netip.Addr) bool {
	if s == nil || !addr.IsValid() {
		return false
	}

	addr = addr.Unmap()

	for _, prefix := range s.prefixes {
		prefixAddr := prefix.Addr().Unmap()

		// A mapped IPv4 address and a native IPv4 prefix should be compared
		// using their unmapped representations.
		if addr.BitLen() != prefixAddr.BitLen() {
			continue
		}

		normalizedPrefix := netip.PrefixFrom(
			prefixAddr,
			prefix.Bits(),
		)

		if normalizedPrefix.Contains(addr) {
			return true
		}
	}

	return false
}

// remoteIP extracts the direct TCP peer IP address from http.Request.RemoteAddr.
//
// It intentionally does not inspect X-Forwarded-For or Forwarded, because those
// headers may be supplied by an untrusted client.
func remoteIP(remoteAddr string) (netip.Addr, error) {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return netip.Addr{}, fmt.Errorf("remote address is empty")
	}

	// The normal net/http representation is "host:port".
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		addr, parseErr := netip.ParseAddr(
			strings.Trim(host, "[]"),
		)
		if parseErr != nil {
			return netip.Addr{}, fmt.Errorf(
				"failed to parse remote IP %q: %w",
				host,
				parseErr,
			)
		}

		return addr.Unmap(), nil
	}

	// Supporting a bare address makes the helper easier to use in tests and
	// also handles custom HTTP transports that do not include a port.
	addr, parseErr := netip.ParseAddr(
		strings.Trim(remoteAddr, "[]"),
	)
	if parseErr != nil {
		return netip.Addr{}, fmt.Errorf(
			"failed to parse remote address %q: %w",
			remoteAddr,
			parseErr,
		)
	}

	return addr.Unmap(), nil
}

// parseHeaderList parses one or more HTTP header values into a normalized,
// sorted and deduplicated list.
//
// For example:
//
//	X-Auth-Groups: platform,developers
//	X-Auth-Groups: flux-admin
//
// becomes:
//
//	[]string{"developers", "flux-admin", "platform"}
func parseHeaderList(headerValues []string,
	cfg *fluxcdv1.HeaderListSpec) []string {
	if len(headerValues) == 0 {
		return nil
	}

	separator := ","
	if cfg != nil && cfg.Separator != "" {
		separator = cfg.Separator
	}

	seen := make(map[string]struct{})
	values := make([]string, 0)

	for _, headerValue := range headerValues {
		var parts []string

		if separator == "" {
			parts = []string{headerValue}
		} else {
			parts = strings.Split(headerValue, separator)
		}

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			if _, exists := seen[part]; exists {
				continue
			}

			seen[part] = struct{}{}
			values = append(values, part)
		}
	}

	if len(values) == 0 {
		return nil
	}

	slices.Sort(values)
	return values
}
