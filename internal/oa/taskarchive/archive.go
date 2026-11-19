package taskarchive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"

	"workflow/domain"
	"workflow/internal/oa/platform"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

const Action = "task.archive"
const maxBatch = 100

// GlobalAllowed checks the official assignment in addition to the core's
// identity response, which labels a scoped super_admin as an administrator too.
func GlobalAllowed(ctx context.Context, source *sql.DB, actor platform.Actor) bool {
	if actor.ID <= 0 || !actor.SuperAdmin || !actor.Actions[string(domain.PermissionTaskView)] {
		return false
	}
	var allowed bool
	err := source.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM auth_user_role_assignments a
			JOIN auth_roles r ON r.id = a.role_id AND r.code = 'super_admin' AND r.archived_at IS NULL
			JOIN auth_role_permissions p ON p.role_id = r.id AND p.permission_code = 'task.view'
			WHERE a.user_id = ? AND a.scope_mode = 'global'
		)`, actor.ID).Scan(&allowed)
	if err != nil {
		log.Printf("oa_task_archive_scope_failed trace_id=%s error_type=%T", platform.TraceID(ctx), err)
	}
	return err == nil && allowed
}

type archiveHandler struct {
	db     *mysqlrepo.DB
	tasks  repo.TaskAutoArchiveRepo
	events repo.TaskEventRepo
}

func New(db *sql.DB) http.Handler {
	wrapped := mysqlrepo.New(db)
	return archiveHandler{wrapped, mysqlrepo.NewTaskAutoArchiveRepo(wrapped), mysqlrepo.NewAppendOnlyTaskEventRepo(wrapped)}
}

type request struct {
	TaskIDs []int64 `json:"task_ids"`
}

type result struct {
	Archived        int `json:"archived"`
	AlreadyArchived int `json:"already_archived"`
}

func (h archiveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if actor.ID <= 0 || !actor.Actions[Action] {
		platform.Error(w, http.StatusForbidden, "当前账号没有归档任务的权限")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	var input request
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF ||
		len(input.TaskIDs) == 0 || len(input.TaskIDs) > maxBatch {
		platform.Error(w, http.StatusBadRequest, "请选择 1 至 100 条任务")
		return
	}
	slices.Sort(input.TaskIDs)
	ids := slices.Compact(input.TaskIDs)
	if ids[0] <= 0 || len(ids) != len(input.TaskIDs) {
		platform.Error(w, http.StatusBadRequest, "任务编号无效或重复")
		return
	}

	out, err := h.archive(r.Context(), actor.ID, ids)
	switch {
	case err == nil:
		platform.JSON(w, http.StatusOK, out)
	case errors.Is(err, sql.ErrNoRows):
		platform.Error(w, http.StatusNotFound, "有任务已不存在，请刷新列表后重新选择")
	case errors.Is(err, repo.ErrConflict):
		platform.Error(w, http.StatusConflict, "只可归档已结单或已取消的任务，请刷新列表后重新选择")
	default:
		log.Printf("oa_task_archive_failed trace_id=%s error_type=%T", platform.TraceID(r.Context()), err)
		platform.Error(w, http.StatusServiceUnavailable, "批量归档暂未确认，请刷新列表后重试")
	}
}

type candidate struct {
	id     int64
	status domain.TaskStatus
}

func (h archiveHandler) archive(ctx context.Context, actorID int64, ids []int64) (result, error) {
	var out result
	err := h.db.RunInTx(ctx, func(tx repo.Tx) error {
		pending := make([]candidate, 0, len(ids))
		pendingIDs := make([]int64, 0, len(ids))
		for _, id := range ids {
			var status domain.TaskStatus
			if err := mysqlrepo.Unwrap(tx).QueryRowContext(ctx, "SELECT task_status FROM tasks WHERE id = ? FOR UPDATE", id).Scan(&status); err != nil {
				return err
			}
			switch status {
			case domain.TaskStatusCompleted, domain.TaskStatusCancelled:
				pending = append(pending, candidate{id: id, status: status})
				pendingIDs = append(pendingIDs, id)
			case domain.TaskStatusArchived:
				out.AlreadyArchived++
			default:
				return repo.ErrConflict
			}
		}
		n, err := h.tasks.ArchiveTasks(ctx, tx, pendingIDs)
		if err != nil {
			return err
		}
		if n != len(pending) {
			return fmt.Errorf("archive updated %d of %d locked tasks", n, len(pending))
		}
		for _, task := range pending {
			_, err := h.events.Append(ctx, tx, task.id, domain.TaskEventStatusChanged, &actorID, map[string]string{
				"from_task_status": string(task.status),
				"to_task_status":   string(domain.TaskStatusArchived),
				"source":           "oa_manual_archive",
			})
			if err != nil {
				return err
			}
		}
		out.Archived = n
		return nil
	})
	return out, err
}
