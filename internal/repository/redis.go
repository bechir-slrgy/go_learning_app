package repository

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func MustConnectRedis(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis %s: %v", addr, err)
	}
	log.Println("connected to redis")
	return rdb
}
