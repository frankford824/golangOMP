//go:build integration

package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	"workflow/testsupport/r35"
)

func TestAssetObjectDeletionOutboxSchemaSupportsAdapterDispatch(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	if !workflowGroupMigrationSchemaReady(t, db) {
		t.Skip("workflow-group migrations are not applied")
	}
	rows, err := db.Query(`
		SELECT column_name, is_nullable
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'asset_object_deletion_outbox'
		  AND column_name IN ('storage_ref_id','storage_adapter','storage_is_placeholder')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var name, nullable string
		if err := rows.Scan(&name, &nullable); err != nil {
			t.Fatal(err)
		}
		got[name] = nullable
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"storage_ref_id":         "YES",
		"storage_adapter":        "NO",
		"storage_is_placeholder": "NO",
	}
	for column, nullable := range want {
		if got[column] != nullable {
			t.Fatalf("asset_object_deletion_outbox.%s nullable = %q, want %q (columns=%v)", column, got[column], nullable, got)
		}
	}
}

func TestTaskFinalizerReplacesSourceWithAdapterSnapshotAndCompletesTask(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const (
		taskID           = int64(970501)
		actorID          = int64(10001)
		workflowRevision = int64(11)
	)
	cleanupWorkflowGroupFixture(t, db, taskID)
	t.Cleanup(func() { cleanupWorkflowGroupFixture(t, db, taskID) })
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), "normal", nil)
	if _, err := db.Exec(`
		UPDATE tasks
		SET task_status='PendingAudit', workflow_revision=?, current_handler_id=?,
		    owner_department='', owner_team='', owner_org_team='',
		    owner_department_id=NULL, owner_team_id=NULL
		WHERE id=?`, workflowRevision, actorID, taskID); err != nil {
		t.Fatal(err)
	}

	groupResult, err := db.Exec(`
		INSERT INTO task_asset_groups (task_id,scope_kind,migration_incomplete,migration_issue,lock_version)
		VALUES (?,'task',0,'',2)`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	groupID, _ := groupResult.LastInsertId()

	type assetFixture struct {
		id          int64
		refID       string
		assetType   string
		fileName    string
		storageKey  string
		storageRole string
	}
	assets := []assetFixture{
		{id: taskID*10 + 1, refID: fmt.Sprintf("finalizer-old-source-%d", taskID), assetType: "draft", fileName: "old-source.psd", storageKey: fmt.Sprintf("finalizer/%d/old-source.psd", taskID), storageRole: "source"},
		{id: taskID*10 + 2, refID: fmt.Sprintf("finalizer-old-final-%d", taskID), assetType: "final", fileName: "old-final.png", storageKey: fmt.Sprintf("finalizer/%d/old-final.png", taskID), storageRole: "final"},
		{id: taskID*10 + 3, refID: fmt.Sprintf("finalizer-new-source-%d", taskID), assetType: "draft", fileName: "new-source.psd", storageKey: fmt.Sprintf("finalizer/%d/new-source.psd", taskID), storageRole: "source"},
		{id: taskID*10 + 4, refID: fmt.Sprintf("finalizer-new-final-%d", taskID), assetType: "final", fileName: "new-final.png", storageKey: fmt.Sprintf("finalizer/%d/new-final.png", taskID), storageRole: "final"},
	}
	for index, asset := range assets {
		if _, err := db.Exec(`
			INSERT INTO asset_storage_refs
				(ref_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name,mime_type,is_placeholder,status)
			VALUES (?,'workflow_groups_migrate_test',?,'oss_upload_service','storage_key',?,?,?,0,'ready')`,
			asset.refID, taskID, asset.storageKey, asset.fileName, map[bool]string{true: "image/png", false: "application/octet-stream"}[asset.assetType == "final"]); err != nil {
			t.Fatalf("insert storage ref %d: %v", index, err)
		}
		if _, err := db.Exec(`
			INSERT INTO task_assets
				(id,task_id,asset_type,binding_state,bound_group_id,bound_role,version_no,file_name,mime_type,storage_key,storage_ref_id,uploaded_by,uploaded_at,remark)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,NOW(),'finalizer adapter integration')`,
			asset.id, taskID, asset.assetType, "bound", groupID, asset.storageRole, index+1, asset.fileName,
			map[bool]string{true: "image/png", false: "application/octet-stream"}[asset.assetType == "final"], asset.storageKey, asset.refID, actorID); err != nil {
			t.Fatalf("insert task asset %d: %v", index, err)
		}
	}

	oldResult, err := db.Exec(`
		INSERT INTO task_asset_group_revisions
			(group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at)
		VALUES (?,1,'finalized','single',?,'design',?,'initial final',NOW(),NOW())`, groupID, assets[0].id, actorID)
	if err != nil {
		t.Fatal(err)
	}
	oldRevisionID, _ := oldResult.LastInsertId()
	newResult, err := db.Exec(`
		INSERT INTO task_asset_group_revisions
			(group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at)
		VALUES (?,2,'submitted','single',?,'audit',?,'audit replacement',NOW())`, groupID, assets[2].id, actorID)
	if err != nil {
		t.Fatal(err)
	}
	newRevisionID, _ := newResult.LastInsertId()
	if _, err := db.Exec(`INSERT INTO task_asset_group_revision_items (revision_id,task_asset_id,sort_order,item_name) VALUES (?,?,0,'old')`, oldRevisionID, assets[1].id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO task_asset_group_revision_items (revision_id,task_asset_id,sort_order,item_name) VALUES (?,?,0,'new')`, newRevisionID, assets[3].id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE task_asset_groups SET working_revision_id=?,finalized_revision_id=? WHERE id=?`, newRevisionID, oldRevisionID, groupID); err != nil {
		t.Fatal(err)
	}

	database := mysqlrepo.New(db)
	resourceRepo := mysqlrepo.NewTaskResourceGroupRepo(database)
	finalizer := service.NewTaskFinalizer(resourceRepo, mysqlrepo.NewTaskEventRepo(database))
	group := domain.TaskAssetGroup{
		ID: groupID, TaskID: taskID, ScopeKind: domain.TaskAssetGroupScopeTask,
		WorkingRevisionID: &newRevisionID, FinalizedRevisionID: &oldRevisionID, LockVersion: 2,
	}
	task := domain.TaskWorkflowLock{
		TaskID: taskID, TaskType: domain.TaskTypeOriginalProductDevelopment,
		Status: domain.TaskStatusPendingAudit, WorkflowRevision: workflowRevision, CreatorID: actorID,
	}
	if err := database.RunInTx(context.Background(), func(tx repo.Tx) error {
		_, err := finalizer.FinalizeInTx(context.Background(), tx, &task, []domain.TaskAssetGroup{group}, service.FinalizeModeDesignAudit, actorID)
		return err
	}); err != nil {
		t.Fatalf("finalize source replacement: %v", err)
	}

	var taskStatus string
	var gotWorkflowRevision int64
	if err := db.QueryRow(`SELECT task_status,workflow_revision FROM tasks WHERE id=?`, taskID).Scan(&taskStatus, &gotWorkflowRevision); err != nil {
		t.Fatal(err)
	}
	if taskStatus != string(domain.TaskStatusCompleted) || gotWorkflowRevision != workflowRevision+1 {
		t.Fatalf("finalized task = %s/revision %d, want Completed/%d", taskStatus, gotWorkflowRevision, workflowRevision+1)
	}
	var finalizedRevisionID, workingRevisionID, lockVersion int64
	if err := db.QueryRow(`SELECT finalized_revision_id,working_revision_id,lock_version FROM task_asset_groups WHERE id=?`, groupID).Scan(&finalizedRevisionID, &workingRevisionID, &lockVersion); err != nil {
		t.Fatal(err)
	}
	if finalizedRevisionID != newRevisionID || workingRevisionID != newRevisionID || lockVersion != 3 {
		t.Fatalf("finalized group = %d/%d lock %d, want %d/%d lock 3", finalizedRevisionID, workingRevisionID, lockVersion, newRevisionID, newRevisionID)
	}
	var oldStatus, newStatus string
	if err := db.QueryRow(`SELECT status FROM task_asset_group_revisions WHERE id=?`, oldRevisionID).Scan(&oldStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM task_asset_group_revisions WHERE id=?`, newRevisionID).Scan(&newStatus); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "superseded" || newStatus != "finalized" {
		t.Fatalf("revision states = %s/%s, want superseded/finalized", oldStatus, newStatus)
	}
	var revokedAt sql.NullTime
	var revokedReason string
	if err := db.QueryRow(`SELECT access_revoked_at,access_revoked_reason FROM task_assets WHERE id=?`, assets[0].id).Scan(&revokedAt, &revokedReason); err != nil {
		t.Fatal(err)
	}
	if !revokedAt.Valid || revokedReason != "source_replaced_by_audit" {
		t.Fatalf("old source revoke = %v/%q", revokedAt, revokedReason)
	}
	var outboxAssetID int64
	var storageRefID, storageAdapter, storageKey, status string
	var placeholder bool
	if err := db.QueryRow(`
		SELECT task_asset_id,storage_ref_id,storage_adapter,storage_is_placeholder,storage_key,status
		FROM asset_object_deletion_outbox WHERE task_asset_id=?`, assets[0].id).
		Scan(&outboxAssetID, &storageRefID, &storageAdapter, &placeholder, &storageKey, &status); err != nil {
		t.Fatal(err)
	}
	if outboxAssetID != assets[0].id || storageRefID != assets[0].refID || storageAdapter != "oss_upload_service" || placeholder || storageKey != assets[0].storageKey || status != "pending" {
		t.Fatalf("outbox snapshot = asset %d ref %q adapter %q placeholder %v key %q status %q", outboxAssetID, storageRefID, storageAdapter, placeholder, storageKey, status)
	}
	var completedEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_event_logs WHERE task_id=? AND event_type='task.completed'`, taskID).Scan(&completedEvents); err != nil || completedEvents != 1 {
		t.Fatalf("task.completed events = %d/%v", completedEvents, err)
	}
}

func TestWorkflowGroupsMigratePrecreatedShellApplyRerunRollback(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const taskID = int64(970001)
	mapping, groupID, _, _, _ := createResourceMigrationFixture(t, db, taskID)
	assertNoUnrelatedIntegrationBlockers(t, db, mapping)
	database, _ := currentDatabase(context.Background(), db)
	o := options{SnapshotDir: t.TempDir()}
	if err := apply(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("apply precreated shell: %v", err)
	}
	var revisionID sql.NullInt64
	var marker bool
	if err := db.QueryRow(`SELECT migration_incomplete,working_revision_id FROM task_asset_groups WHERE id=?`, groupID).Scan(&marker, &revisionID); err != nil || marker || !revisionID.Valid {
		t.Fatalf("applied shell marker/pointer/error = %v/%v/%v", marker, revisionID, err)
	}
	var refIDSnapshot, fileNameSnapshot, scopeSnapshot string
	if err := db.QueryRow(`
		SELECT ref_id_snapshot,file_name_snapshot,scope_snapshot
		FROM task_asset_group_revision_references
		WHERE revision_id=?`, revisionID.Int64).Scan(&refIDSnapshot, &fileNameSnapshot, &scopeSnapshot); err != nil {
		t.Fatal(err)
	}
	if refIDSnapshot != fmt.Sprintf("mig-ref-%d", taskID) || fileNameSnapshot != "reference.png" || scopeSnapshot != fmt.Sprintf("sku:%d", mapping.Resources[0].ScopeRefID) {
		t.Fatalf("migrated reference snapshot = %q/%q/%q", refIDSnapshot, fileNameSnapshot, scopeSnapshot)
	}
	if err := apply(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("idempotent apply rerun: %v", err)
	}
	if err := rollback(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if err := db.QueryRow(`SELECT migration_incomplete,working_revision_id FROM task_asset_groups WHERE id=?`, groupID).Scan(&marker, &revisionID); err != nil || !marker || revisionID.Valid {
		t.Fatalf("rolled-back shell marker/pointer/error = %v/%v/%v", marker, revisionID, err)
	}
}

func TestWorkflowGroupsMigratedReferenceSnapshotsPreserveEveryScope(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const taskID = int64(970051)
	cleanupWorkflowGroupFixture(t, db, taskID)
	t.Cleanup(func() { cleanupWorkflowGroupFixture(t, db, taskID) })
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), "normal", nil)
	if _, err := db.Exec(`UPDATE tasks SET owner_department='',owner_team='',owner_org_team='',owner_department_id=NULL,owner_team_id=NULL WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	skuResult, err := db.Exec(`
		INSERT INTO task_sku_items
			(task_id,sequence_no,sku_code,sku_status,sku_origin,product_name_snapshot,product_short_name,category_code,material_mode,cost_price_mode,design_requirement,reference_file_refs_json,dedupe_key)
		VALUES (?,1,?,'generated','native','Reference scopes','','','','','','[]','reference-scopes')`, taskID, fmt.Sprintf("REF-SKU-%d", taskID))
	if err != nil {
		t.Fatal(err)
	}
	skuItemID, _ := skuResult.LastInsertId()
	retouchResult, err := db.Exec(`INSERT INTO task_retouch_requirements (task_id,description,sort_order,created_by) VALUES (?,'Reference scope',0,10001)`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	retouchRequirementID, _ := retouchResult.LastInsertId()
	groupResult, err := db.Exec(`INSERT INTO task_asset_groups (task_id,scope_kind,migration_incomplete,migration_issue) VALUES (?,'task',0,'')`, taskID)
	if err != nil {
		t.Fatal(err)
	}
	groupID, _ := groupResult.LastInsertId()
	revisionResult, err := db.Exec(`
		INSERT INTO task_asset_group_revisions (group_id,revision_no,status,mode,source_stage,created_by,reason)
		VALUES (?,1,'draft','single','migration',10001,'reference snapshot integration')`, groupID)
	if err != nil {
		t.Fatal(err)
	}
	revisionID, _ := revisionResult.LastInsertId()

	type referenceFixture struct {
		name          string
		skuID         any
		retouchID     any
		expectedScope string
	}
	fixtures := []referenceFixture{
		{name: "task", expectedScope: "task"},
		{name: "sku", skuID: skuItemID, expectedScope: fmt.Sprintf("sku:%d", skuItemID)},
		{name: "retouch", retouchID: retouchRequirementID, expectedScope: fmt.Sprintf("retouch_requirement:%d", retouchRequirementID)},
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for order, fixture := range fixtures {
		refID := fmt.Sprintf("mig-scope-%d-%s", taskID, fixture.name)
		fileName := fixture.name + ".png"
		if _, err := tx.Exec(`
			INSERT INTO asset_storage_refs
				(ref_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name,mime_type,is_placeholder,status)
			VALUES (?,'workflow_groups_migrate_test',?,'local','storage_key',?,?,'image/png',0,'ready')`,
			refID, taskID, fmt.Sprintf("migration/%d/%s", taskID, fileName), fileName); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		result, err := tx.Exec(`
			INSERT INTO reference_file_refs (task_id,sku_item_id,retouch_requirement_id,ref_id,owner_module_key,context)
			VALUES (?,?,?,?, 'design','migration integration')`, taskID, fixture.skuID, fixture.retouchID, refID)
		if err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		referenceID, _ := result.LastInsertId()
		if err := insertMigratedReferenceSnapshot(context.Background(), tx, revisionID, referenceID, order); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT ref_id_snapshot,file_name_snapshot,scope_snapshot FROM task_asset_group_revision_references WHERE revision_id=? ORDER BY sort_order`, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var refID, fileName, scope string
		if err := rows.Scan(&refID, &fileName, &scope); err != nil {
			t.Fatal(err)
		}
		fixture := fixtures[index]
		if refID != fmt.Sprintf("mig-scope-%d-%s", taskID, fixture.name) || fileName != fixture.name+".png" || scope != fixture.expectedScope {
			t.Fatalf("snapshot %s = %q/%q/%q", fixture.name, refID, fileName, scope)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(fixtures) {
		t.Fatalf("snapshot rows = %d, want %d", index, len(fixtures))
	}
}

func TestWorkflowGroupsMigratePlanningApplyRerunRollbackRestoresSKUOrigin(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const taskID = int64(970101)
	mapping, skuItemID := createPlanningMigrationFixture(t, db, taskID)
	assertNoUnrelatedIntegrationBlockers(t, db, mapping)
	database, _ := currentDatabase(context.Background(), db)
	o := options{SnapshotDir: t.TempDir()}
	if err := apply(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("planning apply: %v", err)
	}
	if err := apply(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("planning rerun: %v", err)
	}
	var taskType, taskStatus, origin string
	if err := db.QueryRow(`SELECT task_type,task_status FROM tasks WHERE id=?`, taskID).Scan(&taskType, &taskStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT sku_origin FROM task_sku_items WHERE id=?`, skuItemID).Scan(&origin); err != nil {
		t.Fatal(err)
	}
	if taskType != "sku_planning" || taskStatus != "Completed" || origin != "legacy_migration" {
		t.Fatalf("applied task/origin = %s/%s/%s", taskType, taskStatus, origin)
	}
	if err := rollback(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("planning rollback: %v", err)
	}
	if err := db.QueryRow(`SELECT task_type,task_status FROM tasks WHERE id=?`, taskID).Scan(&taskType, &taskStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT sku_origin FROM task_sku_items WHERE id=?`, skuItemID).Scan(&origin); err != nil {
		t.Fatal(err)
	}
	if taskType != "purchase_task" || taskStatus != "InProgress" || origin != "native" {
		t.Fatalf("restored task/origin = %s/%s/%s", taskType, taskStatus, origin)
	}
	var planningRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_planning_settings WHERE task_id=?`, taskID).Scan(&planningRows); err != nil || planningRows != 0 {
		t.Fatalf("planning settings after rollback = %d/%v", planningRows, err)
	}
}

func TestWorkflowGroupsRollbackRejectsForwardResourceGroupAssetAndTaskWrites(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *sql.DB, int64, int64)
	}{
		{
			name: "new group without event",
			mutate: func(t *testing.T, db *sql.DB, taskID, _ int64) {
				if _, err := db.Exec(`INSERT INTO task_asset_groups (task_id,scope_kind,migration_incomplete,migration_issue) VALUES (?,'task',0,'')`, taskID); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "new staged asset without event",
			mutate: func(t *testing.T, db *sql.DB, taskID, _ int64) {
				if _, err := db.Exec(`INSERT INTO task_assets (task_id,asset_type,version_no,file_name,uploaded_by,uploaded_at,remark,binding_state) VALUES (?,'draft',99,'forward.psd',10001,NOW(),'forward write','staged')`, taskID); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non mapping asset binding",
			mutate: func(t *testing.T, db *sql.DB, _ int64, extraAssetID int64) {
				if _, err := db.Exec(`UPDATE task_assets SET binding_state='discarded',access_revoked_reason='forward write' WHERE id=?`, extraAssetID); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "task workflow revision",
			mutate: func(t *testing.T, db *sql.DB, taskID, _ int64) {
				if _, err := db.Exec(`UPDATE tasks SET workflow_revision=workflow_revision+1 WHERE id=?`, taskID); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := workflowGroupsIntegrationDB(t)
			taskID := int64(970200 + index)
			mapping, _, _, _, extraAssetID := createResourceMigrationFixture(t, db, taskID)
			assertNoUnrelatedIntegrationBlockers(t, db, mapping)
			database, _ := currentDatabase(context.Background(), db)
			o := options{SnapshotDir: t.TempDir()}
			if err := apply(context.Background(), db, database, o, mapping); err != nil {
				t.Fatalf("apply: %v", err)
			}
			tt.mutate(t, db, taskID, extraAssetID)
			var beforeType, beforeStatus string
			var beforeRevision int64
			if err := db.QueryRow(`SELECT task_type,task_status,workflow_revision FROM tasks WHERE id=?`, taskID).Scan(&beforeType, &beforeStatus, &beforeRevision); err != nil {
				t.Fatal(err)
			}
			err := rollback(context.Background(), db, database, o, mapping)
			if err == nil || !strings.Contains(err.Error(), "rollback refused") {
				t.Fatalf("rollback should preserve forward write, got %v", err)
			}
			var afterType, afterStatus string
			var afterRevision int64
			if err := db.QueryRow(`SELECT task_type,task_status,workflow_revision FROM tasks WHERE id=?`, taskID).Scan(&afterType, &afterStatus, &afterRevision); err != nil {
				t.Fatal(err)
			}
			if beforeType != afterType || beforeStatus != afterStatus || beforeRevision != afterRevision {
				t.Fatalf("rejected rollback mutated task: before=%s/%s/%d after=%s/%s/%d", beforeType, beforeStatus, beforeRevision, afterType, afterStatus, afterRevision)
			}
		})
	}
}

func TestWorkflowGroupsRollbackRejectsForwardPlanningRevision(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const taskID = int64(970301)
	mapping, skuItemID := createPlanningMigrationFixture(t, db, taskID)
	assertNoUnrelatedIntegrationBlockers(t, db, mapping)
	database, _ := currentDatabase(context.Background(), db)
	o := options{SnapshotDir: t.TempDir()}
	if err := apply(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO task_planning_sku_revisions (task_sku_item_id,version_no,description_spec,quantity,target_price,currency,note,reference_url,erp_product_i_id,erp_product_name,reason,created_by) VALUES (?,2,'forward revision',2,NULL,'CNY','','','','','forward write',10001)`, skuItemID); err != nil {
		t.Fatal(err)
	}
	err := rollback(context.Background(), db, database, o, mapping)
	if err == nil || !strings.Contains(err.Error(), "rollback refused") {
		t.Fatalf("expected forward planning revision rejection, got %v", err)
	}
	var revisions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM task_planning_sku_revisions WHERE task_sku_item_id=?`, skuItemID).Scan(&revisions); err != nil || revisions != 2 {
		t.Fatalf("rollback changed planning revisions = %d/%v", revisions, err)
	}
}

func TestWorkflowGroupsApplyCapturesWriterStateAfterLock(t *testing.T) {
	db := workflowGroupsIntegrationDB(t)
	const taskID = int64(970401)
	mapping, _, _, _, _ := createResourceMigrationFixture(t, db, taskID)
	assertNoUnrelatedIntegrationBlockers(t, db, mapping)
	database, _ := currentDatabase(context.Background(), db)
	o := options{SnapshotDir: t.TempDir()}
	writer, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(`UPDATE tasks SET workflow_revision=workflow_revision+7,current_handler_id=10001 WHERE id=?`, taskID); err != nil {
		_ = writer.Rollback()
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- apply(context.Background(), db, database, o, mapping) }()
	select {
	case err := <-result:
		_ = writer.Rollback()
		t.Fatalf("apply should wait for writer lock, returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("apply after writer commit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("apply did not resume after writer commit")
	}
	if err := rollback(context.Background(), db, database, o, mapping); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var workflowRevision int64
	var handlerID sql.NullInt64
	if err := db.QueryRow(`SELECT workflow_revision,current_handler_id FROM tasks WHERE id=?`, taskID).Scan(&workflowRevision, &handlerID); err != nil {
		t.Fatal(err)
	}
	if workflowRevision != 7 || !handlerID.Valid || handlerID.Int64 != 10001 {
		t.Fatalf("writer state was not preserved in before snapshot: revision=%d handler=%v", workflowRevision, handlerID)
	}
}

func workflowGroupsIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	if !workflowGroupMigrationSchemaReady(t, db) {
		db.Close()
		t.Skip("migrations 124-126 are not applied to the integration database")
	}
	t.Cleanup(func() { _ = db.Close() })
	ensureWorkflowGroupIntegrationActor(t, db)
	cleanupWorkflowGroupFixtureRange(t, db)
	return db
}

func ensureWorkflowGroupIntegrationActor(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT IGNORE INTO users
			(id,username,display_name,department,team,mobile,email,password_hash,status)
		VALUES
			(10001,'workflow-migration-integration-10001','Workflow migration integration','','',
			 'workflow-migration-10001','workflow-migration-10001@example.invalid','integration-only','active')`); err != nil {
		t.Fatalf("ensure workflow migration integration actor: %v", err)
	}
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=10001`).Scan(&exists); err != nil || exists != 1 {
		t.Fatalf("workflow migration integration actor = %d/%v, want 1", exists, err)
	}
}

func cleanupWorkflowGroupFixtureRange(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`SELECT id FROM tasks WHERE id BETWEEN 970000 AND 970999 ORDER BY id`)
	if err != nil {
		t.Fatalf("list reserved workflow migration fixtures: %v", err)
	}
	var taskIDs []int64
	for rows.Next() {
		var taskID int64
		if err := rows.Scan(&taskID); err != nil {
			rows.Close()
			t.Fatalf("scan reserved workflow migration fixture: %v", err)
		}
		taskIDs = append(taskIDs, taskID)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close reserved workflow migration fixture rows: %v", err)
	}
	for _, taskID := range taskIDs {
		cleanupWorkflowGroupFixture(t, db, taskID)
	}
}

