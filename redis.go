package main

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisStore struct {
	redisClient *redis.Client
}

func NewRedisStore(redisClient *redis.Client) *RedisStore {
	return &RedisStore{redisClient: redisClient}
}

func (r *RedisStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.redisClient.Set(ctx, key, value, ttl).Err()
}

func (r *RedisStore) Incr(ctx context.Context, key string) (int64, error) {
	return r.redisClient.Incr(ctx, key).Result()
}

func (r *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	res, err := r.redisClient.Get(ctx, key).Bytes()
	switch {
	case errors.Is(err, redis.Nil):
		return nil, ErrKeyNotFound
	case err != nil:
		return nil, err
	}
	return res, nil
}

func (r *RedisStore) Del(ctx context.Context, key string) error {
	return r.redisClient.Del(ctx, key).Err()
}

func (r *RedisStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.redisClient.Expire(ctx, key, ttl).Err()
}
