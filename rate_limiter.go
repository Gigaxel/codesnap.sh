package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	MaxAttempts    = 100
	RateLimiterTTL = time.Hour
)

type RateLimiter struct {
	store Store
}

func NewRateLimiter(store Store) *RateLimiter {
	return &RateLimiter{
		store: store,
	}
}

func (r *RateLimiter) KeyValue(key string) string {
	return fmt.Sprintf("rate_limiter:%s", key)
}

func (r *RateLimiter) IsRateLimited(ctx context.Context, key string) (bool, error) {
	key = r.KeyValue(key)
	currentValue, err := r.store.Get(ctx, key)
	if err != nil && !errors.Is(err, ErrKeyNotFound) {
		return false, err
	}
	if string(currentValue) == "" {
		currentValue = []byte("0")
	}
	count, err := strconv.Atoi(string(currentValue))
	if err != nil {
		return false, err
	}
	if count >= MaxAttempts {
		return true, nil
	}
	val, err := r.store.Incr(ctx, key)
	if err != nil {
		return false, err
	}
	if val == 1 {
		err = r.store.Expire(ctx, key, RateLimiterTTL)
		if err != nil {
			return false, err
		}
	}
	return false, nil
}
