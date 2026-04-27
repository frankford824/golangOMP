package tests

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/policy"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	"workflow/workers"
)

func TestWorkflowEndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	if err = db.PingContext(ctx); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}

	if err = resetTables(ctx, db); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	mdb := mysqlrepo.New(db)
	skuRepo := mysqlrepo.NewSKURepo(mdb)
	eventRepo := mysqlrepo.NewEventRepo(mdb)
	assetRepo := mysqlrepo.NewAssetVersionRepo(mdb)
	auditRepo := mysqlrepo.NewAuditRepo(mdb)
	jobRepo := mysqlrepo.NewJobRepo(mdb)
	incidentRepo := mysqlrepo.NewIncidentRepo(mdb)
	policyRepo := mysqlrepo.NewPolicyRepo(mdb)
	engine := policy.NewEngine()

	if err = seedPolicies(ctx, policyRepo); err != nil {
		t.Fatalf("seed policies: %v", err)
	}

	skuSvc := service.NewSKUService(skuRepo, eventRepo, mdb, engine)
	auditSvc := service.NewAuditService(auditRepo, skuRepo, assetRepo, jobRepo, eventRepo, incidentRepo, policyRepo, mdb, engine)
	agentSvc := service.NewAgentService(assetRepo, skuRepo, jobRepo, eventRepo, incidentRepo, policyRepo, mdb, engine)

	// 1. Create SKU
	sku, appErr := skuSvc.Create(ctx, "SKU-001", "Test Product")
	mustNoAppErr(t, appErr)
	if sku.WorkflowStatus != domain.WorkflowDraft {
		t.Fatalf("unexpected status after create: %s", sku.WorkflowStatus)
	}

	events := mustEventsBySKU(t, ctx, db, sku.ID)
	if len(events) == 0 || events[0].EventType != domain.EventSKUCreated || events[0].Sequence != 1 {
		t.Fatalf("first event should be sku.created sequence=1, got %+v", events)
	}

	// 2. Draft -> Submitted -> AuditA_Pending
	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowDraft,
		NextStatus:     domain.WorkflowSubmitted,
		TriggeredBy:    "system",
		Reason:         "submit",
	})
	mustNoAppErr(t, appErr)

	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowSubmitted,
		NextStatus:     domain.WorkflowAuditAPending,
		TriggeredBy:    "system",
		Reason:         "enqueue audit A",
	})
	mustNoAppErr(t, appErr)

	// 3. Agent sync stable version
	syncResult, appErr := agentSvc.Sync(ctx, service.AgentSyncParams{
		AgentID:       "agent-1",
		SKUCode:       "SKU-001",
		FilePath:      "/tmp/file.bin",
		WholeHash:     "hash-v1",
		FileSizeBytes: 1024,
		IsStable:      true,
	})
	mustNoAppErr(t, appErr)
	if syncResult.AssetVersionID <= 0 {
		t.Fatalf("invalid asset version id: %d", syncResult.AssetVersionID)
	}

	assetVer, err := assetRepo.GetByID(ctx, syncResult.AssetVersionID)
	if err != nil || assetVer == nil {
		t.Fatalf("load synced version: %v", err)
	}
	if !assetVer.IsStable {
		t.Fatalf("expected stable version")
	}

	// 4. Audit A approve
	auditA, appErr := auditSvc.Submit(ctx, service.AuditSubmitParams{
		ActionID:       "a1",
		AssetVersionID: syncResult.AssetVersionID,
		WholeHash:      "hash-v1",
		Stage:          domain.AuditStageA,
		Decision:       domain.AuditDecisionApprove,
	})
	mustNoAppErr(t, appErr)
	if auditA == nil || auditA.Action == nil {
		t.Fatalf("expected audit A action")
	}

	// 5. AuditAApproved -> AuditBPending
	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowAuditAApproved,
		NextStatus:     domain.WorkflowAuditBPending,
		TriggeredBy:    "system",
		Reason:         "enqueue audit B",
	})
	mustNoAppErr(t, appErr)

	// 6. Audit B approve -> ApprovedPendingVerify + jobs
	auditB, appErr := auditSvc.Submit(ctx, service.AuditSubmitParams{
		ActionID:       "a2",
		AssetVersionID: syncResult.AssetVersionID,
		WholeHash:      "hash-v1",
		Stage:          domain.AuditStageB,
		Decision:       domain.AuditDecisionApprove,
	})
	mustNoAppErr(t, appErr)
	if auditB == nil || auditB.Action == nil {
		t.Fatalf("expected audit B action")
	}
	if len(auditB.Jobs) != 2 {
		t.Fatalf("expected 2 jobs from audit B, got %d", len(auditB.Jobs))
	}

	jobsBySKU, err := jobRepo.ListBySKUID(ctx, sku.ID)
	if err != nil {
		t.Fatalf("list jobs by sku: %v", err)
	}
	if len(jobsBySKU) != 2 {
		t.Fatalf("expected 2 jobs in db, got %d", len(jobsBySKU))
	}
	for _, j := range jobsBySKU {
		if j.Status != domain.JobStatusPendingVerify {
			t.Fatalf("job should be PendingVerify, got %s", j.Status)
		}
	}

	// 7. ApprovedPendingVerify -> Approved, and set jobs Pending
	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowApprovedPendingVerify,
		NextStatus:     domain.WorkflowApproved,
		TriggeredBy:    "system",
		Reason:         "verify gate passed",
	})
	mustNoAppErr(t, appErr)

	if _, err = db.ExecContext(ctx,
		`UPDATE distribution_jobs SET status = ?, updated_at = NOW() WHERE sku_id = ?`,
		domain.JobStatusPending,
		sku.ID,
	); err != nil {
		t.Fatalf("set jobs pending: %v", err)
	}

	// 8-10. Pull + Heartbeat + Ack job #1
	pull1, appErr := agentSvc.PullJob(ctx, "agent-1")
	mustNoAppErr(t, appErr)
	if pull1 == nil || pull1.Job == nil {
		t.Fatalf("expected first pulled job")
	}

	var attemptCount int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_attempts WHERE id = ?`, pull1.AttemptID).Scan(&attemptCount); err != nil {
		t.Fatalf("query attempt row: %v", err)
	}
	if attemptCount != 1 {
		t.Fatalf("expected attempt row count=1, got %d", attemptCount)
	}

	hb1, appErr := agentSvc.Heartbeat(ctx, pull1.AttemptID)
	mustNoAppErr(t, appErr)
	if !hb1.LeaseExpiresAt.After(pull1.LeaseExpiresAt) {
		t.Fatalf("heartbeat did not extend lease")
	}

	size := int64(1024)
	fileID1 := "file-id-1"
	appErr = agentSvc.AckJob(ctx, service.AckJobParams{
		AttemptID: pull1.AttemptID,
		Success:   true,
		Evidence: &domain.Evidence{
			Level:     domain.EvidenceLevelL1,
			FileID:    &fileID1,
			SizeBytes: &size,
		},
	})
	mustNoAppErr(t, appErr)

	job1, err := jobRepo.GetByID(ctx, pull1.Job.ID)
	if err != nil || job1 == nil {
		t.Fatalf("load job1: %v", err)
	}
	if job1.Status != domain.JobStatusDone {
		t.Fatalf("job1 should be done, got %s", job1.Status)
	}

	// 11. Pull + Heartbeat + Ack job #2 -> SKU Completed
	pull2, appErr := agentSvc.PullJob(ctx, "agent-1")
	mustNoAppErr(t, appErr)
	if pull2 == nil || pull2.Job == nil {
		t.Fatalf("expected second pulled job")
	}

	hb2, appErr := agentSvc.Heartbeat(ctx, pull2.AttemptID)
	mustNoAppErr(t, appErr)
	if !hb2.LeaseExpiresAt.After(pull2.LeaseExpiresAt) {
		t.Fatalf("heartbeat2 did not extend lease")
	}

	fileID2 := "file-id-2"
	appErr = agentSvc.AckJob(ctx, service.AckJobParams{
		AttemptID: pull2.AttemptID,
		Success:   true,
		Evidence: &domain.Evidence{
			Level:     domain.EvidenceLevelL1,
			FileID:    &fileID2,
			SizeBytes: &size,
		},
	})
	mustNoAppErr(t, appErr)

	finalSKU, err := skuRepo.GetByID(ctx, sku.ID)
	if err != nil || finalSKU == nil {
		t.Fatalf("load final sku: %v", err)
	}
	if finalSKU.WorkflowStatus != domain.WorkflowCompleted {
		t.Fatalf("sku should be completed, got %s", finalSKU.WorkflowStatus)
	}

	// 12. Event sequence contiguity + key event order
	events = mustEventsBySKU(t, ctx, db, sku.ID)
	if len(events) < 10 {
		t.Fatalf("too few events, got %d", len(events))
	}
	for i, e := range events {
		want := int64(i + 1)
		if e.Sequence != want {
			t.Fatalf("event sequence gap at index %d: got %d want %d", i, e.Sequence, want)
		}
	}
	mustContainEventTypesInOrder(t, events, []string{
		domain.EventSKUCreated,
		domain.EventVersionStable,
		domain.EventAuditSubmitted,
		domain.EventAuditSubmitted,
		domain.EventJobCreated,
		domain.EventJobCreated,
		domain.EventJobRunning,
		domain.EventJobDone,
		domain.EventJobRunning,
		domain.EventJobDone,
		domain.EventSKUStatusChanged,
	})

	t.Run("idempotency replay", func(t *testing.T) {
		var auditCountBefore, jobCountBefore int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_actions`).Scan(&auditCountBefore); err != nil {
			t.Fatalf("count audit actions before: %v", err)
		}
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM distribution_jobs`).Scan(&jobCountBefore); err != nil {
			t.Fatalf("count jobs before: %v", err)
		}

		replay, replayErr := auditSvc.Submit(ctx, service.AuditSubmitParams{
			ActionID:       "a2",
			AssetVersionID: syncResult.AssetVersionID,
			WholeHash:      "hash-v1",
			Stage:          domain.AuditStageB,
			Decision:       domain.AuditDecisionApprove,
		})
		mustNoAppErr(t, replayErr)
		if replay == nil || replay.Action == nil || replay.Action.ActionID != "a2" {
			t.Fatalf("unexpected replay result: %+v", replay)
		}

		var auditCountAfter, jobCountAfter int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_actions`).Scan(&auditCountAfter); err != nil {
			t.Fatalf("count audit actions after: %v", err)
		}
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM distribution_jobs`).Scan(&jobCountAfter); err != nil {
			t.Fatalf("count jobs after: %v", err)
		}
		if auditCountAfter != auditCountBefore || jobCountAfter != jobCountBefore {
			t.Fatalf("idempotency replay created rows: audits %d->%d jobs %d->%d", auditCountBefore, auditCountAfter, jobCountBefore, jobCountAfter)
		}
	})

	t.Run("stale attempt rejected after reap", func(t *testing.T) {
		staleSKU, staleAssetID := mustPrepareRunnableSKU(t, ctx, skuSvc, auditSvc, agentSvc, db, "SKU-STALE", "hash-stale")

		pull, staleErr := agentSvc.PullJob(ctx, "agent-stale")
		mustNoAppErr(t, staleErr)
		if pull == nil || pull.Job == nil {
			t.Fatalf("expected stale test pull job")
		}

		// Force lease expiry for deterministic reaper test.
		if _, err = db.ExecContext(ctx,
			`UPDATE job_attempts SET lease_expires_at = ? WHERE id = ?`,
			time.Now().Add(-1*time.Minute),
			pull.AttemptID,
		); err != nil {
			t.Fatalf("force lease expiry: %v", err)
		}

		reaper := workers.NewLeaseReaper(db, zap.NewNop())
		if err = reaper.ReapOnce(ctx); err != nil {
			t.Fatalf("reaper reap once: %v", err)
		}

		staleJob, err := jobRepo.GetByID(ctx, pull.Job.ID)
		if err != nil || staleJob == nil {
			t.Fatalf("load stale job: %v", err)
		}
		if staleJob.Status != domain.JobStatusStale {
			t.Fatalf("expected stale job status, got %s", staleJob.Status)
		}

		fileID := "file-id-stale"
		size := int64(4096)
		staleAckErr := agentSvc.AckJob(ctx, service.AckJobParams{
			AttemptID: pull.AttemptID,
			Success:   true,
			Evidence: &domain.Evidence{
				Level:     domain.EvidenceLevelL1,
				FileID:    &fileID,
				SizeBytes: &size,
			},
		})
		if staleAckErr == nil || staleAckErr.Code != domain.ErrCodeJobAttemptExpired {
			t.Fatalf("expected JOB_ATTEMPT_EXPIRED, got %+v", staleAckErr)
		}

		_ = staleSKU
		_ = staleAssetID
	})
}

func mustPrepareRunnableSKU(
	t *testing.T,
	ctx context.Context,
	skuSvc service.SKUService,
	auditSvc service.AuditService,
	agentSvc service.AgentService,
	db *sql.DB,
	skuCode string,
	hash string,
) (*domain.SKU, int64) {
	t.Helper()

	sku, appErr := skuSvc.Create(ctx, skuCode, "Stale Test")
	mustNoAppErr(t, appErr)

	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowDraft,
		NextStatus:     domain.WorkflowSubmitted,
		TriggeredBy:    "system",
		Reason:         "submit",
	})
	mustNoAppErr(t, appErr)

	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowSubmitted,
		NextStatus:     domain.WorkflowAuditAPending,
		TriggeredBy:    "system",
		Reason:         "audit a pending",
	})
	mustNoAppErr(t, appErr)

	syncRes, appErr := agentSvc.Sync(ctx, service.AgentSyncParams{
		AgentID:       "agent-stale",
		SKUCode:       skuCode,
		FilePath:      "/tmp/stale.bin",
		WholeHash:     hash,
		FileSizeBytes: 4096,
		IsStable:      true,
	})
	mustNoAppErr(t, appErr)

	_, appErr = auditSvc.Submit(ctx, service.AuditSubmitParams{
		ActionID:       skuCode + "-a1",
		AssetVersionID: syncRes.AssetVersionID,
		WholeHash:      hash,
		Stage:          domain.AuditStageA,
		Decision:       domain.AuditDecisionApprove,
	})
	mustNoAppErr(t, appErr)

	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowAuditAApproved,
		NextStatus:     domain.WorkflowAuditBPending,
		TriggeredBy:    "system",
		Reason:         "audit b pending",
	})
	mustNoAppErr(t, appErr)

	_, appErr = auditSvc.Submit(ctx, service.AuditSubmitParams{
		ActionID:       skuCode + "-a2",
		AssetVersionID: syncRes.AssetVersionID,
		WholeHash:      hash,
		Stage:          domain.AuditStageB,
		Decision:       domain.AuditDecisionApprove,
	})
	mustNoAppErr(t, appErr)

	_, appErr = skuSvc.TransitionStatus(ctx, service.TransitionStatusParams{
		SKUID:          sku.ID,
		ExpectedStatus: domain.WorkflowApprovedPendingVerify,
		NextStatus:     domain.WorkflowApproved,
		TriggeredBy:    "system",
		Reason:         "ready to run",
	})
	mustNoAppErr(t, appErr)

	if _, err := db.ExecContext(ctx,
		`UPDATE distribution_jobs SET status = ?, updated_at = NOW() WHERE sku_id = ?`,
		domain.JobStatusPending,
		sku.ID,
	); err != nil {
		t.Fatalf("set stale-case jobs pending: %v", err)
	}

	return sku, syncRes.AssetVersionID
}

type eventRow struct {
	Sequence  int64
	EventType string
}

func mustEventsBySKU(t *testing.T, ctx context.Context, db *sql.DB, skuID int64) []eventRow {
	t.Helper()
	rows, err := db.QueryContext(ctx,
		`SELECT sequence, event_type FROM event_logs WHERE sku_id = ? ORDER BY sequence ASC`,
		skuID,
	)
	if err != nil {
		t.Fatalf("query events by sku: %v", err)
	}
	defer rows.Close()

	var out []eventRow
	for rows.Next() {
		var e eventRow
		if err = rows.Scan(&e.Sequence, &e.EventType); err != nil {
			t.Fatalf("scan event row: %v", err)
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		t.Fatalf("iterate event rows: %v", err)
	}
	return out
}

func mustContainEventTypesInOrder(t *testing.T, got []eventRow, expected []string) {
	t.Helper()
	pos := 0
	for _, ev := range got {
		if pos < len(expected) && ev.EventType == expected[pos] {
			pos++
		}
	}
	if pos != len(expected) {
		t.Fatalf("event types not found in order, matched %d/%d", pos, len(expected))
	}
}

func mustNoAppErr(t *testing.T, appErr *domain.AppError) {
	t.Helper()
	if appErr != nil {
		t.Fatalf("unexpected app error: code=%s message=%s", appErr.Code, appErr.Message)
	}
}

func seedPolicies(ctx context.Context, policyRepo repo.PolicyRepo) error {
	if err := policyRepo.Upsert(ctx, &domain.SystemPolicy{
		Key:       "distribution_targets",
		Value:     `["target_A","target_B"]`,
		UpdatedBy: 1,
	}); err != nil {
		return err
	}
	if err := policyRepo.Upsert(ctx, &domain.SystemPolicy{
		Key:       "job_lease_seconds",
		Value:     `"5"`,
		UpdatedBy: 1,
	}); err != nil {
		return err
	}
	return nil
}

func resetTables(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`SET FOREIGN_KEY_CHECKS = 0`,
		`TRUNCATE TABLE event_logs`,
		`TRUNCATE TABLE sku_sequences`,
		`TRUNCATE TABLE job_attempts`,
		`TRUNCATE TABLE distribution_jobs`,
		`TRUNCATE TABLE audit_actions`,
		`TRUNCATE TABLE incidents`,
		`TRUNCATE TABLE asset_versions`,
		`TRUNCATE TABLE skus`,
		`TRUNCATE TABLE system_policies`,
		`SET FOREIGN_KEY_CHECKS = 1`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