func createResourceMigrationFixture(t *testing.T, db *sql.DB, taskID int64) (mappingFile, int64, int64, int64, int64) {
	t.Helper()
	cleanupWorkflowGroupFixture(t, db, taskID)
	t.Cleanup(func() { cleanupWorkflowGroupFixture(t, db, taskID) })
	r35.InsertTaskWithModules(t, db, taskID, string(domain.TaskTypeOriginalProductDevelopment), "normal", nil)
	if _, err := db.Exec(`UPDATE tasks SET owner_department='',owner_team='',owner_org_team='',owner_department_id=NULL,owner_team_id=NULL,workflow_revision=0,current_handler_id=NULL WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`
		INSERT INTO task_sku_items
			(task_id,sequence_no,sku_code,sku_status,sku_origin,product_name_snapshot,product_short_name,category_code,material_mode,cost_price_mode,design_requirement,reference_file_refs_json,dedupe_key)
		VALUES (?,1,?,'generated','native','Migration test','','','','','','[]','migration-test')`, taskID, fmt.Sprintf("MIG-SKU-%d", taskID))
	if err != nil {
		t.Fatalf("insert SKU: %v", err)
	}
	skuItemID, _ := result.LastInsertId()
	assetIDs := []int64{taskID*10 + 1, taskID*10 + 2, taskID*10 + 3}
	for index, assetID := range assetIDs {
		kind := "draft"
		name := fmt.Sprintf("asset-%d.psd", index)
		if index == 1 {
			kind = "final"
			name = "final.png"
		}
		if _, err := db.Exec(`INSERT INTO task_assets (id,task_id,asset_type,version_no,file_name,uploaded_by,uploaded_at,remark) VALUES (?,?,?,?,?,10001,NOW(),'migration integration')`, assetID, taskID, kind, index+1, name); err != nil {
			t.Fatalf("insert task asset %d: %v", assetID, err)
		}
	}
	groupResult, err := db.Exec(`INSERT INTO task_asset_groups (task_id,scope_kind,task_sku_item_id,migration_incomplete,migration_issue) VALUES (?,'sku',?,1,'precreated by migration 125')`, taskID, skuItemID)
	if err != nil {
		t.Fatal(err)
	}
	groupID, _ := groupResult.LastInsertId()
	refID := fmt.Sprintf("mig-ref-%d", taskID)
	if _, err := db.Exec(`
		INSERT INTO asset_storage_refs
			(ref_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name,mime_type,is_placeholder,status)
		VALUES (?,'workflow_groups_migrate_test',?,'local','storage_key',?,'reference.png','image/png',0,'ready')`,
		refID, taskID, fmt.Sprintf("migration/%d/reference.png", taskID)); err != nil {
		t.Fatal(err)
	}
	referenceResult, err := db.Exec(`
		INSERT INTO reference_file_refs (task_id,sku_item_id,ref_id,owner_module_key,context)
		VALUES (?,?,?,'design','migration integration')`, taskID, skuItemID, refID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, _ := referenceResult.LastInsertId()
	mapping := mappingFile{Resources: []resourceMapping{{
		TaskID: taskID, ScopeKind: "sku", ScopeRefID: skuItemID, Mode: "single",
		SourceAssetID: int64Pointer(assetIDs[0]), FinalAssetIDs: []int64{assetIDs[1]},
		ReferenceIDs: []int64{referenceID}, CreatedBy: 10001, TargetStatus: "draft", Reason: "integration mapping",
	}}}
	return mapping, groupID, assetIDs[0], assetIDs[1], assetIDs[2]
}

func createPlanningMigrationFixture(t *testing.T, db *sql.DB, taskID int64) (mappingFile, int64) {
	t.Helper()
	cleanupWorkflowGroupFixture(t, db, taskID)
	t.Cleanup(func() { cleanupWorkflowGroupFixture(t, db, taskID) })
	r35.InsertTaskWithModules(t, db, taskID, "purchase_task", "normal", nil)
	if _, err := db.Exec(`UPDATE tasks SET task_type='purchase_task',owner_department='',owner_team='',owner_org_team='',owner_department_id=NULL,owner_team_id=NULL,workflow_revision=0,current_handler_id=NULL WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`
		INSERT INTO task_sku_items
			(task_id,sequence_no,sku_code,sku_status,sku_origin,product_name_snapshot,product_short_name,category_code,material_mode,cost_price_mode,design_requirement,reference_file_refs_json,dedupe_key)
		VALUES (?,1,?,'generated','native','Planning migration','','','','','','[]','planning-migration')`, taskID, fmt.Sprintf("PLAN-SKU-%d", taskID))
	if err != nil {
		t.Fatal(err)
	}
	skuItemID, _ := result.LastInsertId()
	var ruleRevisionID int64
	if err := db.QueryRow(`SELECT id FROM code_rule_revisions ORDER BY id LIMIT 1`).Scan(&ruleRevisionID); err != nil {
		t.Fatalf("planning rule revision: %v", err)
	}
	mapping := mappingFile{Planning: []planningMapping{{
		TaskID: taskID, CodeRuleRevisionID: ruleRevisionID, CreatedBy: 10001,
		Items: []planningItemMapping{{TaskSKUItemID: skuItemID, DescriptionSpec: "迁移规格", Quantity: 2}},
	}}}
	return mapping, skuItemID
}

