package mysqlrepo

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestHydrateResourceGroupRevisionsUsesOneBatchPerRelation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewTaskResourceGroupRepo(New(db))
	now := time.Now().UTC()

	mock.ExpectQuery(`FROM task_asset_group_revisions r[\s\S]+JOIN task_asset_groups g ON g\.id = r\.group_id[\s\S]+WHERE r\.id IN \(\?,\?\)`).
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows(revisionHydrationColumns()).
			AddRow(101, 1, 1, "finalized", "single", 201, "design", 9, "审核员", "", now, now, now, 1001, "task", nil, nil).
			AddRow(102, 2, 1, "finalized", "set", 202, "audit", 9, "审核员", "", now, now, now, 1002, "task", nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta("FROM task_asset_group_revision_items\n\t\tWHERE revision_id IN (?,?)")).
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}).
			AddRow(301, 101, 211, 0, "正面", now).
			AddRow(302, 102, 212, 0, "正面", now).
			AddRow(303, 102, 213, 1, "侧面", now))
	mock.ExpectQuery(regexp.QuoteMeta("FROM task_asset_group_revision_references rr")).
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows(revisionReferenceHydrationColumns()))
	mock.ExpectQuery(regexp.QuoteMeta("FROM task_assets ta\n\t\t\tLEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id\n\t\t\tWHERE ta.id IN (?,?,?,?,?)")).
		WithArgs(int64(201), int64(202), int64(211), int64(212), int64(213)).
		WillReturnRows(sqlmock.NewRows(revisionFileHydrationColumns()).
			AddRow(201, "A.psd", "image/vnd.adobe.photoshop", 10, "a.psd", "recorded", true, 1001, "source", 1, "source").
			AddRow(202, "B.psd", "image/vnd.adobe.photoshop", 10, "b.psd", "recorded", true, 1002, "source", 2, "source").
			AddRow(211, "A.png", "image/png", 10, "a.png", "recorded", true, 1001, "delivery", 1, "final").
			AddRow(212, "B-1.png", "image/png", 10, "b1.png", "recorded", true, 1002, "delivery", 2, "final").
			AddRow(213, "B-2.png", "image/png", 10, "b2.png", "recorded", true, 1002, "delivery", 2, "final"))

	groups := []domain.TaskAssetGroup{
		{ID: 1, FinalizedRevisionID: int64Ptr(101)},
		{ID: 2, FinalizedRevisionID: int64Ptr(102)},
	}
	if err := repository.hydrateResourceGroupRevisions(context.Background(), groups); err != nil {
		t.Fatalf("hydrateResourceGroupRevisions() error = %v", err)
	}
	if groups[0].FinalizedRevision == nil || len(groups[0].FinalizedRevision.Items) != 1 || groups[0].FinalizedRevision.SourceFile == nil {
		t.Fatalf("first group was not fully hydrated: %#v", groups[0].FinalizedRevision)
	}
	if groups[1].FinalizedRevision == nil || len(groups[1].FinalizedRevision.Items) != 2 || groups[1].FinalizedRevision.SourceStage != domain.TaskAssetSourceAudit {
		t.Fatalf("second group was not fully hydrated: %#v", groups[1].FinalizedRevision)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func int64Ptr(value int64) *int64 { return &value }
