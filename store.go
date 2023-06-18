package main

import (
	"context"
	"time"
)

type CodeStore interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Del(ctx context.Context, key string) error
	Incr(ctx context.Context, key string) (int64, error)
	Exists(ctx context.Context, key string) (int64, error)
}
