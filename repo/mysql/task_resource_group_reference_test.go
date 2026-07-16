package mysqlrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestCreateRevisionConstrainsRetouchReferencesToTheSameRequirement(t *testing.T) {
	tests := []struct {
		name          string
		referenceRows int64
		wantErr       error
	}{
		{name: "matching retouch requirement", referenceRows: 1},
		{name: "different retouch requirement", referenceRows: 0, wantErr: repo.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
				for _, token := range strings.Split(expected, "&&") {
					if !strings.Contains(actual, token) {
						return fmt.Errorf("query missing %q: %s", token, actual)
					}
				}
				return nil
			})))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			database := New(db)
			repository := NewTaskResourceGroupRepo(database)
			mock.ExpectBegin()
			wrapped, sqlTx, err := database.BeginTx(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery("SELECT COALESCE(MAX(revision_no)&&task_asset_group_revisions&&FOR UPDATE").
				WithArgs(int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"revision_no"}).AddRow(1))
			mock.ExpectExec("INSERT INTO task_asset_group_revisions").
				WillReturnResult(sqlmock.NewResult(20, 1))
			mock.ExpectExec("INSERT INTO task_asset_group_revision_references&&retouch_requirement_id IS NOT NULL&&CONCAT('retouch_requirement:'&&rfr.sku_item_id IS NULL&&(rfr.retouch_requirement_id IS NULL OR rfr.retouch_requirement_id = ?)").
				WithArgs(int64(20), 0, int64(30), int64(40), int64(50)).
				WillReturnResult(sqlmock.NewResult(0, tt.referenceRows))
			if tt.wantErr == nil {
				mock.ExpectExec("UPDATE task_asset_groups SET working_revision_id").
					WithArgs(int64(20), int64(10), int64(0)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}

			retouchRequirementID := int64(50)
			revisionID, gotErr := repository.CreateRevision(context.Background(), wrapped, domain.TaskAssetGroup{
				ID: 10, TaskID: 40, ScopeKind: domain.TaskAssetGroupScopeRetouch,
				RetouchRequirementID: &retouchRequirementID,
			}, domain.SubmitResourceGroupInput{
				Mode: domain.TaskAssetGroupModeSingle, ReferenceFileRefIDs: []int64{30},
			}, domain.TaskAssetGroupRevisionSubmitted, domain.TaskAssetSourceRetouch, 60, "submit retouch")
			if tt.wantErr != nil {
				if !errors.Is(gotErr, tt.wantErr) {
					t.Fatalf("CreateRevision() error = %v, want %v", gotErr, tt.wantErr)
				}
				mock.ExpectRollback()
				if err := sqlTx.Rollback(); err != nil {
					t.Fatal(err)
				}
			} else {
				if gotErr != nil || revisionID != 20 {
					t.Fatalf("CreateRevision() revision/error = %d/%v", revisionID, gotErr)
				}
				if err := sqlTx.Commit(); err != nil {
					t.Fatal(err)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
