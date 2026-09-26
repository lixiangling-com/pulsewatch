package checker

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrBlockedAddress = errors.New("blocked address")

const maxResponseBytes = 4 << 10

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type HTTPChecker struct {
	client   *http.Client
	resolver Resolver
	dev      bool
}

func NewHTTPChecker(development bool) *HTTPChecker {
	return NewHTTPCheckerWithResolver(development, net.DefaultResolver)
}

func NewHTTPCheckerWithResolver(development bool, resolver Resolver) *HTTPChecker {
	checker := &HTTPChecker{resolver: resolver, dev: development}
	transport := &http.Transport{
		Proxy:           nil,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:     checker.dialContext,
	}
	checker.client = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("redirect_limit")
			}
			if req.URL.User != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
				return errors.New("invalid_redirect")
			}
			return nil
		},
	}
	return checker
}

func (h *HTTPChecker) Check(ctx context.Context, target Target) Result {
	started := time.Now()
	result := Result{Outcome: OutcomeTargetFailure}
	u, err := url.ParseRequestURI(target.URL)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return finish(result, started, "invalid_url", "target URL is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return finish(result, started, "invalid_url", "target URL is invalid")
	}
	request.Header.Set("User-Agent", "PulseWatch-Checker/1.0")
	response, err := h.client.Do(request)
	if err != nil {
		if errors.Is(err, ErrBlockedAddress) || strings.Contains(err.Error(), ErrBlockedAddress.Error()) {
			result.Outcome = OutcomeBlocked
			return finish(result, started, "blocked_address", "target resolves to a restricted address")
		}
		code := classifyError(err)
		return finish(result, started, code, safeSummary(code))
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	_, _ = io.CopyN(io.Discard, response.Body, maxResponseBytes)
	if response.StatusCode == target.ExpectedStatus {
		result.Outcome = OutcomeSuccess
		return finish(result, started, "", "")
	}
	code := "unexpected_status"
	if response.StatusCode >= 500 {
		code = "http_5xx"
	} else if response.StatusCode >= 400 {
		code = "http_4xx"
	}
	return finish(result, started, code, safeSummary(code))
}

func (h *HTTPChecker) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if h.dev && strings.EqualFold(strings.TrimSuffix(host, "."), "mocktarget") {
		ips, err := h.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("dns_error")
		}
		for _, ip := range ips {
			if !ip.IsValid() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
				return nil, ErrBlockedAddress
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	ips, err := h.resolvePublic(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

func (h *HTTPChecker) resolvePublic(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if !allowedIP(ip) {
			return nil, ErrBlockedAddress
		}
		return []netip.Addr{ip}, nil
	}
	ips, err := h.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("dns_error")
	}
	for _, ip := range ips {
		if !allowedIP(ip) {
			return nil, ErrBlockedAddress
		}
	}
	return ips, nil
}

func allowedIP(ip netip.Addr) bool {
	if ip.Is4In6() {
		ip = ip.Unmap()
	}
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	blocked := []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32", "2002::/16",
	}
	for _, raw := range blocked {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case strings.Contains(err.Error(), "dns_error"):
		return "dns_error"
	case strings.Contains(err.Error(), "redirect_limit"):
		return "redirect_limit"
	case strings.Contains(err.Error(), "invalid_redirect"):
		return "invalid_redirect"
	case strings.Contains(err.Error(), "tls") || strings.Contains(err.Error(), "certificate"):
		return "tls_error"
	case func() bool { var n net.Error; return errors.As(err, &n) && n.Timeout() }():
		return "timeout"
	case strings.Contains(err.Error(), "connection refused"), strings.Contains(err.Error(), "no such host"), strings.Contains(err.Error(), "network is unreachable"):
		return "connection_error"
	default:
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return "connection_error"
		}
		return "connection_error"
	}
}

func safeSummary(code string) string {
	return map[string]string{
		"timeout": "target request timed out", "dns_error": "target name could not be resolved",
		"redirect_limit": "target exceeded the redirect limit", "tls_error": "TLS connection failed",
		"invalid_redirect": "target returned an unsafe redirect",
		"connection_error": "target connection failed", "http_4xx": "target returned a client error",
		"http_5xx": "target returned a server error", "unexpected_status": "target returned an unexpected status",
	}[code]
}

func finish(result Result, started time.Time, code, summary string) Result {
	result.Latency = time.Since(started)
	result.ErrorCode, result.Summary = code, summary
	return result
}