func assertNoUnrelatedIntegrationBlockers(t *testing.T, db *sql.DB, mapping mappingFile) {
	t.Helper()
	blockers, err := queryCutoverBlockers(context.Background(), db, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if !blockers.Empty() {
		t.Fatalf("integration database has cutover blockers: org=%+v access=%+v tasks=%+v groups=%+v", blockers.Org, blockers.Access, blockers.Tasks, blockers.Resources)
	}
}

func workflowGroupMigrationSchemaReady(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('auth_roles','task_asset_groups','task_planning_settings','code_rule_revisions')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 4
}

func cleanupWorkflowGroupFixture(t *testing.T, db *sql.DB, taskID int64) {
	t.Helper()
	mustCleanupExec(t, db, `DELETE FROM search_reindex_outbox WHERE entity_type='task' AND entity_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE o FROM search_reindex_outbox o JOIN task_asset_groups g ON o.entity_type='task_resource_group' AND o.entity_id=g.id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE cm FROM asset_workbench_client_materials cm JOIN task_asset_groups g ON g.id=cm.resource_group_id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE sd FROM task_asset_group_search_documents sd JOIN task_asset_groups g ON g.id=sd.group_id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE o FROM asset_object_deletion_outbox o JOIN task_assets a ON a.id=o.task_asset_id WHERE a.task_id=?`, taskID)
	mustCleanupExec(t, db, `UPDATE task_asset_groups SET working_revision_id=NULL,finalized_revision_id=NULL WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE rr FROM task_asset_group_revision_references rr JOIN task_asset_group_revisions r ON r.id=rr.revision_id JOIN task_asset_groups g ON g.id=r.group_id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE ri FROM task_asset_group_revision_items ri JOIN task_asset_group_revisions r ON r.id=ri.revision_id JOIN task_asset_groups g ON g.id=r.group_id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE r FROM task_asset_group_revisions r JOIN task_asset_groups g ON g.id=r.group_id WHERE g.task_id=?`, taskID)
	mustCleanupExec(t, db, `UPDATE task_assets SET bound_group_id=NULL,bound_role=NULL WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_asset_staging_drafts WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_asset_groups WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE i FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=?`, taskID)
	mustCleanupExec(t, db, `UPDATE task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id SET d.current_revision_id=NULL WHERE si.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE r FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE d FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_planning_settings WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_erp_outbox WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_reference_asset_bindings WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_assets WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM reference_file_refs WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM asset_storage_refs WHERE owner_type='workflow_groups_migrate_test' AND owner_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM upload_requests WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM design_assets WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_retouch_requirements WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_module_events WHERE task_module_id IN (SELECT id FROM task_modules WHERE task_id=?)`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_modules WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_event_logs WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_event_sequences WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM workflow_action_idempotency WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_create_requests WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_customization_orders WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM customization_jobs WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_sku_items WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM task_details WHERE task_id=?`, taskID)
	mustCleanupExec(t, db, `DELETE FROM tasks WHERE id=?`, taskID)
}

func mustCleanupExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("cleanup workflow migration fixture: %v; query=%s", err, query)
	}
}

func int64Pointer(value int64) *int64 { return &value }
