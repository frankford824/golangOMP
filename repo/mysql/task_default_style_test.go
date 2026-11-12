package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestDefaultStyleBindingUsesExactScopeAndCommitsPointer(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "pending audit"
		if completed {
			name = "completed planning"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			d := New(db)
			repository := NewTaskResourceGroupRepo(d)
			sku, asset, version := int64(7), int64(100), int64(3)
			group := domain.TaskAssetGroup{ID: 4, TaskID: 5, TaskSKUItemID: &sku, ScopeKind: domain.TaskAssetGroupScopeSKU, LockVersion: version}
			mock.ExpectBegin()
			mock.ExpectExec(`UPDATE task_assets ta JOIN asset_storage_refs[\s\S]+ta.staged_by = [\s\S]+ta.staged_task_sku_item_id <=> [\s\S]+ta.staged_retouch_requirement_id <=>`).
				WithArgs(group.ID, asset, group.TaskID, int64(9), sku, nil).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO task_default_style_revisions").
				WithArgs(group.ID, asset, int64(9), "确认默认款").WillReturnResult(sqlmock.NewResult(200, 1))
			mock.ExpectExec(`UPDATE task_asset_groups SET lock_version[\s\S]+current_default_style_id[\s\S]+pending_default_style_id`).
				WithArgs(completed, int64(200), completed, int64(200), group.ID, version).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			err = d.RunInTx(context.Background(), func(tx repo.Tx) error {
				id, err := repository.CreateDefaultStyleRevision(context.Background(), tx, group, domain.DefaultStyleUpdate{
					TaskAssetID: &asset, ExpectedGroupLockVersion: &version, Reason: "确认默认款",
				}, 9, completed)
				if err == nil && id != 200 {
					t.Errorf("revision id=%d, want 200", id)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
