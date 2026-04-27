package service

import (
	"context"
	"strconv"

	"workflow/domain"
)

// callerFromCtx returns the numeric actor ID for audit fields.
// Session-backed request actors win first, then legacy context keys, then any
// remaining request actor, and finally system actor 1 for non-request contexts.
func callerFromCtx(ctx context.Context) int64 {
	if actor, ok := domain.RequestActorFromContext(ctx); ok && domain.IsSessionBackedRequestActor(actor) {
		return actor.ID
	}
	for _, key := range []string{"caller_id", "user_id", "actor_id"} {
		v := ctx.Value(key)
		switch tv := v.(type) {
		case int64:
			if tv > 0 {
				return tv
			}
		case int:
			if tv > 0 {
				return int64(tv)
			}
		case string:
			if parsed, err := strconv.ParseInt(tv, 10, 64); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	if actor, ok := domain.RequestActorFromContext(ctx); ok && actor.ID > 0 {
		return actor.ID
	}
	return 1
}
