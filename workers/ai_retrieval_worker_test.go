package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type aiRetrievalWorkerRepoStub struct {
	items       []domain.AIRetrievalOutboxItem
	succeeded   []int64
	retried     []int64
	indexed     []string
	retryAlert  bool
	retryAt     time.Time
	claimedWith string
}

func (*aiRetrievalWorkerRepoStub) UpsertRetrievalDocument(context.Context, repo.Tx, domain.AIRetrievalDocument) error {
	return nil
}
func (*aiRetrievalWorkerRepoStub) GetRetrievalDocument(context.Context, string) (*domain.AIRetrievalDocument, error) {
	return nil, errors.New("unused")
}
func (*aiRetrievalWorkerRepoStub) SearchRetrievalDocuments(context.Context, string, int) ([]domain.AIRetrievalHit, error) {
	return nil, nil
}
func (*aiRetrievalWorkerRepoStub) AuthorizeRetrievalDocument(context.Context, domain.RequestActor, string) (bool, error) {
	return false, nil
}
func (*aiRetrievalWorkerRepoStub) EnqueueRetrievalDocument(context.Context, repo.Tx, domain.AIRetrievalOutboxItem) error {
	return nil
}
func (s *aiRetrievalWorkerRepoStub) ClaimRetrievalOutbox(_ context.Context, _ repo.Tx, token string, _, _ time.Time, _ int) ([]domain.AIRetrievalOutboxItem, error) {
	s.claimedWith = token
	return append([]domain.AIRetrievalOutboxItem{}, s.items...), nil
}
func (s *aiRetrievalWorkerRepoStub) MarkRetrievalOutboxSucceeded(_ context.Context, _ repo.Tx, id int64, _ string, _ time.Time) error {
	s.succeeded = append(s.succeeded, id)
	return nil
}
func (s *aiRetrievalWorkerRepoStub) MarkRetrievalOutboxRetry(_ context.Context, _ repo.Tx, id int64, _ string, _ string, retryAt time.Time, alert bool) error {
	s.retried = append(s.retried, id)
	s.retryAt = retryAt
	s.retryAlert = alert
	return nil
}
func (s *aiRetrievalWorkerRepoStub) MarkRetrievalDocumentIndexed(_ context.Context, _ repo.Tx, id, _, _ string, _ time.Time) error {
	s.indexed = append(s.indexed, id)
	return nil
}

type aiRetrievalProcessorStub struct{ err error }

func (s aiRetrievalProcessorStub) IndexDocument(context.Context, domain.AIRetrievalOutboxItem) error {
	return s.err
}

func TestAIRetrievalWorkerMarksIndexAndOutboxSuccess(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	repository := &aiRetrievalWorkerRepoStub{items: []domain.AIRetrievalOutboxItem{{ID: 1, DocumentID: "task:7", Operation: "upsert", ContentHash: "h", EmbeddingVersion: "v1", Attempt: 1}}}
	worker := NewAIRetrievalWorker(repository, asyncOutboxTxRunner{}, aiRetrievalProcessorStub{}, AsyncOutboxWorkerConfig{}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	count, err := worker.RunOnce(context.Background())
	if err != nil || count != 1 || repository.claimedWith != "lease" || len(repository.indexed) != 1 || len(repository.succeeded) != 1 || len(repository.retried) != 0 {
		t.Fatalf("count=%d err=%v repo=%+v", count, err, repository)
	}
}

func TestAIRetrievalWorkerRetriesAndAlertsAfterThreshold(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	repository := &aiRetrievalWorkerRepoStub{items: []domain.AIRetrievalOutboxItem{{ID: 2, DocumentID: "task:8", Operation: "upsert", Attempt: 5}}}
	worker := NewAIRetrievalWorker(repository, asyncOutboxTxRunner{}, aiRetrievalProcessorStub{err: errors.New("embedding unavailable")}, AsyncOutboxWorkerConfig{RetryBase: time.Minute, AlertAfterAttempt: 5}, nil)
	worker.now = func() time.Time { return now }
	worker.token = func() string { return "lease" }
	count, err := worker.RunOnce(context.Background())
	if err != nil || count != 1 || len(repository.succeeded) != 0 || len(repository.retried) != 1 || !repository.retryAlert || !repository.retryAt.Equal(now.Add(16*time.Minute)) {
		t.Fatalf("count=%d err=%v repo=%+v", count, err, repository)
	}
}
