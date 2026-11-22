package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-test-secret-test-secret-test")

func tokenFor(t *testing.T, sub string, secret []byte) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func call(h http.Handler, token, ip string) int {
	req := httptest.NewRequest(http.MethodGet, "/api/folders", nil)
	req.RemoteAddr = ip + ":4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func newTestHandler(burst int) http.Handler {
	rl := newRateLimiter(0.001, burst, testSecret)
	return rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestRateLimitReturns429AfterBurst(t *testing.T) {
	h := newTestHandler(3)
	token := tokenFor(t, "user-1", testSecret)

	for i := 0; i < 3; i++ {
		if code := call(h, token, "10.0.0.1"); code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, code)
		}
	}
	if code := call(h, token, "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", code)
	}
}

func TestRateLimitIsPerUserNotPerIP(t *testing.T) {
	h := newTestHandler(2)
	first := tokenFor(t, "user-1", testSecret)
	second := tokenFor(t, "user-2", testSecret)

	call(h, first, "10.0.0.1")
	call(h, first, "10.0.0.1")
	if code := call(h, first, "10.0.0.2"); code != http.StatusTooManyRequests {
		t.Fatalf("same user from another IP should stay limited, got %d", code)
	}
	if code := call(h, second, "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("other user on the same IP should not be limited, got %d", code)
	}
}

func TestRateLimitIgnoresForgedToken(t *testing.T) {
	h := newTestHandler(2)

	for i := 0; i < 2; i++ {
		forged := tokenFor(t, "user-"+string(rune('a'+i)), []byte("another-secret-another-secret-another"))
		call(h, forged, "10.0.0.9")
	}
	forged := tokenFor(t, "user-z", []byte("another-secret-another-secret-another"))
	if code := call(h, forged, "10.0.0.9"); code != http.StatusTooManyRequests {
		t.Fatalf("forged tokens must fall back to the IP bucket, got %d", code)
	}
}

func TestRateLimitAnonymousUsesLastForwardedAddress(t *testing.T) {
	h := newTestHandler(1)

	send := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/users/login", nil)
		req.RemoteAddr = "172.18.0.5:5000"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := send("1.1.1.1, 203.0.113.7"); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if code := send("2.2.2.2, 203.0.113.7"); code != http.StatusTooManyRequests {
		t.Fatalf("spoofed leading XFF entry must not reset the bucket, got %d", code)
	}
}
