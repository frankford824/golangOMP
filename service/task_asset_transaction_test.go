package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

func TestAssetTransactionDeadlockRetry(t *testing.T) {
	for _, tc := range []struct {
		name               string
		number             uint16
		failures, attempts int
	}{
		{"recovers", 1213, 2, 3},
		{"bounded", 1213, 9, 4},
		{"duplicate_is_not_deadlock", 1062, 1, 1},
		{"lock_timeout_is_not_full_rollback", 1205, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for i := 0; i < tc.attempts; i++ {
				mock.ExpectBegin()
				if i < tc.failures {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			}
			svc := &taskAssetCenterService{txRunner: mysqlrepo.New(db)}
			calls, committed := 0, 0
			err = svc.runAssetTransaction(context.Background(), 4908, "test", func(tx repo.Tx) error {
				calls++
				tx.(*mysqlrepo.MySQLTx).AfterCommit(func() { committed++ })
				if calls <= tc.failures {
					return fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: tc.number})
				}
				return nil
			})
			wantSuccess := tc.failures < tc.attempts
			if (err == nil) != wantSuccess || calls != tc.attempts {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			wantCommitted := 0
			if wantSuccess {
				wantCommitted = 1
			}
			if committed != wantCommitted {
				t.Fatalf("after-commit effects=%d want=%d", committed, wantCommitted)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAssetTransactionDoesNotRetryAmbiguousCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := errors.New("connection lost during commit")
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(want)
	svc := &taskAssetCenterService{txRunner: mysqlrepo.New(db)}
	err = svc.runAssetTransaction(context.Background(), 4908, "test", func(repo.Tx) error { return nil })
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAssetTransactionCancellationStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := &taskAssetCenterService{txRunner: step04TxRunner{}}
	calls := 0
	err := svc.runAssetTransaction(ctx, 4908, "test", func(repo.Tx) error {
		calls++
		cancel()
		return &mysql.MySQLError{Number: 1213}
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

// These guards exercise the real service call paths without relying on timing
// to reproduce the production task-row / asset-range circular wait.
type assetLockOrderTx struct{ taskLocked bool }

func (*assetLockOrderTx) IsTx() {}

type assetLockOrderRunner struct{ inTx bool }

func (r *assetLockOrderRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	r.inTx = true
	defer func() { r.inTx = false }()
	return fn(&assetLockOrderTx{})
}

type assetLockOrderTaskRepo struct{ *step04TaskRepo }

func (r *assetLockOrderTaskRepo) GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.Task, error) {
	tx.(*assetLockOrderTx).taskLocked = true
	return r.step04TaskRepo.GetByIDForUpdate(ctx, tx, id)
}

type assetLockOrderDesignRepo struct {
	*step67DesignAssetRepo
	deadlocks  int
	onDeadlock func()
}

func (r *assetLockOrderDesignRepo) check(tx repo.Tx) error {
	if !tx.(*assetLockOrderTx).taskLocked {
		return errors.New("asset lock acquired before parent task lock")
	}
	if r.deadlocks > 0 {
		r.deadlocks--
		if r.onDeadlock != nil {
			r.onDeadlock()
		}
		return fmt.Errorf("lock asset: %w", &mysql.MySQLError{Number: 1213})
	}
	return nil
}

func TestUploadDeadlockRetryRechecksTaskAndSession(t *testing.T) {
	for _, changed := range []string{"task_completed", "session_cancelled", "session_type_missing"} {
		t.Run(changed, func(t *testing.T) {
			task := &domain.Task{ID: 4908, TaskNo: "T-4908", TaskStatus: domain.TaskStatusPendingAudit}
			tasks := &assetLockOrderTaskRepo{newStep04TaskRepo(task)}
			assets := &assetLockOrderDesignRepo{step67DesignAssetRepo: newStep67DesignAssetRepo()}
			versions := newStep04TaskAssetRepo()
			requests := newStep37UploadRequestRepo()
			client := newStubUploadServiceClient().(*stubUploadServiceClient)
			client.remoteSessionStatus = domain.DesignAssetSessionStatusCompleted
			svc := NewTaskAssetCenterService(tasks, assets, versions, requests, newStep37AssetStorageRefRepo(), &step04TaskEventRepo{}, &assetLockOrderRunner{}, client).(*taskAssetCenterService)
			created, appErr := svc.CreateMultipartUploadSession(taskAssetMutationTestContext(), CreateTaskAssetUploadSessionParams{
				TaskID: task.ID, CreatedBy: 652, AssetType: domain.TaskAssetTypeDelivery, Filename: "final.psd", MimeType: "image/vnd.adobe.photoshop", ExpectedSize: uploadRequestInt64Ptr(1024),
			})
			if appErr != nil {
				t.Fatal(appErr)
			}
			assets.deadlocks = 1
			assets.onDeadlock = func() {
				switch changed {
				case "task_completed":
					copyTask := *task
					copyTask.TaskStatus = domain.TaskStatusCompleted
					tasks.forUpdateTask = &copyTask
				case "session_cancelled":
					requests.requests[created.Session.ID].Status = domain.UploadRequestStatusCancelled
				case "session_type_missing":
					requests.requests[created.Session.ID].TaskAssetType = nil
				}
			}
			_, appErr = svc.CompleteUploadSession(taskAssetMutationTestContext(), CompleteTaskAssetUploadSessionParams{TaskID: task.ID, SessionID: created.Session.ID, CompletedBy: 652})
			if appErr == nil || assets.deadlocks != 0 {
				t.Fatalf("err=%+v remaining deadlocks=%d", appErr, assets.deadlocks)
			}
			if len(versions.assets) != 0 {
				t.Fatalf("created %d versions despite changed state", len(versions.assets))
			}
		})
	}
}
func (r *assetLockOrderDesignRepo) NextAssetNo(ctx context.Context, tx repo.Tx, id int64) (string, error) {
	if err := r.check(tx); err != nil {
		return "", err
	}
	return r.step67DesignAssetRepo.NextAssetNo(ctx, tx, id)
}
func (r *assetLockOrderDesignRepo) GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.DesignAsset, error) {
	if err := r.check(tx); err != nil {
		return nil, err
	}
	return r.step67DesignAssetRepo.GetByIDForUpdate(ctx, tx, id)
}

type assetOutsideTxTransport struct {
	runner *assetLockOrderRunner
	base   http.RoundTripper
	t      *testing.T
}

func (r assetOutsideTxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.runner.inTx {
		r.t.Errorf("network I/O inside asset transaction: %s", req.Method)
	}
	return r.base.RoundTrip(req)
}
