package aiagent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisAIRateLimitScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local max_calls = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count >= max_calls then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local reset_at = now + window
  if oldest[2] then
    reset_at = tonumber(oldest[2]) + window
  end
  redis.call('PEXPIRE', key, window)
  return {0, count, reset_at}
end

redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, window)
return {1, count + 1, now + window}
`

type RedisAIRateLimiter struct {
	client redis.Cmdable
	prefix string
	script *redis.Script
}

func NewRedisAIRateLimiter(client redis.Cmdable, prefix string) AIRateLimiter {
	if client == nil {
		return nil
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "omp"
	}
	return &RedisAIRateLimiter{
		client: client,
		prefix: normalizeRateLimitPart(prefix),
		script: redis.NewScript(redisAIRateLimitScript),
	}
}

func (l *RedisAIRateLimiter) Reserve(ctx context.Context, key string, window time.Duration, maxCalls int) (AIRateLimitReservation, error) {
	if l == nil || l.client == nil {
		return AIRateLimitReservation{}, fmt.Errorf("redis rate limiter is not configured")
	}
	if window <= 0 {
		return AIRateLimitReservation{}, fmt.Errorf("rate limit window must be positive")
	}
	if maxCalls <= 0 {
		return AIRateLimitReservation{}, fmt.Errorf("rate limit max calls must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now()
	nowMS := now.UnixMilli()
	windowMS := window.Milliseconds()
	if windowMS <= 0 {
		windowMS = 1
	}
	redisKey := "aiagent:rate:" + l.prefix + ":" + normalizeRateLimitPart(key)
	member := strconv.FormatInt(nowMS, 10) + ":" + strconv.FormatInt(now.UnixNano(), 10)
	result, err := l.script.Run(ctx, l.client, []string{redisKey}, nowMS, windowMS, maxCalls, member).Result()
	if err != nil {
		return AIRateLimitReservation{}, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 3 {
		return AIRateLimitReservation{}, fmt.Errorf("unexpected redis rate limit result: %T", result)
	}
	allowedRaw, err := redisRateLimitInt(values[0])
	if err != nil {
		return AIRateLimitReservation{}, fmt.Errorf("parse redis allowed flag: %w", err)
	}
	count, err := redisRateLimitInt(values[1])
	if err != nil {
		return AIRateLimitReservation{}, fmt.Errorf("parse redis count: %w", err)
	}
	resetMS, err := redisRateLimitInt64(values[2])
	if err != nil {
		return AIRateLimitReservation{}, fmt.Errorf("parse redis reset time: %w", err)
	}
	return AIRateLimitReservation{
		Allowed: allowedRaw == 1,
		Count:   count,
		ResetAt: time.UnixMilli(resetMS),
	}, nil
}

func redisRateLimitInt(value interface{}) (int, error) {
	number, err := redisRateLimitInt64(value)
	return int(number), err
}

func redisRateLimitInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported value %T", value)
	}
}
