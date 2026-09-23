package checker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

func TestHTTPCheckerClassifiesStatusAndDoesNotPersistBody(t *testing.T) {
	server := startTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("User-Agent") == "" {
			t.Errorf("unexpected request: %s", r.Method)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("private response secret"))
	}))
	defer server.Close()
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	targetURL := strings.Replace(server.URL, "127.0.0.1", "mocktarget", 1)
	result := NewHTTPCheckerWithResolver(true, resolver).Check(context.Background(), Target{URL: targetURL, ExpectedStatus: http.StatusOK})
	if result.Outcome != OutcomeTargetFailure || result.ErrorCode != "http_5xx" || result.StatusCode != 500 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Contains(result.Summary, "secret") {
		t.Fatalf("response content leaked: %q", result.Summary)
	}
}

func TestHTTPCheckerRejectsPrivateAndRedirectToPrivate(t *testing.T) {
	privateResolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	result := NewHTTPCheckerWithResolver(false, privateResolver).Check(context.Background(), Target{URL: "http://private.invalid", ExpectedStatus: 200})
	if result.Outcome != OutcomeBlocked || result.ErrorCode != "blocked_address" {
		t.Fatalf("private target result: %+v", result)
	}
	redirectServer := startTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://private.invalid", http.StatusFound)
	}))
	defer redirectServer.Close()
	redirectResolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	redirectURL := strings.Replace(redirectServer.URL, "127.0.0.1", "mocktarget", 1)
	redirectResult := NewHTTPCheckerWithResolver(true, redirectResolver).Check(context.Background(), Target{URL: redirectURL, ExpectedStatus: 200})
	if redirectResult.Outcome != OutcomeBlocked || redirectResult.ErrorCode != "blocked_address" {
		t.Fatalf("redirect to private target was not blocked: %+v", redirectResult)
	}

}

func TestDNSRebindingIsRevalidatedAndRedirectLimit(t *testing.T) {
	answers := [][]netip.Addr{{netip.MustParseAddr("1.1.1.1")}, {netip.MustParseAddr("127.0.0.1")}}
	n := 0
	rebinding := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		answer := answers[n]
		n++
		return answer, nil
	})
	dev := NewHTTPCheckerWithResolver(false, rebinding)
	if _, err := dev.resolvePublic(context.Background(), "target.example"); err != nil {
		t.Fatal(err)
	}
	if _, err := dev.resolvePublic(context.Background(), "target.example"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("rebound redirect address was not blocked: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected fresh DNS lookup, got %d", n)
	}
	if err := dev.client.CheckRedirect(&http.Request{URL: mustURL("http://example.com")}, make([]*http.Request, 6)); err == nil || err.Error() != "redirect_limit" {
		t.Fatalf("redirect limit not enforced: %v", err)
	}
}

func startTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local test listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestAllowedIPRejectsReservedAndPrivateRanges(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "192.0.2.1", "198.18.0.1", "::1", "fc00::1", "fe80::1", "2001:db8::1", "224.0.0.1"} {
		if allowedIP(netip.MustParseAddr(raw)) {
			t.Errorf("allowed restricted IP %s", raw)
		}
	}
	if !allowedIP(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public IP rejected")
	}
}

func TestDNSFailureIsTargetFailureAndSummaryIsSafe(t *testing.T) {
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return nil, errors.New("resolver secret detail")
	})
	result := NewHTTPCheckerWithResolver(false, resolver).Check(context.Background(), Target{URL: "http://example.invalid", ExpectedStatus: 200})
	if result.Outcome != OutcomeTargetFailure || result.ErrorCode != "dns_error" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Contains(result.Summary, "secret") {
		t.Fatalf("DNS error leaked: %q", result.Summary)
	}
}

func TestDevelopmentMockTargetExceptionIsExact(t *testing.T) {
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	checker := NewHTTPCheckerWithResolver(true, resolver)
	if _, err := checker.dialContext(context.Background(), "tcp", net.JoinHostPort("mocktarget", "8080")); err == nil || errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("exact development host should allow private address, got %v", err)
	}
	if _, err := checker.dialContext(context.Background(), "tcp", net.JoinHostPort("mocktarget.evil", "8080")); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("non-exact hostname should not get private exception, got %v", err)
	}
}
