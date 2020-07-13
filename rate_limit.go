package ratelimiter

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	json "github.com/json-iterator/go"
	"math"
	"strings"
	"time"
)

// Limit defines the maximum frequency of some events.
// Limit is represented as number of events per second.
// A zero Limit allows no events.
type Limit float64

// Inf is the infinite rate limit; it allows all events (even if burst is zero).
const Inf = Limit(math.MaxFloat64)

// Every converts a minimum time interval between events to a Limit.
func Every(interval time.Duration) Limit {
	if interval <= 0 {
		return Inf
	}
	return 1 / Limit(interval.Seconds())
}

// Limiter implements the Token Bucket Algorithm.
// See https://en.wikipedia.org/wiki/Token_bucket.
type Limiter struct {
	baseBucket

	script *Script
	key    string
}

// NewLimiter returns a new token-bucket rate limiter special for key in redis
// with the specified bucket configuration.
func NewLimiter(redis Redis, key string, config *Config) *Limiter {
	return &Limiter{
		baseBucket: baseBucket{config: config},
		script:     NewScript(redis, luaTokenBucket),
		key:        key,
	}
}

type Reservation struct {
	OK        bool
	Tokens    int64
	TimeToAct int64 `json:"timeToAct"`
	Update    bool
}

// Take takes amount tokens from the bucket.
func (b *Limiter) TakeN(ctx context.Context, n int64) (bool, error) {

	result, err := b.reserveN(ctx, time.Now(), n)
	if err != nil || result.Update == false {
		return false, err
	}
	return result.OK, nil
}
func (b *Limiter) WaitN(ctx context.Context, n int64) (err error) {
	if n > b.config.Capacity {
		return fmt.Errorf("rate: Wait(n=%d) exceeds limiter's burst %d", n, b.config.Capacity)
	}
	// Check if ctx is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Determine wait limit
	now := time.Now()
	// Reserve
	r, reserveErr := b.reserveN(ctx, now, n)
	if reserveErr != nil || r.OK {
		return reserveErr
	}

	// Wait if necessary
	delay := r.DelayFrom(now)
	if delay == 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		// We can proceed.
		return b.WaitN(ctx, n)
	case <-ctx.Done():
		// Context was canceled before we could proceed.  Cancel the
		// reservation, which may permit other events to proceed sooner.
		return ctx.Err()
	}
}

func (r *Reservation) DelayFrom(now time.Time) time.Duration {
	nano := time.Duration(r.TimeToAct) * time.Millisecond
	delay := time.Unix(0, int64(nano)).Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}
func (b *Limiter) reserveN(ctx context.Context, now time.Time, n int64) (Reservation, error) {

	config := b.Config()
	if n > config.Capacity {
		return Reservation{}, fmt.Errorf("amount is larger than capacity")
	}
	b.mu.Lock()
	defer func() {
		b.mu.Unlock()
	}()
	if config.Limit == Inf {
		return Reservation{
			OK:        true,
			Tokens:    n,
			TimeToAct: int64(time.Duration(now.UnixNano()) / time.Millisecond),
			Update:    true,
		}, nil
	}
	result, err := b.script.Run(ctx,
		[]string{b.key},
		float64(config.Limit),
		config.Capacity,
		int64(time.Duration(now.UnixNano())/time.Millisecond),
		n,
	)
	reservation := Reservation{}
	if err != nil {
		return reservation, err
	}
	resultStr := result.(string)
	if err = json.Unmarshal([]byte(resultStr), &reservation); err != nil {
		println(err)
		return reservation, err
	}
	return reservation, nil
}

type RedisClient struct {
	Client *redis.Client
}

func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.Client.Eval(ctx, script, keys, args...).Result()
}

func (r *RedisClient) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error, bool) {
	result, err := r.Client.EvalSha(ctx, sha1, keys, args...).Result()
	noScript := err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT ")
	return result, err, noScript
}
