package main

import (
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/time/rate"
)

func RateLimiter(numberOfRequests int, handler ssh.Handler) ssh.Handler {
	limiter := rate.NewLimiter(rate.Every(time.Hour), numberOfRequests)
	return func(s ssh.Session) {
		if limiter.Allow() {
			handler(s)
		} else {
			s.Exit(1)
		}
	}
}
