package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

type Service struct {
	repo      repo.AIRetrievalRepo
	embedding EmbeddingProvider
	vectors   VectorStore
	enabled   bool
	logger    *zap.Logger
}

func NewService(repository repo.AIRetrievalRepo, embedding EmbeddingProvider, vectors VectorStore, enabled bool, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repository, embedding: embedding, vectors: vectors, enabled: enabled, logger: logger}
}

func (s *Service) HybridReady() bool {
	return s != nil && s.enabled && s.repo != nil && s.embedding != nil && s.embedding.Ready() && s.vectors != nil && s.vectors.Ready()
}

func (s *Service) Search(ctx context.Context, actor domain.RequestActor, query string, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	query = sanitizeText(query)
	if query == "" {
		return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "exact"}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	exactLimit := 50
	var exact, dense []domain.AIRetrievalHit
	var exactErr, denseErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		exact, exactErr = s.repo.SearchRetrievalDocuments(ctx, query, exactLimit)
	}()
	if s.HybridReady() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vectors, err := s.embedding.Embed(ctx, []string{query})
			if err != nil {
				denseErr = fmt.Errorf("embed hybrid query: %w", err)
				return
			}
			if len(vectors) != 1 {
				denseErr = fmt.Errorf("embed hybrid query: expected 1 vector, got %d", len(vectors))
				return
			}
			dense, denseErr = s.vectors.Search(ctx, vectors[0], exactLimit)
		}()
	}
	wg.Wait()
	if exactErr != nil {
		return nil, domain.AIRetrievalMeta{}, exactErr
	}
	meta := domain.AIRetrievalMeta{Mode: "exact", Candidates: len(exact)}
	if s.HybridReady() && denseErr == nil {
		meta.Mode = "hybrid"
		meta.Candidates += len(dense)
	} else if s.HybridReady() && denseErr != nil {
		meta.Degraded = true
		meta.Reason = "vector_unavailable"
		s.logger.Warn("vector retrieval degraded to mysql", zap.Error(denseErr))
	}
	merged := reciprocalRankFusion(exact, dense, 60)
	visible := make([]domain.AIRetrievalHit, 0, limit)
	for _, hit := range merged {
		allowed, err := s.repo.AuthorizeRetrievalDocument(ctx, actor, hit.DocumentID)
		if err != nil {
			return nil, meta, err
		}
		if !allowed {
			continue
		}
		hit.Excerpt = sanitizeText(hit.Excerpt)
		visible = append(visible, hit)
		if len(visible) == limit {
			break
		}
	}
	return visible, meta, nil
}

func (s *Service) IndexDocument(ctx context.Context, item domain.AIRetrievalOutboxItem) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("retrieval service is not configured")
	}
	if s.embedding == nil || !s.embedding.Ready() || s.vectors == nil || !s.vectors.Ready() {
		return fmt.Errorf("vector index dependencies are unavailable")
	}
	document, err := s.repo.GetRetrievalDocument(ctx, item.DocumentID)
	if err != nil {
		return err
	}
	if item.Operation == "delete" || document.DeletedAt != nil {
		return s.vectors.Delete(ctx, item.DocumentID)
	}
	if document.ContentHash != item.ContentHash || document.EmbeddingVersion != item.EmbeddingVersion {
		return fmt.Errorf("retrieval document changed after outbox enqueue")
	}
	vectors, err := s.embedding.Embed(ctx, []string{document.SearchText})
	if err != nil {
		return err
	}
	if len(vectors) != 1 {
		return fmt.Errorf("embedding provider returned %d vectors", len(vectors))
	}
	return s.vectors.Upsert(ctx, *document, vectors[0])
}

func reciprocalRankFusion(exact, dense []domain.AIRetrievalHit, k int) []domain.AIRetrievalHit {
	if k <= 0 {
		k = 60
	}
	type scored struct {
		hit   domain.AIRetrievalHit
		score float64
	}
	byID := make(map[string]*scored)
	for _, list := range [][]domain.AIRetrievalHit{exact, dense} {
		for index, hit := range list {
			if hit.DocumentID == "" {
				continue
			}
			entry := byID[hit.DocumentID]
			if entry == nil {
				copy := hit
				entry = &scored{hit: copy}
				byID[hit.DocumentID] = entry
			}
			entry.score += 1.0 / float64(k+index+1)
			if entry.hit.Title == "" {
				entry.hit = hit
			}
		}
	}
	items := make([]scored, 0, len(byID))
	for _, item := range byID {
		item.hit.Score = item.score
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].hit.DocumentID < items[j].hit.DocumentID
		}
		return items[i].score > items[j].score
	})
	result := make([]domain.AIRetrievalHit, 0, len(items))
	for _, item := range items {
		result = append(result, item.hit)
	}
	return result
}

func sanitizeText(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r >= 0x20 {
			return r
		}
		return -1
	}, value)
	return strings.TrimSpace(value)
}
