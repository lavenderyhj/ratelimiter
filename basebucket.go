package ratelimiter

import (
	"sync"
)

// Config is the bucket configuration.
type Config struct {
	// Limit defines the maximum frequency of some events.
	// Limit is represented as number of events per second.
	// A zero Limit allows no events.
	Limit Limit

	// the capacity of the bucket
	Capacity int64
}

// baseBucket is a basic structure both for Limiter and LeakyBucket.
type baseBucket struct {
	mu     sync.RWMutex
	config *Config
}

// Config returns the bucket configuration in a concurrency-safe way.
func (b *baseBucket) Config() Config {
	b.mu.RLock()
	config := *b.config
	b.mu.RUnlock()
	return config
}

// SetConfig updates the bucket configuration in a concurrency-safe way.
func (b *baseBucket) SetConfig(config *Config) {
	b.mu.Lock()
	b.config = config
	b.mu.Unlock()
}
