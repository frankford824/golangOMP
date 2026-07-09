package mysqlrepo

import (
	"context"
	"time"
)

const mysqlReadQueryTimeout = 3 * time.Second

func mysqlReadQueryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= mysqlReadQueryTimeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, mysqlReadQueryTimeout)
}
