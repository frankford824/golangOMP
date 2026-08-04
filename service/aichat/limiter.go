package aichat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrConcurrentLimit = errors.New("ai chat concurrent stream limit reached")

type StreamLimiter interface {
	Acquire(ctx context.Context, userID int64) (func(context.Context), error)
}

type RedisStreamLimiter struct {
	redis     *redis.Client
	prefix    string
	globalMax int
	userMax   int
	leaseTTL  time.Duration
}

func NewRedisStreamLimiter(client *redis.Client, prefix string, globalMax, userMax int, leaseTTL time.Duration) *RedisStreamLimiter {
	if globalMax <= 0 {
		globalMax = 20
	}
	if userMax <= 0 {
		userMax = 2
	}
	if leaseTTL <= 0 {
		leaseTTL = 2 * time.Minute
	}
	if prefix == "" {
		prefix = "omp:ai-chat"
	}
	return &RedisStreamLimiter{redis: client, prefix: prefix, globalMax: globalMax, userMax: userMax, leaseTTL: leaseTTL}
}

func (l *RedisStreamLimiter) Acquire(ctx context.Context, userID int64) (func(context.Context), error) {
	if l == nil || l.redis == nil || userID <= 0 {
		return nil, errors.New("ai chat stream limiter is unavailable")
	}
	token := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(l.leaseTTL).UnixMilli()
	keys := []string{l.prefix + ":global", l.prefix + ":user:" + strconv.FormatInt(userID, 10)}
	result, err := l.redis.Eval(ctx, acquireStreamLua, keys,
		now.UnixMilli(), expiresAt, token, l.globalMax, l.userMax, int64(l.leaseTTL/time.Millisecond)).Int()
	if err != nil {
		return nil, fmt.Errorf("acquire ai chat stream lease: %w", err)
	}
	if result != 1 {
		return nil, ErrConcurrentLimit
	}
	return func(releaseCtx context.Context) {
		_, _ = l.redis.Eval(releaseCtx, releaseStreamLua, keys, token).Result()
	}, nil
}

const acquireStreamLua = `
local now = tonumber(ARGV[1])
local expires = tonumber(ARGV[2])
local token = ARGV[3]
local globalMax = tonumber(ARGV[4])
local userMax = tonumber(ARGV[5])
local ttl = tonumber(ARGV[6])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', now)
if redis.call('ZCARD', KEYS[1]) >= globalMax or redis.call('ZCARD', KEYS[2]) >= userMax then
  return 0
end
redis.call('ZADD', KEYS[1], expires, token)
redis.call('ZADD', KEYS[2], expires, token)
redis.call('PEXPIRE', KEYS[1], ttl)
redis.call('PEXPIRE', KEYS[2], ttl)
return 1
`

const releaseStreamLua = `
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
return 1
`
