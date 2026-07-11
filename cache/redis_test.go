package cache

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func TestRedisCacheSetGetDelete(t *testing.T) {
	client := &fakeRedisClient{
		getFunc: func(context.Context, string) *redis.StringCmd {
			return redis.NewStringResult("value", nil)
		},
		setFunc: func(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("OK", nil)
		},
		delFunc: func(context.Context, ...string) *redis.IntCmd {
			return redis.NewIntResult(1, nil)
		},
	}
	cache, err := NewRedis(client)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}

	value := []byte("value")
	if err := cache.Set(context.Background(), "key", value, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value[0] = 'x'
	if client.setKey != "key" || string(client.setValue) != "value" || client.setTTL != time.Minute {
		t.Fatalf("Set() call = key %q value %q ttl %s", client.setKey, client.setValue, client.setTTL)
	}

	got, ok, err := cache.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != "value" {
		t.Fatalf("Get() = %q, %v; want value, true", got, ok)
	}
	got[0] = 'x'
	again, ok, err := cache.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get() again error = %v", err)
	}
	if !ok || string(again) != "value" {
		t.Fatalf("Get() after mutation = %q, %v; want value, true", again, ok)
	}

	if err := cache.Delete(context.Background(), "key"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !reflect.DeepEqual(client.delKeys, []string{"key"}) {
		t.Fatalf("Delete() keys = %v, want [key]", client.delKeys)
	}
}

func TestRedisCacheMissMapsRedisNil(t *testing.T) {
	client := &fakeRedisClient{
		getFunc: func(context.Context, string) *redis.StringCmd {
			return redis.NewStringResult("", redis.Nil)
		},
	}
	cache, err := NewRedis(client)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}

	got, ok, err := cache.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok || got != nil {
		t.Fatalf("Get() = %q, %v; want miss", got, ok)
	}
}

func TestRedisCachePropagatesCommandErrors(t *testing.T) {
	down := errors.New("redis down")
	client := &fakeRedisClient{
		getFunc: func(context.Context, string) *redis.StringCmd {
			return redis.NewStringResult("", down)
		},
		setFunc: func(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
			return redis.NewStatusResult("", down)
		},
		delFunc: func(context.Context, ...string) *redis.IntCmd {
			return redis.NewIntResult(0, down)
		},
		pingFunc: func(context.Context) *redis.StatusCmd {
			return redis.NewStatusResult("", down)
		},
		closeFunc: func() error {
			return down
		},
	}
	cache, err := NewRedis(client)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}

	if _, _, err := cache.Get(context.Background(), "key"); !errors.Is(err, down) {
		t.Fatalf("Get() error = %v, want redis down", err)
	}
	if err := cache.Set(context.Background(), "key", []byte("value"), 0); !errors.Is(err, down) {
		t.Fatalf("Set() error = %v, want redis down", err)
	}
	if err := cache.Delete(context.Background(), "key"); !errors.Is(err, down) {
		t.Fatalf("Delete() error = %v, want redis down", err)
	}
	if err := cache.Ping(context.Background()); !errors.Is(err, down) {
		t.Fatalf("Ping() error = %v, want redis down", err)
	}
	if err := cache.Close(); !errors.Is(err, down) {
		t.Fatalf("Close() error = %v, want redis down", err)
	}
}

func TestRedisCacheContextCancellationDoesNotCallClient(t *testing.T) {
	client := &fakeRedisClient{}
	cache, err := NewRedis(client)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := cache.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get(canceled) error = %v, want context.Canceled", err)
	}
	if err := cache.Set(ctx, "key", []byte("value"), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Set(canceled) error = %v, want context.Canceled", err)
	}
	if err := cache.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete(canceled) error = %v, want context.Canceled", err)
	}
	if err := cache.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping(canceled) error = %v, want context.Canceled", err)
	}
	if client.getCalled || client.setCalled || client.delCalled || client.pingCalled {
		t.Fatalf("canceled operations called client: get=%v set=%v del=%v ping=%v", client.getCalled, client.setCalled, client.delCalled, client.pingCalled)
	}
}

func TestRedisCachePing(t *testing.T) {
	client := &fakeRedisClient{
		pingFunc: func(context.Context) *redis.StatusCmd {
			return redis.NewStatusResult("PONG", nil)
		},
	}
	cache, err := NewRedis(client)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}

	if err := cache.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if !client.pingCalled {
		t.Fatal("Ping() did not call client")
	}
}

