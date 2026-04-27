# V1.2-D-1 · Task Detail Fallback Removal Report

## §1 Meta

- date: 2026-04-27T06:02:41Z
- terminator: `V1_2_D_1_FALLBACK_REMOVED`
- scope: remove `TaskDetailHandler` legacy fallback response path; no OpenAPI/frontend/route change

## §2 Baseline And Final SHA

```text
704aaa07165996b2a3cf5681d823debd51f492e662e341d22b36041a60044df9  transport/handler/task_detail.go
61a52019eaa506d051742cf5a7f912ff9821d3d6b1e8428dc3e940969d25a831  cmd/server/main.go
6dc899e4a83dac480eea272fb5cdcef94678c90d4aef847146173029a1a236fa  cmd/api/main.go
80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f  docs/api/openapi.yaml
41858a1fe4f9398ba640e28a9cb7ba6fadd58214fe9d065c45ac254117039be9  docs/frontend/V1_API_TASKS.md
9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396  transport/http.go
315aef20dc7e34ad3233bf8f3e6bf8ae8e7477103586856d494a8c9e62bb82f0  domain/task_detail_aggregate.go
6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e  service/task_aggregator/detail_aggregator.go
```

## §3 Handler Before/After

- before: `TaskDetailHandler` had `svc service.TaskDetailAggregateService`, `SetR3DetailService`, fast-path branch, and fallback `h.svc.GetByTaskID` response.
- after: handler has only `r3Svc *task_aggregator.DetailService`; `r3Svc == nil` is an internal error; success response is typed as `*task_aggregator.Detail` and returns the 5-section schema.
- line count after: 41.

## §4 cmd/server main.go

- `handler.NewTaskDetailHandler(taskDetailSvc)` + `SetR3DetailService(r3DetailSvc)` replaced with `handler.NewTaskDetailHandler(r3DetailSvc)`.
- `taskDetailSvc := service.NewTaskDetailAggregateService(...)` retained because `TaskHandler` still uses it.

## §5 cmd/api main.go

- `handler.NewTaskDetailHandler(taskDetailSvc)` + `SetR3DetailService(r3DetailSvc)` replaced with `handler.NewTaskDetailHandler(r3DetailSvc)`.
- `taskDetailSvc` retained and wired into `handler.NewTaskHandler(taskSvc, costRuleSvc, taskDetailSvc)` so cmd/api remains buildable and preserves TaskHandler detail dependencies.

## §6 Handler Tests

No task detail handler-specific tests required constructor updates; full `go test ./transport/handler/... -count=1` passed.

## §7 contract_audit Before/After

- before: `summary.drift=72`, `summary.clean=84`, `GET /v1/tasks/:id/detail verdict=both_diff`.
- after: `summary.drift=71`, `summary.clean=85`, detail verdict=`clean`.
- detail code fields: `events, modules, reference_file_refs, task, task_detail`.
- detail OpenAPI fields: `events, modules, reference_file_refs, task, task_detail`.
- detail only_in_code=0, only_in_openapi=0.

## §8 Verify Matrix

| # | check | result |
|---|---|---|
| 1 | baseline SHA except allowed files | PASS |
| 2 | `go vet ./...` | PASS |
| 3 | `go build ./...` | PASS |
| 4 | `go test ./tools/contract_audit/... -count=1` | PASS |
| 5 | `go test ./transport/handler/... -count=1` | PASS |
| 6 | `go test ./service/task_aggregator/... -count=1` | PASS |
| 7 | `go test ./... -count=1` | PASS |
| 8 | contract_audit run to `tmp/v1_2_d_1_audit.json` | PASS |
| 9 | detail verdict | PASS · clean |
| 10 | detail code_fields | PASS · 5 sections |
| 11 | detail only_in_code + only_in_openapi | PASS · 0 + 0 |
| 12 | summary.drift | PASS · 71 |
| 13 | summary.clean | PASS · 85 |
| 14 | OpenAPI SHA unchanged | PASS |
| 15 | frontend TASKS SHA unchanged | PASS |
| 16 | transport/http.go SHA unchanged | PASS |
| 17 | cmd/server diff scope | PASS · constructor injection only |

## §9 Diff Summary

