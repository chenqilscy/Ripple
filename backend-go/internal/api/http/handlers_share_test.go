package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsShareTokenFormat(t *testing.T) {
	valid := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP_"
	if len(valid) != shareTokenLength {
		t.Fatalf("test token length drifted: got %d", len(valid))
	}
	if !isShareTokenFormat(valid) {
		t.Fatalf("expected valid share token")
	}

	cases := []string{
		"",
		"short",
		valid + "x",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO/",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO.",
	}
	for _, tc := range cases {
		if isShareTokenFormat(tc) {
			t.Fatalf("expected invalid share token format for %q", tc)
		}
	}
}

func TestShareRateLimiter_AllowAndRefill(t *testing.T) {
	now := time.Unix(100, 0)
	lim := newShareRateLimiter(1, 2, time.Minute)
	lim.now = func() time.Time { return now }

	if !lim.allow("ip-1") {
		t.Fatalf("first request should pass")
	}
	if !lim.allow("ip-1") {
		t.Fatalf("burst request should pass")
	}
	if lim.allow("ip-1") {
		t.Fatalf("third immediate request should be limited")
	}

	now = now.Add(time.Second)
	if !lim.allow("ip-1") {
		t.Fatalf("request after refill should pass")
	}
}

func TestShareRateLimiter_CleansStaleEntries(t *testing.T) {
	now := time.Unix(100, 0)
	lim := newShareRateLimiter(1, 1, time.Minute)
	lim.now = func() time.Time { return now }

	if !lim.allow("ip-old") {
		t.Fatalf("initial request should pass")
	}
	if got := len(lim.entries); got != 1 {
		t.Fatalf("want 1 entry, got %d", got)
	}

	now = now.Add(2 * time.Minute)
	if !lim.allow("ip-new") {
		t.Fatalf("new key should pass")
	}
	if _, ok := lim.entries["ip-old"]; ok {
		t.Fatalf("stale entry was not removed")
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	if got := clientIP(req); got != "203.0.113.10" {
		t.Fatalf("want host only, got %q", got)
	}

	req.RemoteAddr = "unix-socket"
	if got := clientIP(req); got != "unix-socket" {
		t.Fatalf("want raw remote addr fallback, got %q", got)
	}
}
