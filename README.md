# ratelimiter
A distributed token-bucket-based rate limiter ,using Redis.

##Quickstart

    
   
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

	if err := limiter.WaitN(context.Background(), 5); err != nil {
    	panic(err)
    }
   
##References
  https://godoc.org/golang.org/x/time/rate
