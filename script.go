package ratelimiter

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
)

// the Lua script that implements the Token Bucket Algorithm.
// bucket.tc represents the token count.
// bucket.ts represents the timestamp of the last time the bucket was refilled.
const luaTokenBucket = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local amount = tonumber(ARGV[4])
local reservation = {ok=false ,tokens= 0,timeToAct = now,update = false}
local bucket = {tc=capacity, ts=now}
local mill = 1000

local value = redis.call("get", key)

if value then
  bucket = cjson.decode(value)
end

if now < bucket.ts then
   bucket.ts = now
end

local maxElapsed = math.floor((capacity - bucket.tc) * mill / limit)
local elapsed = now - bucket.ts
if elapsed > maxElapsed then
  elapsed = maxElapsed
end

local delta = math.floor(elapsed * limit / mill)
local tokens = bucket.tc + delta

if tokens > capacity then
  tokens = capacity
end

tokens = tokens - amount

local waitDuration = 0
if tokens < 0 then 
  waitDuration = math.floor(-tokens * mill / limit)
  reservation.timeToAct = now + waitDuration
else 
  reservation.ok = true
  reservation.tokens = amount
  
  bucket.ts = now
  bucket.tc = tokens
  
  reservation.ts = bucket.ts
  reservation.tc = bucket.tc
  
  if redis.call("set", key, cjson.encode(bucket)) then
    reservation.update = true
  end

end

local result = cjson.encode(reservation)

return result
`

type Redis interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error, bool)
}

type Script struct {
	redis Redis
	src   string
	hash  string
}

func NewScript(redis Redis, src string) *Script {
	h := sha1.New()
	io.WriteString(h, src)
	return &Script{
		redis: redis,
		src:   src,
		hash:  hex.EncodeToString(h.Sum(nil)),
	}
}

func (s *Script) Run(ctx context.Context, keys []string, args ...interface{}) (interface{}, error) {
	result, err, noScript := s.redis.EvalSha(ctx, s.hash, keys, args...)
	if noScript {
		result, err = s.redis.Eval(ctx, s.src, keys, args...)
	}
	return result, err
}
