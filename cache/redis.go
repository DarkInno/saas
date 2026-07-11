package cache

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var _ Cache = (*Redis)(nil)

// RedisClient is the subset of go-redis used by the cache adapter.
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

// RedisPinger is implemented by go-redis clients that support PING health checks.
type RedisPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// Redis stores cache values in Redis through a go-redis client.
type Redis struct {
	client RedisClient
}

// NewRedis creates a cache adapter from an existing go-redis client.
//
// The caller owns the client configuration. Close closes the provided client.
func NewRedis(client RedisClient) (*Redis, error) {
	if client == nil {
		return nil, ErrInvalidRedisConfig
	}
	return &Redis{client: client}, nil
}

// NewRedisFromOptions creates a Redis cache adapter from standalone client options.
func NewRedisFromOptions(options *redis.Options) (*Redis, error) {
	if options == nil {
		return nil, ErrInvalidRedisConfig
	}
	cloned := *options
	return NewRedis(redis.NewClient(&cloned))
}

// NewRedisFromClusterOptions creates a Redis cache adapter from cluster client options.
func NewRedisFromClusterOptions(options *redis.ClusterOptions) (*Redis, error) {
	if options == nil {
		return nil, ErrInvalidRedisConfig
	}
	cloned := *options
	return NewRedis(redis.NewClusterClient(&cloned))
}

// NewRedisFromURL creates a Redis cache adapter from a redis:// or rediss:// URL.
func NewRedisFromURL(rawURL string) (*Redis, error) {
	if rawURL == "" {
		return nil, ErrInvalidRedisConfig
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, ErrInvalidRedisConfig
	}
	return NewRedisFromOptions(options)
}

// Get returns a cache value.
func (cache *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	client, err := cache.redisClient()
	if err != nil {
		return nil, false, err
	}

	cmd := client.Get(ctx, key)
	if cmd == nil {
		return nil, false, ErrInvalidRedisConfig
	}
	value, err := cmd.Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cloneBytes(value), true, nil
}

// Set stores a cache value.
func (cache *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, err := cache.redisClient()
	if err != nil {
		return err
	}

	cmd := client.Set(ctx, key, cloneBytes(value), ttl)
	if cmd == nil {
		return ErrInvalidRedisConfig
	}
	return cmd.Err()
}

// Delete removes a cache value.
func (cache *Redis) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, err := cache.redisClient()
	if err != nil {
		return err
	}

	cmd := client.Del(ctx, key)
	if cmd == nil {
		return ErrInvalidRedisConfig
	}
	return cmd.Err()
}

// Ping checks whether the underlying Redis client can reach the server.
func (cache *Redis) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, err := cache.redisClient()
	if err != nil {
		return err
	}
	pinger, ok := client.(RedisPinger)
	if !ok {
		return ErrInvalidRedisConfig
	}

	cmd := pinger.Ping(ctx)
	if cmd == nil {
		return ErrInvalidRedisConfig
	}
	return cmd.Err()
}

// Close closes the underlying Redis client.
func (cache *Redis) Close() error {
	client, err := cache.redisClient()
	if err != nil {
		return err
	}
	return client.Close()
}

func (cache *Redis) redisClient() (RedisClient, error) {
	if cache == nil || cache.client == nil {
		return nil, ErrInvalidRedisConfig
	}
	return cache.client, nil
}
