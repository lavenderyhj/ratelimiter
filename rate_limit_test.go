package ratelimiter

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"testing"

	"time"
)

func TestRedis(t *testing.T) {
	key := "ratelimiter:tokenbucket:test"

	redisClient := &RedisClient{
		redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
	}
	limiter := NewLimiter(redisClient, key, &Config{
		Limit:    1,
		Capacity: 10,
	})
	for i := 0; i < 100; i++ {

		if err := limiter.WaitN(context.Background(), 5); err != nil {
			panic(err)
		}

		fmt.Printf("%s, \n", time.Now())
	}

}
