package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"workflow/repo"
)

type asyncOutboxTestTx struct{}

func (asyncOutboxTestTx) IsTx() {}

type asyncOutboxTxRunner struct{}

func (asyncOutboxTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	return fn(asyncOutboxTestTx{})
}

type asyncProjectionRepoStub struct {
	taskItems       []repo.TaskERPOutboxItem
	searchItems     []repo.SearchReindexOutboxItem
	taskSucceeded   []int64
	taskRetried     []int64
	taskRetryAlert  bool
	searchApplied   []int64
	searchSucceeded []int64
	searchRetried   []int64
	searchApplyErr  error
}

func (s *asyncProjectionRepoStub) ClaimTaskERPOutbox(context.Context, repo.Tx, string, time.Time, time.Time, int) ([]repo.TaskERPOutboxItem, error) {
	return s.taskItems, nil
}
func (s *asyncProjectionRepoStub) MarkTaskERPOutboxSucceeded(_ context.Context, _ repo.Tx, id int64, _ string) error {
	s.taskSucceeded = append(s.taskSucceeded, id)
	return nil
}
func (s *asyncProjectionRepoStub) MarkTaskERPOutboxRetry(_ context.Context, _ repo.Tx, id int64, _ string, _ string, _ time.Time, alert bool) error {
	s.taskRetried = append(s.taskRetried, id)
	s.taskRetryAlert = alert
	return nil
}
func (s *asyncProjectionRepoStub) ClaimSearchReindexOutbox(context.Context, repo.Tx, string, time.Time, time.Time, int) ([]repo.SearchReindexOutboxItem, error) {
	return s.searchItems, nil
}
func (s *asyncProjectionRepoStub) ApplySearchReindex(_ context.Context, _ repo.Tx, item repo.SearchReindexOutboxItem) error {
	s.searchApplied = append(s.searchApplied, item.ID)
	return s.searchApplyErr
}
func (s *asyncProjectionRepoStub) MarkSearchReindexOutboxSucceeded(_ context.Context, _ repo.Tx, id int64, _ string) error {
	s.searchSucceeded = append(s.searchSucceeded, id)
	return nil
}
func (s *asyncProjectionRepoStub) MarkSearchReindexOutboxRetry(_ context.Context, _ repo.Tx, id int64, _ string, _ string, _ time.Time) error {
	s.searchRetried = append(s.searchRetried, id)
	return nil
}

type taskERPProcessorStub struct{ err error }

func (s taskERPProcessorStub) ProcessTaskERPOutbox(context.Context, repo.TaskERPOutboxItem) error {
	return s.err
}

func TestTaskERPOutboxWorkerMarksSuccessAndRetry(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	repository := &asyncProjectionRepoStub{taskItems: []repo.TaskERPOutboxItem{{ID: 1, TaskID: 42, JobType: "task_filing", Attempt: 1}}}
	worker := NewTaskERPOutboxWorker(repository, asyncOutboxTxRunner{}, taskERPProcessorStub{}, AsyncOutboxWorkerConfig{}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	processed, err := worker.RunOnce(context.Background())
	if err != nil || processed != 1 || len(repository.taskSucceeded) != 1 || len(repository.taskRetried) != 0 {
		t.Fatalf("success result processed=%d err=%v succeeded=%v retried=%v", processed, err, repository.taskSucceeded, repository.taskRetried)
	}

	repository = &asyncProjectionRepoStub{taskItems: []repo.TaskERPOutboxItem{{ID: 2, TaskID: 42, JobType: "task_filing", Attempt: 5}}}
	worker = NewTaskERPOutboxWorker(repository, asyncOutboxTxRunner{}, taskERPProcessorStub{err: errors.New("ERP unavailable")}, AsyncOutboxWorkerConfig{AlertAfterAttempt: 5}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	processed, err = worker.RunOnce(context.Background())
	if err != nil || processed != 1 || len(repository.taskSucceeded) != 0 || len(repository.taskRetried) != 1 || !repository.taskRetryAlert {
		t.Fatalf("retry result processed=%d err=%v succeeded=%v retried=%v alert=%v", processed, err, repository.taskSucceeded, repository.taskRetried, repository.taskRetryAlert)
	}
}

func TestSearchReindexOutboxWorkerAppliesOrRetriesAtomically(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	repository := &asyncProjectionRepoStub{searchItems: []repo.SearchReindexOutboxItem{{ID: 10, EntityType: "task", EntityID: 42, Attempt: 1}}}
	worker := NewSearchReindexOutboxWorker(repository, asyncOutboxTxRunner{}, AsyncOutboxWorkerConfig{}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	processed, err := worker.RunOnce(context.Background())
	if err != nil || processed != 1 || len(repository.searchApplied) != 1 || len(repository.searchSucceeded) != 1 || len(repository.searchRetried) != 0 {
		t.Fatalf("success result processed=%d err=%v applied=%v succeeded=%v retried=%v", processed, err, repository.searchApplied, repository.searchSucceeded, repository.searchRetried)
	}

	repository = &asyncProjectionRepoStub{searchItems: []repo.SearchReindexOutboxItem{{ID: 11, EntityType: "task_resource_group", EntityID: 7, Attempt: 1}}, searchApplyErr: errors.New("reindex failed")}
	worker = NewSearchReindexOutboxWorker(repository, asyncOutboxTxRunner{}, AsyncOutboxWorkerConfig{}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	processed, err = worker.RunOnce(context.Background())
	if err != nil || processed != 1 || len(repository.searchSucceeded) != 0 || len(repository.searchRetried) != 1 {
		t.Fatalf("retry result processed=%d err=%v succeeded=%v retried=%v", processed, err, repository.searchSucceeded, repository.searchRetried)
	}
}
