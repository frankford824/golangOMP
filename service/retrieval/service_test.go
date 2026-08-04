package retrieval

import (
	"context"
	"errors"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type retrievalRepoStub struct {
	exact     []domain.AIRetrievalHit
	exactErr  error
	allowed   map[string]bool
	authErr   error
	document  *domain.AIRetrievalDocument
	indexedID string
}

func (r *retrievalRepoStub) UpsertRetrievalDocument(context.Context, repo.Tx, domain.AIRetrievalDocument) error {
	return nil
}
func (r *retrievalRepoStub) GetRetrievalDocument(context.Context, string) (*domain.AIRetrievalDocument, error) {
	if r.document == nil {
		return nil, errors.New("missing")
	}
	copy := *r.document
	return &copy, nil
}
func (r *retrievalRepoStub) SearchRetrievalDocuments(context.Context, string, int) ([]domain.AIRetrievalHit, error) {
	return append([]domain.AIRetrievalHit{}, r.exact...), r.exactErr
}
func (r *retrievalRepoStub) AuthorizeRetrievalDocument(_ context.Context, _ domain.RequestActor, id string) (bool, error) {
	if r.authErr != nil {
		return false, r.authErr
	}
	return r.allowed[id], nil
}
func (r *retrievalRepoStub) EnqueueRetrievalDocument(context.Context, repo.Tx, domain.AIRetrievalOutboxItem) error {
	return nil
}
func (r *retrievalRepoStub) ClaimRetrievalOutbox(context.Context, repo.Tx, string, time.Time, time.Time, int) ([]domain.AIRetrievalOutboxItem, error) {
	return nil, nil
}
func (r *retrievalRepoStub) MarkRetrievalOutboxSucceeded(context.Context, repo.Tx, int64, string, time.Time) error {
	return nil
}
func (r *retrievalRepoStub) MarkRetrievalOutboxRetry(context.Context, repo.Tx, int64, string, string, time.Time, bool) error {
	return nil
}
func (r *retrievalRepoStub) MarkRetrievalDocumentIndexed(_ context.Context, _ repo.Tx, id, _, _ string, _ time.Time) error {
	r.indexedID = id
	return nil
}

type embeddingStub struct {
	vector []float32
	err    error
	ready  bool
}

func (s embeddingStub) Ready() bool     { return s.ready }
func (s embeddingStub) Version() string { return "test:3" }
func (s embeddingStub) Embed(context.Context, []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	return [][]float32{append([]float32{}, s.vector...)}, nil
}

type vectorStub struct {
	hits     []domain.AIRetrievalHit
	err      error
	ready    bool
	upserted string
	deleted  string
}

func (s *vectorStub) Ready() bool { return s.ready }
func (s *vectorStub) Search(context.Context, []float32, int) ([]domain.AIRetrievalHit, error) {
	return append([]domain.AIRetrievalHit{}, s.hits...), s.err
}
func (s *vectorStub) Upsert(_ context.Context, document domain.AIRetrievalDocument, _ []float32) error {
	s.upserted = document.DocumentID
	return s.err
}
func (s *vectorStub) Delete(_ context.Context, id string) error { s.deleted = id; return s.err }

func TestReciprocalRankFusionDeduplicatesAndRanksCrossSourceHits(t *testing.T) {
	items := reciprocalRankFusion(
		[]domain.AIRetrievalHit{{DocumentID: "a"}, {DocumentID: "b"}},
		[]domain.AIRetrievalHit{{DocumentID: "b"}, {DocumentID: "c"}}, 60,
	)
	if len(items) != 3 || items[0].DocumentID != "b" {
		t.Fatalf("items=%+v", items)
	}
	if items[0].Score <= items[1].Score {
		t.Fatalf("cross-source result should rank first: %+v", items)
	}
}

func TestSearchHybridRechecksCurrentAuthorization(t *testing.T) {
	r := &retrievalRepoStub{
		exact:   []domain.AIRetrievalHit{{DocumentID: "exact-only", Title: "A"}, {DocumentID: "shared", Title: "B"}},
		allowed: map[string]bool{"shared": true, "dense-only": true},
	}
	v := &vectorStub{ready: true, hits: []domain.AIRetrievalHit{{DocumentID: "shared", Title: "B"}, {DocumentID: "dense-only", Title: "C"}}}
	svc := NewService(r, embeddingStub{ready: true, vector: []float32{1, 2, 3}}, v, true, nil)
	hits, meta, err := svc.Search(context.Background(), domain.RequestActor{ID: 8}, "任务延期原因", 20)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Mode != "hybrid" || meta.Degraded || meta.Candidates != 4 {
		t.Fatalf("meta=%+v", meta)
	}
	if len(hits) != 2 || hits[0].DocumentID != "shared" || hits[1].DocumentID != "dense-only" {
		t.Fatalf("permission-filtered hits=%+v", hits)
	}
}

func TestSearchDegradesToExactWhenVectorFails(t *testing.T) {
	r := &retrievalRepoStub{exact: []domain.AIRetrievalHit{{DocumentID: "exact", Title: "A"}}, allowed: map[string]bool{"exact": true}}
	v := &vectorStub{ready: true, err: errors.New("qdrant timeout")}
	svc := NewService(r, embeddingStub{ready: true, vector: []float32{1, 2, 3}}, v, true, nil)
	hits, meta, err := svc.Search(context.Background(), domain.RequestActor{ID: 8}, "延期", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !meta.Degraded || meta.Reason != "vector_unavailable" || meta.Mode != "exact" {
		t.Fatalf("hits=%+v meta=%+v", hits, meta)
	}
}

func TestIndexDocumentRejectsStaleOutboxAndHandlesDelete(t *testing.T) {
	document := &domain.AIRetrievalDocument{DocumentID: "doc", SearchText: "text", ContentHash: "new", EmbeddingVersion: "v2"}
	r := &retrievalRepoStub{document: document}
	v := &vectorStub{ready: true}
	svc := NewService(r, embeddingStub{ready: true, vector: []float32{1, 2, 3}}, v, true, nil)
	if err := svc.IndexDocument(context.Background(), domain.AIRetrievalOutboxItem{DocumentID: "doc", Operation: "upsert", ContentHash: "old", EmbeddingVersion: "v2"}); err == nil {
		t.Fatal("stale outbox must be rejected")
	}
	if v.upserted != "" {
		t.Fatalf("stale document reached vector store: %q", v.upserted)
	}
	if err := svc.IndexDocument(context.Background(), domain.AIRetrievalOutboxItem{DocumentID: "doc", Operation: "delete"}); err != nil {
		t.Fatal(err)
	}
	if v.deleted != "doc" {
		t.Fatalf("deleted=%q", v.deleted)
	}
}
