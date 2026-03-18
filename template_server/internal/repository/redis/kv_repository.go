package redis

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type KVRepository interface {
	SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error)
	Consume(ctx context.Context, key string) (bool, error)
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type kvRepository struct {
	client *goredis.Client
}

var consumeScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func NewKVRepository(client *goredis.Client) KVRepository {
	return &kvRepository{client: client}
}

func (r *kvRepository) SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

func (r *kvRepository) Consume(ctx context.Context, key string) (bool, error) {
	result, err := consumeScript.Run(ctx, r.client, []string{key}).Int64()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (r *kvRepository) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *kvRepository) Get(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == goredis.Nil {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (r *kvRepository) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *kvRepository) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
