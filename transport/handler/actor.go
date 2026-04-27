package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func actorIDOrRequestValue(c *gin.Context, raw *int64, fieldName string) (int64, *domain.AppError) {
	if raw != nil && *raw > 0 {
		return *raw, nil
	}
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if ok && actor.ID > 0 {
		if meta, metaOK := domain.RouteAccessMetaFromContext(c.Request.Context()); metaOK && domain.CanImplicitlyUseActorID(actor, meta) {
			return actor.ID, nil
		}
		if domain.IsSessionBackedRequestActor(actor) {
			return actor.ID, nil
		}
	}
	return 0, domain.NewAppError(domain.ErrCodeUnauthorized, fieldName+" is required unless a session-backed actor is present", nil)
}
