package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInt64ChunksSortsDeduplicatesAndBoundsChunks(t *testing.T) {
	values := make([]int64, 0, workflowGroupsBulkChunkSize+3)
	for value := int64(workflowGroupsBulkChunkSize + 1); value >= 1; value-- {
		values = append(values, value)
	}
	values = append(values, 1, 0, -1)
	chunks := int64Chunks(values)
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	if len(chunks[0]) != workflowGroupsBulkChunkSize || len(chunks[1]) != 1 {
		t.Fatalf("chunk lengths = %d,%d", len(chunks[0]), len(chunks[1]))
	}
	if chunks[0][0] != 1 || chunks[0][workflowGroupsBulkChunkSize-1] != workflowGroupsBulkChunkSize || chunks[1][0] != workflowGroupsBulkChunkSize+1 {
		t.Fatalf("chunks are not globally sorted and deduplicated: first=%d boundary=%d last=%d", chunks[0][0], chunks[0][workflowGroupsBulkChunkSize-1], chunks[1][0])
	}
}

func TestValidateAndLockPlanningImagesRequiresExactUniqueReviewedCandidate(t *testing.T) {
	tests := []struct {
		name      string
		rows      *sqlmock.Rows
		wantError bool
	}{
		{
			name: "exact unique candidate",
			rows: sqlmock.NewRows([]string{"id", "storage_ref_id"}).
				AddRow(900, "ref-product-70"),
		},
		{
			name: "reviewed ref drifted",
			rows: sqlmock.NewRows([]string{"id", "storage_ref_id"}).
				AddRow(900, "different-ref"),
			wantError: true,
		},
		{
			name: "ambiguous candidates",
			rows: sqlmock.NewRows([]string{"id", "storage_ref_id"}).
				AddRow(900, "ref-product-70").
				AddRow(901, "other-ref"),
			wantError: true,
		},
		{
			name:      "candidate disappeared",
			rows:      sqlmock.NewRows([]string{"id", "storage_ref_id"}),
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery("SELECT ta.id,ta.storage_ref_id.*FOR UPDATE").
				WithArgs(int64(701), int64(70), int64(70)).
				WillReturnRows(tt.rows)
			mapping := []planningMapping{{
				TaskID: 70,
				Items: []planningItemMapping{{
					TaskSKUItemID:   701,
					ImageStorageRef: "ref-product-70",
				}},
			}}
			err = validateAndLockPlanningImages(context.Background(), tx, mapping)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateAndLockPlanningImages() error = %v, wantError %v", err, tt.wantError)
			}
			mock.ExpectRollback()
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidatePlanningImagesDryRunUsesSameCandidateContractWithoutWriteLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT ta.id,ta.storage_ref_id").
		WithArgs(int64(701), int64(70), int64(70)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "storage_ref_id"}).
			AddRow(900, "ref-product-70"))
	mapping := []planningMapping{{
		TaskID: 70,
		Items: []planningItemMapping{{
			TaskSKUItemID:   701,
			ImageStorageRef: "ref-product-70",
		}},
	}}
	if err := validatePlanningImages(
		context.Background(),
		db,
		mapping,
		false,
	); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePlanningImagesRejectsUnexpectedCandidateForReviewedEmptyImage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("BINARY TRIM\\(ta.scope_sku_code\\)=BINARY TRIM\\(si.sku_code\\)").
		WithArgs(int64(701), int64(70), int64(70)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "storage_ref_id"}).
			AddRow(900, "unexpected-ref"))
	mapping := []planningMapping{{
		TaskID: 70,
		Items: []planningItemMapping{{
			TaskSKUItemID: 701,
		}},
	}}
	err = validatePlanningImages(context.Background(), db, mapping, false)
	if err == nil {
		t.Fatal("unexpected eligible candidate must invalidate a reviewed empty image")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePlanningImagesFailsClosedOnRowIterationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT ta.id,ta.storage_ref_id").
		WithArgs(int64(701), int64(70), int64(70)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "storage_ref_id"}).
				AddRow(900, "ref-product-70").
				RowError(0, errors.New("driver iteration failed")),
		)
	err = validatePlanningImages(
		context.Background(),
		db,
		[]planningMapping{{
			TaskID: 70,
			Items: []planningItemMapping{{
				TaskSKUItemID:   701,
				ImageStorageRef: "ref-product-70",
			}},
		}},
		false,
	)
	if err == nil {
		t.Fatal("row iteration error must block planning image validation")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTaskSnapshotsBulkPreservesOrderAndEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	updatedAt := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id,task_type,task_status,workflow_revision,current_handler_id,updated_at").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status", "workflow_revision", "current_handler_id", "updated_at"}).
			AddRow(1, "design_task", "Completed", 7, nil, updatedAt).
			AddRow(2, "retouch_task", "InProgress", 3, 9, updatedAt))
	mock.ExpectQuery("SELECT task_id,id.*FROM task_event_logs").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "id"}).
			AddRow(1, "event-a").
			AddRow(1, "event-b").
			AddRow(2, "event-c"))
	mock.ExpectQuery("SELECT tm.task_id,e.id.*FROM task_module_events").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "id"}).
			AddRow(2, 12))

	actual, err := captureTaskSnapshotsBulk(context.Background(), db, []int64{2, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	handlerID := int64(9)
	expected := []taskSnapshot{
		{
			ID: 1, TaskType: "design_task", TaskStatus: "Completed", WorkflowRevision: 7,
			EventIDs: []string{"event-a", "event-b"}, ModuleEventIDs: []int64{}, UpdatedAt: updatedAt,
		},
		{
			ID: 2, TaskType: "retouch_task", TaskStatus: "InProgress", WorkflowRevision: 3, CurrentHandlerID: &handlerID,
			EventIDs: []string{"event-c"}, ModuleEventIDs: []int64{12}, UpdatedAt: updatedAt,
		},
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bulk task snapshot mismatch:\nactual=%#v\nexpected=%#v", actual, expected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSortResourceGroupRevisionIDsDoesNotReorderRevisionHistory(t *testing.T) {
	groups := []resourceGroupSnapshot{{
		ID:          7,
		RevisionIDs: []int64{11, 10},
		Revisions: []resourceRevisionSnapshot{
			{ID: 11, RevisionNo: 1},
			{ID: 10, RevisionNo: 2},
		},
	}}
	sortResourceGroupRevisionIDs(groups)
	if !reflect.DeepEqual(groups[0].RevisionIDs, []int64{10, 11}) {
		t.Fatalf("revision ids = %v, want numeric id order", groups[0].RevisionIDs)
	}
	if got := []int64{groups[0].Revisions[0].ID, groups[0].Revisions[1].ID}; !reflect.DeepEqual(got, []int64{11, 10}) {
		t.Fatalf("revision history order changed: %v", got)
	}
}

func TestCaptureTaskSnapshotsBulkClassifiesMissingExpectedTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,task_type,task_status,workflow_revision,current_handler_id,updated_at").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status", "workflow_revision", "current_handler_id", "updated_at"}).
			AddRow(1, "design_task", "Completed", 1, nil, time.Now().UTC()))
	_, err = captureTaskSnapshotsBulk(context.Background(), db, []int64{1, 2})
	if !errors.Is(err, errBulkSnapshotMissingRows) {
		t.Fatalf("error = %v, want errBulkSnapshotMissingRows", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTaskModulesForTasksPreservesRollbackState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	terminalAt := at.Add(time.Hour)
	mock.ExpectQuery("SELECT id,task_id,module_key,state,pool_team_code,claimed_by").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "module_key", "state", "pool_team_code",
			"claimed_by", "claimed_team_code", "claimed_at",
			"actor_org_snapshot", "entered_at", "terminal_at", "data",
			"updated_at",
		}).
			AddRow(11, 1, "basic_info", "active", nil, nil, nil, nil, nil, at, nil, []byte(`{}`), at).
			AddRow(12, 2, "design", "completed", "design", 9, "design-a", at, []byte(`{"team":1}`), at, terminalAt, []byte(`{"x":1}`), terminalAt))

	items, err := captureTaskModulesForTasks(context.Background(), db, []int64{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != 11 || items[1].ID != 12 {
		t.Fatalf("module snapshots = %#v", items)
	}
	if items[0].ActorOrgSnapshot != nil {
		t.Fatalf("NULL actor snapshot changed: %s", items[0].ActorOrgSnapshot)
	}
	if string(items[1].ActorOrgSnapshot) != `{"team":1}` || string(items[1].Data) != `{"x":1}` {
		t.Fatalf("JSON snapshots changed: %#v", items[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureSearchDocumentsForTasksPreservesProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT group_id,task_id,finalized_revision_id,internal_text,final_text,updated_at").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "task_id", "finalized_revision_id",
			"internal_text", "final_text", "updated_at",
		}).AddRow(7, 3, 9, "internal", "final", at))

	items, err := captureSearchDocumentsForTasks(context.Background(), db, []int64{3})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].FinalizedRevisionID == nil ||
		*items[0].FinalizedRevisionID != 9 || items[0].FinalText != "final" {
		t.Fatalf("search snapshots = %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchDocumentsAfterSnapshotRejectsForwardProjectionDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC)
	revisionID := int64(9)
	expected := []searchDocumentSnapshot{{
		GroupID: 7, TaskID: 3, FinalizedRevisionID: &revisionID,
		InternalText: "apply-time", FinalText: "apply-time", UpdatedAt: at,
	}}
	mock.ExpectQuery("SELECT group_id,task_id,finalized_revision_id,internal_text,final_text,updated_at").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "task_id", "finalized_revision_id",
			"internal_text", "final_text", "updated_at",
		}).AddRow(7, 3, 10, "reindexed-later", "reindexed-later", at.Add(time.Minute)))

	matches, err := searchDocumentsMatchSnapshot(
		context.Background(),
		db,
		[]int64{3},
		expected,
	)
	if err != nil {
		t.Fatal(err)
	}
	if matches {
		t.Fatal("post-apply search projection drift must block workflow rollback")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTaskSearchDocumentsForTasksPreservesProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 10, 45, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT task_id,task_no,product_name_snapshot,sku_code,primary_sku_code").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "task_no", "product_name_snapshot", "sku_code",
			"primary_sku_code", "product_i_id", "task_type", "task_status",
			"priority", "owner_department", "owner_team", "owner_org_team",
			"creator_id", "creator_name", "requester_id", "requester_name",
			"designer_id", "designer_name", "current_handler_id",
			"current_handler_name", "created_at", "updated_at", "deadline_at",
			"asset_text", "search_text",
		}).AddRow(
			3, "RW-3", "product", "SKU-3", "SKU-3", "I-3",
			"design_task", "Completed", "normal", "design", "team-a",
			"team-a", 1, "creator", nil, "", 2, "designer", nil, "",
			at, at, nil, nil, "search projection",
		))

	items, err := captureTaskSearchDocumentsForTasks(
		context.Background(),
		db,
		[]int64{3},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].TaskID != 3 ||
		items[0].TaskStatus != "Completed" ||
		items[0].DesignerID == nil || *items[0].DesignerID != 2 ||
		items[0].RequesterID != nil || items[0].AssetText != nil {
		t.Fatalf("task search snapshots = %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureTaskSearchDocumentsFailsClosedOnRowIterationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT task_id,task_no,product_name_snapshot,sku_code,primary_sku_code").
		WithArgs(int64(3)).
		WillReturnRows(
			sqlmock.NewRows([]string{
				"task_id", "task_no", "product_name_snapshot", "sku_code",
				"primary_sku_code", "product_i_id", "task_type", "task_status",
				"priority", "owner_department", "owner_team", "owner_org_team",
				"creator_id", "creator_name", "requester_id", "requester_name",
				"designer_id", "designer_name", "current_handler_id",
				"current_handler_name", "created_at", "updated_at", "deadline_at",
				"asset_text", "search_text",
			}).
				AddRow(
					3, "RW-3", "product", "SKU-3", "SKU-3", "I-3",
					"design_task", "Completed", "normal", "design", "team-a",
					"team-a", nil, "", nil, "", nil, "", nil, "",
					nil, nil, nil, nil, "search",
				).
				RowError(0, errors.New("driver iteration failed")),
		)
	_, err = captureTaskSearchDocumentsForTasks(
		context.Background(),
		db,
		[]int64{3},
	)
	if err == nil {
		t.Fatal("row iteration error must block task search snapshot")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreTaskSearchDocumentsDeletesAndRestoresExactRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 10, 45, 0, 0, time.UTC)
	creatorID := int64(1)
	assetText := "asset"
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("DELETE FROM task_search_documents WHERE task_id IN").
		WithArgs(int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO task_search_documents").
		WithArgs(
			int64(3), "RW-3", "product", "SKU-3", "SKU-3", "I-3",
			"design_task", "Completed", "normal", "design", "team-a",
			"team-a", creatorID, "creator", nil, "", nil, "", nil, "",
			at, at, nil, assetText, "search projection",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err = restoreTaskSearchDocuments(
		context.Background(),
		tx,
		[]int64{3},
		[]taskSearchDocumentSnapshot{{
			TaskID: 3, TaskNo: "RW-3", ProductNameSnapshot: "product",
			SKUCode: "SKU-3", PrimarySKUCode: "SKU-3", ProductIID: "I-3",
			TaskType: "design_task", TaskStatus: "Completed", Priority: "normal",
			OwnerDepartment: "design", OwnerTeam: "team-a",
			OwnerOrgTeam: "team-a", CreatorID: &creatorID, CreatorName: "creator",
			CreatedAt: &at, UpdatedAt: &at, AssetText: &assetText,
			SearchText: "search projection",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeCompletedTaskModulesUsesRuntimeTerminalContract(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("UPDATE task_modules tm.*JOIN tasks t.*tm.state='completed'").
		WithArgs(int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 4))
	if err := normalizeCompletedTaskModules(context.Background(), tx, []int64{2, 1}); err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureResourceGroupsForTasksAssemblesCompleteOrderedGraph(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Date(2026, 7, 24, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id,task_id,working_revision_id,finalized_revision_id,lock_version,migration_incomplete,migration_issue,updated_at").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "working_revision_id", "finalized_revision_id", "lock_version", "migration_incomplete", "migration_issue", "updated_at"}).
			AddRow(7, 3, 11, 10, 4, false, "", at))
	mock.ExpectQuery("SELECT id,group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at,created_at").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "revision_no", "status", "mode", "source_task_asset_id", "source_stage", "created_by", "reason", "submitted_at", "finalized_at", "created_at"}).
			AddRow(11, 7, 1, "superseded", "single", 101, "migration", 1, "first", at, at, at).
			AddRow(10, 7, 2, "finalized", "set", 102, "reopen", 1, "second", at, at, at))
	mock.ExpectQuery("SELECT id,revision_id,task_asset_id,sort_order,item_name,created_at").
		WithArgs(int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "task_asset_id", "sort_order", "item_name", "created_at"}).
			AddRow(201, 10, 301, 0, "final-a", at).
			AddRow(202, 11, 302, 0, "final-b", at))
	mock.ExpectQuery("SELECT id,revision_id,reference_file_ref_id,formal_task_asset_id,sort_order").
		WithArgs(int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "revision_id", "reference_file_ref_id", "formal_task_asset_id", "sort_order", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot", "created_at"}).
			AddRow(401, 10, 501, nil, 0, "ref-a", "a.jpg", "task:0", at).
			AddRow(402, 11, 502, 601, 0, "ref-b", "b.jpg", "task:0", at))

	groups, err := captureResourceGroupsForTasks(context.Background(), db, []int64{3})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1", len(groups))
	}
	if !reflect.DeepEqual(groups[0].RevisionIDs, []int64{10, 11}) {
		t.Fatalf("revision ids = %v", groups[0].RevisionIDs)
	}
	if got := []int64{groups[0].Revisions[0].ID, groups[0].Revisions[1].ID}; !reflect.DeepEqual(got, []int64{11, 10}) {
		t.Fatalf("revision history order = %v", got)
	}
	if groups[0].Revisions[0].Items[0].TaskAssetID != 302 || groups[0].Revisions[1].Items[0].TaskAssetID != 301 {
		t.Fatalf("items were attached to the wrong revisions: %#v", groups[0].Revisions)
	}
	if groups[0].Revisions[0].References[0].ReferenceFileRefID != 502 || groups[0].Revisions[1].References[0].ReferenceFileRefID != 501 {
		t.Fatalf("references were attached to the wrong revisions: %#v", groups[0].Revisions)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureAssetBindingsForTasksPreservesNullAndEmptyScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,task_id,binding_state,bound_group_id,bound_role").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "binding_state", "bound_group_id", "bound_role",
			"staged_task_sku_item_id", "staged_retouch_requirement_id", "staged_role", "staged_by", "upload_session_id", "staged_expires_at",
			"access_revoked_at", "access_revoked_reason", "object_deleted_at",
			"asset_type", "scope_sku_code", "retouch_requirement_id", "mime_type", "whole_hash", "deleted_at", "cleaned_at",
		}).
			AddRow(1, 3, "unbound", nil, nil, nil, nil, nil, nil, nil, nil, nil, "", nil, "delivery", nil, nil, "image/jpeg", "a", nil, nil).
			AddRow(2, 3, "unbound", nil, nil, nil, nil, nil, nil, nil, nil, nil, "", nil, "delivery", "", nil, "image/jpeg", "b", nil, nil))
	assets, err := captureAssetBindingsForTasks(context.Background(), db, []int64{3})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("asset count = %d, want 2", len(assets))
	}
	if assets[0].ScopeSKUCode != nil {
		t.Fatalf("NULL scope became non-nil: %#v", assets[0].ScopeSKUCode)
	}
	if assets[1].ScopeSKUCode == nil || *assets[1].ScopeSKUCode != "" {
		t.Fatalf("empty scope was not preserved: %#v", assets[1].ScopeSKUCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeTaskModuleSnapshotJSONIgnoresFormattingOnly(t *testing.T) {
	actual := []taskModuleSnapshot{{
		Data:             []byte("{\n  \"backfill_placeholder\": true\n}"),
		ActorOrgSnapshot: []byte("{ \"team_id\": 7 }"),
	}}
	expected := []taskModuleSnapshot{{
		Data:             []byte(`{"backfill_placeholder":true}`),
		ActorOrgSnapshot: []byte(`{"team_id":7}`),
	}}
	if err := normalizeTaskModuleSnapshotJSON(actual); err != nil {
		t.Fatal(err)
	}
	if err := normalizeTaskModuleSnapshotJSON(expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("semantic JSON snapshots differ: %#v != %#v", actual, expected)
	}
	invalid := []taskModuleSnapshot{{Data: []byte("{")}}
	if err := normalizeTaskModuleSnapshotJSON(invalid); err == nil {
		t.Fatal("invalid task module JSON was accepted")
	}
}