func TestRedisCachePingRequiresPinger(t *testing.T) {
	cache, err := NewRedis(redisClientWithoutPing{})
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}

	if err := cache.Ping(context.Background()); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("Ping() error = %v, want ErrInvalidRedisConfig", err)
	}
}

func TestRedisCacheInvalidConfig(t *testing.T) {
	if _, err := NewRedis(nil); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedis(nil) error = %v, want ErrInvalidRedisConfig", err)
	}
	if _, err := NewRedisFromOptions(nil); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedisFromOptions(nil) error = %v, want ErrInvalidRedisConfig", err)
	}
	if _, err := NewRedisFromClusterOptions(nil); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedisFromClusterOptions(nil) error = %v, want ErrInvalidRedisConfig", err)
	}
	if _, err := NewRedisFromURL(""); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedisFromURL(empty) error = %v, want ErrInvalidRedisConfig", err)
	}
	if _, err := NewRedisFromURL("://bad"); !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedisFromURL(invalid) error = %v, want ErrInvalidRedisConfig", err)
	}
}

func TestNewRedisFromURLDoesNotLeakCredentialsOnParseFailure(t *testing.T) {
	const (
		rawURL   = "redis://cache-user:super-secret@localhost/%zz"
		password = "super-secret"
	)

	_, err := NewRedisFromURL(rawURL)
	if !errors.Is(err, ErrInvalidRedisConfig) {
		t.Fatalf("NewRedisFromURL() error = %v, want ErrInvalidRedisConfig", err)
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), rawURL) {
		t.Fatalf("NewRedisFromURL() error leaked credentials: %q", err)
	}
}

func TestRedisCacheConstructors(t *testing.T) {
	cache, err := NewRedisFromOptions(&redis.Options{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("NewRedisFromOptions() error = %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close(options cache) error = %v", err)
	}

	cache, err = NewRedisFromClusterOptions(&redis.ClusterOptions{Addrs: []string{"127.0.0.1:0"}})
	if err != nil {
		t.Fatalf("NewRedisFromClusterOptions() error = %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close(cluster cache) error = %v", err)
	}

	cache, err = NewRedisFromURL("redis://localhost:6379/0")
	if err != nil {
		t.Fatalf("NewRedisFromURL() error = %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("Close(url cache) error = %v", err)
	}
}

type fakeRedisClient struct {
	getFunc   func(context.Context, string) *redis.StringCmd
	setFunc   func(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	delFunc   func(context.Context, ...string) *redis.IntCmd
	pingFunc  func(context.Context) *redis.StatusCmd
	closeFunc func() error

	getCalled  bool
	setCalled  bool
	delCalled  bool
	pingCalled bool

	setKey   string
	setValue []byte
	setTTL   time.Duration
	delKeys  []string
}

func (client *fakeRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	client.getCalled = true
	if client.getFunc != nil {
		return client.getFunc(ctx, key)
	}
	return redis.NewStringResult("", nil)
}

func (client *fakeRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	client.setCalled = true
	client.setKey = key
	client.setTTL = expiration
	if bytes, ok := value.([]byte); ok {
		client.setValue = cloneBytes(bytes)
	}
	if client.setFunc != nil {
		return client.setFunc(ctx, key, value, expiration)
	}
	return redis.NewStatusResult("OK", nil)
}

func (client *fakeRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	client.delCalled = true
	client.delKeys = append([]string(nil), keys...)
	if client.delFunc != nil {
		return client.delFunc(ctx, keys...)
	}
	return redis.NewIntResult(int64(len(keys)), nil)
}

func (client *fakeRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	client.pingCalled = true
	if client.pingFunc != nil {
		return client.pingFunc(ctx)
	}
	return redis.NewStatusResult("PONG", nil)
}

func (client *fakeRedisClient) Close() error {
	if client.closeFunc != nil {
		return client.closeFunc()
	}
	return nil
}

type redisClientWithoutPing struct{}

func (redisClientWithoutPing) Get(context.Context, string) *redis.StringCmd {
	return redis.NewStringResult("", nil)
}

func (redisClientWithoutPing) Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
	return redis.NewStatusResult("OK", nil)
}

func (redisClientWithoutPing) Del(context.Context, ...string) *redis.IntCmd {
	return redis.NewIntResult(0, nil)
}

func (redisClientWithoutPing) Close() error {
	return nil
}