```diff
diff --git a/cmd/api/main.go b/cmd/api/main.go
index 8e3121d..d9385ce 100644
--- a/cmd/api/main.go
+++ b/cmd/api/main.go
@@ -272,7 +272,7 @@ func main() {
 	categoryMappingH := handler.NewCategoryERPMappingHandler(categoryMappingSvc)
 	costRuleH := handler.NewCostRuleHandler(costRuleSvc)
 	erpSyncH := handler.NewERPSyncHandler(erpSyncSvc)
-	taskH := handler.NewTaskHandler(taskSvc, nil, nil)
+	taskH := handler.NewTaskHandler(taskSvc, costRuleSvc, taskDetailSvc)
 	taskH.SetR3Services(r3PoolQuerySvc, r3ClaimSvc, r3ModuleSvc, r3CancelSvc)
 	taskAssignmentH := handler.NewTaskAssignmentHandler(taskAssignmentSvc)
 	taskAssetH := handler.NewTaskAssetHandler(taskAssetSvc)
@@ -282,8 +282,7 @@ func main() {
 	assetUploadH := handler.NewAssetUploadHandler(assetUploadSvc)
 	assetFilesH := handler.NewAssetFilesHandler(cfg.UploadService.BaseURL, cfg.UploadService.InternalToken, cfg.UploadService.StorageProvider, logger)
 	designSubmissionH := handler.NewDesignSubmissionHandler(taskAssetSvc, taskAssetCenterSvc, taskSvc)
-	taskDetailH := handler.NewTaskDetailHandler(taskDetailSvc)
-	taskDetailH.SetR3DetailService(r3DetailSvc)
+	taskDetailH := handler.NewTaskDetailHandler(r3DetailSvc)
 	taskCostOverrideH := handler.NewTaskCostOverrideHandler(taskCostOverrideSvc)
 	taskBoardH := handler.NewTaskBoardHandler(taskBoardSvc)
 	workbenchH := handler.NewWorkbenchHandler(workbenchSvc)
diff --git a/cmd/server/main.go b/cmd/server/main.go
index 92372b3..59dac05 100644
--- a/cmd/server/main.go
+++ b/cmd/server/main.go
@@ -346,8 +346,7 @@ func main() {
 	assetUploadH := handler.NewAssetUploadHandler(assetUploadSvc)
 	assetFilesH := handler.NewAssetFilesHandler(cfg.UploadService.BaseURL, cfg.UploadService.InternalToken, cfg.UploadService.StorageProvider, logger)
 	designSubmissionH := handler.NewDesignSubmissionHandler(taskAssetSvc, taskAssetCenterSvc, taskSvc)
-	taskDetailH := handler.NewTaskDetailHandler(taskDetailSvc)
-	taskDetailH.SetR3DetailService(r3DetailSvc)
+	taskDetailH := handler.NewTaskDetailHandler(r3DetailSvc)
 	taskCostOverrideH := handler.NewTaskCostOverrideHandler(taskCostOverrideSvc)
 	taskBoardH := handler.NewTaskBoardHandler(taskBoardSvc)
 	workbenchH := handler.NewWorkbenchHandler(workbenchSvc)
diff --git a/transport/handler/task_detail.go b/transport/handler/task_detail.go
index 0122b27..27c91ba 100644
--- a/transport/handler/task_detail.go
+++ b/transport/handler/task_detail.go
@@ -4,47 +4,38 @@ import (
 	"github.com/gin-gonic/gin"
 
 	"workflow/domain"
-	"workflow/service"
 	"workflow/service/task_aggregator"
 )
 
 type TaskDetailHandler struct {
-	svc   service.TaskDetailAggregateService
 	r3Svc *task_aggregator.DetailService
 }
 
-func NewTaskDetailHandler(svc service.TaskDetailAggregateService) *TaskDetailHandler {
-	return &TaskDetailHandler{svc: svc}
-}
-
-func (h *TaskDetailHandler) SetR3DetailService(svc *task_aggregator.DetailService) {
-	h.r3Svc = svc
+func NewTaskDetailHandler(r3Svc *task_aggregator.DetailService) *TaskDetailHandler {
+	return &TaskDetailHandler{r3Svc: r3Svc}
 }
 
 // GetByTaskID handles GET /v1/tasks/:id/detail
+// 返回 V1.1-A1 fast-path 5 段 schema(task / task_detail / modules / events / reference_file_refs).
 func (h *TaskDetailHandler) GetByTaskID(c *gin.Context) {
 	taskID, err := parseID(c)
 	if err != nil {
 		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
 		return
 	}
-	if h.r3Svc != nil {
-		aggregate, err := h.r3Svc.Get(c.Request.Context(), taskID)
-		if err != nil {
-			respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
-			return
-		}
-		if aggregate == nil {
-			respondError(c, domain.ErrNotFound)
-			return
-		}
-		respondOK(c, aggregate)
+	if h.r3Svc == nil {
+		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task detail aggregate service not configured", nil))
+		return
+	}
+	aggregate, err := h.r3Svc.Get(c.Request.Context(), taskID)
+	if err != nil {
+		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
 		return
 	}
-	aggregate, appErr := h.svc.GetByTaskID(c.Request.Context(), taskID)
-	if appErr != nil {
-		respondError(c, appErr)
+	if aggregate == nil {
+		respondError(c, domain.ErrNotFound)
 		return
 	}
-	respondOK(c, aggregate)
+	var detail *task_aggregator.Detail = aggregate
+	respondOK(c, detail)
 }
```

terminator: `V1_2_D_1_FALLBACK_REMOVED`
