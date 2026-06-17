package main

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	rlWindow  = time.Minute
	rlMaxReqs = 20
	rlCleanup = 5 * time.Minute
)

var (
	rlMu        sync.Mutex
	rlStore     = map[string][]time.Time{}
	rlLastClean = time.Now()
)

// authRateLimit limits each IP to rlMaxReqs requests per rlWindow on auth endpoints.
func authRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i != -1 {
			ip = ip[:i]
		}
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.SplitN(forwarded, ",", 2)
			ip = strings.TrimSpace(parts[0])
		}

		now := time.Now()
		cutoff := now.Add(-rlWindow)

		rlMu.Lock()
		if now.Sub(rlLastClean) > rlCleanup {
			for k, ts := range rlStore {
				if len(ts) == 0 || ts[len(ts)-1].Before(cutoff) {
					delete(rlStore, k)
				}
			}
			rlLastClean = now
		}
		times := rlStore[ip]
		filtered := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		filtered = append(filtered, now)
		rlStore[ip] = filtered
		count := len(filtered)
		rlMu.Unlock()

		if count > rlMaxReqs {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":true,"message":"rate limit exceeded — try again in a minute"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
