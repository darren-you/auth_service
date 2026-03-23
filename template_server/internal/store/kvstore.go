package store

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type KVStore interface {
	SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error)
	Consume(ctx context.Context, key string) (bool, error)
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type kvStore struct {
	client *goredis.Client
}

var consumeScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func NewKVStore(client *goredis.Client) KVStore {
	return &kvStore{client: client}
}

func (s *kvStore) SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, expiration).Result()
}

func (s *kvStore) Consume(ctx context.Context, key string) (bool, error) {
	result, err := consumeScript.Run(ctx, s.client, []string{key}).Int64()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (s *kvStore) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

func (s *kvStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == goredis.Nil {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (s *kvStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *kvStore) Exists(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
