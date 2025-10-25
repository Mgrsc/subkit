package server

import (
	"subkit/internal/logger"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	defaultDailyLimit = 100
)

type RateLimiter struct {
	mu           sync.RWMutex
	requests     int
	dailyLimit   int
	currentDate  string
}

func NewRateLimiter() *RateLimiter {
	dailyLimit := defaultDailyLimit
	if limitStr := os.Getenv("DAILY_REQUEST_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			dailyLimit = limit
			logger.Info("[RateLimit] Daily request limit set to %d", dailyLimit)
		}
	}

	return &RateLimiter{
		requests:    0,
		dailyLimit:  dailyLimit,
		currentDate: time.Now().Format("2006-01-02"),
	}
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rl.mu.Lock()
		defer rl.mu.Unlock()

		today := time.Now().Format("2006-01-02")
		if today != rl.currentDate {
			logger.Info("[RateLimit] New day: %s, resetting counter (previous: %d requests)", today, rl.requests)
			rl.currentDate = today
			rl.requests = 0
		}

		if rl.requests >= rl.dailyLimit {
			logger.Info("[RateLimit] Rate limit exceeded: %d/%d requests used", rl.requests, rl.dailyLimit)
			w.Header().Set("Content-Type", contentTypeJSON)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.dailyLimit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Daily request limit exceeded. Please try again tomorrow."}`))
			return
		}

		rl.requests++
		remaining := rl.dailyLimit - rl.requests
		logger.Info("[RateLimit] Request allowed: %d/%d (remaining: %d)", rl.requests, rl.dailyLimit, remaining)

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.dailyLimit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", getTomorrowMidnight())

		next.ServeHTTP(w, r)
	}
}

func getTomorrowMidnight() string {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
	return strconv.FormatInt(midnight.Unix(), 10)
}
