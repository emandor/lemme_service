package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
)

func MustConnect(addr string, db int) *redis.Client {
	r := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	if err := r.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	return r
}
