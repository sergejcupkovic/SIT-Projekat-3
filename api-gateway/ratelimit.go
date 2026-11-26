package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiter keeps one token bucket per caller. Authenticated callers are
// keyed by the user ID from their JWT, everybody else by IP address.
type rateLimiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	limit     rate.Limit
	burst     int
	jwtSecret []byte
}

func newRateLimiter(limit rate.Limit, burst int, jwtSecret []byte) *rateLimiter {
	return &rateLimiter{
		visitors:  make(map[string]*visitor),
		limit:     limit,
		burst:     burst,
		jwtSecret: jwtSecret,
	}
}

func (rl *rateLimiter) getVisitor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// startCleanup drops, every interval, the buckets of callers idle for idleTTL.
func (rl *rateLimiter) startCleanup(interval, idleTTL time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			rl.mu.Lock()
			for key, v := range rl.visitors {
				if time.Since(v.lastSeen) > idleTTL {
					delete(rl.visitors, key)
				}
			}
			rl.mu.Unlock()
		}
	}()
}

// key returns "user:<id>" for a request carrying a valid session token and
// "ip:<addr>" otherwise. The signature is verified so that a forged "sub"
// claim cannot be used to get a fresh bucket on every request.
func (rl *rateLimiter) key(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token, err := jwt.Parse(strings.TrimPrefix(h, "Bearer "), func(t *jwt.Token) (any, error) {
			return rl.jwtSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err == nil && token.Valid {
			if sub, err := token.Claims.GetSubject(); err == nil && sub != "" {
				return "user:" + sub
			}
		}
	}
	return "ip:" + clientIP(r)
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.getVisitor(rl.key(r)).Allow() {
			retryAfter := int(1/float64(rl.limit)) + 1
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"code":8,"message":"Too many requests, please slow down.","details":[]}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP prefers the last X-Forwarded-For entry, the one appended by our own
// nginx; the leftmost entries are supplied by the client and can be spoofed.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
