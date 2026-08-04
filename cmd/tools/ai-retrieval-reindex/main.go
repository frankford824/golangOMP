package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"workflow/config"
	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	retrievalsvc "workflow/service/retrieval"
)

type report struct {
	DryRun           bool                                `json:"dry_run"`
	TargetCollection string                              `json:"target_collection,omitempty"`
	Projection       mysqlrepo.AIRetrievalReindexSummary `json:"projection"`
	Indexed          int                                 `json:"indexed"`
	AliasSwitched    bool                                `json:"alias_switched"`
	SnapshotCreated  bool                                `json:"snapshot_created"`
	ElapsedMS        int64                               `json:"elapsed_ms"`
	Error            string                              `json:"error,omitempty"`
}

func main() {
	var dsn, target string
	var apply, switchAlias, snapshot bool
	var batch int
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to MYSQL_DSN")
	flag.StringVar(&target, "target-collection", "", "new versioned Qdrant collection")
	flag.BoolVar(&apply, "apply", false, "write MySQL projections and Qdrant index")
	flag.BoolVar(&switchAlias, "switch-alias", false, "atomically switch the stable alias after a complete build")
	flag.BoolVar(&snapshot, "snapshot", true, "create a Qdrant snapshot before alias switch")
	flag.IntVar(&batch, "batch-size", 200, "projection/index batch size")
	flag.DurationVar(&timeout, "timeout", 30*time.Minute, "whole-run timeout")
	flag.Parse()

	started := time.Now()
	out, err := run(context.Background(), dsn, target, apply, switchAlias, snapshot, batch, timeout)
	out.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Error = err.Error()
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(out)
	if err != nil {
		os.Exit(1)
	}
}

func run(parent context.Context, dsn, target string, apply, switchAlias, snapshot bool, batch int, timeout time.Duration) (report, error) {
	cfg, err := config.Load()
	if err != nil {
		return report{}, err
	}
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.MySQL.DSN
	}
	if strings.TrimSpace(dsn) == "" {
		return report{}, fmt.Errorf("mysql dsn is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return report{}, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return report{}, err
	}
	mdb := mysqlrepo.New(db)
	projector := mysqlrepo.NewAIRetrievalProjector(mdb)
	out := report{DryRun: !apply, TargetCollection: strings.TrimSpace(target)}
	if !apply {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents`).Scan(&out.Projection.Tasks); err != nil {
			return out, err
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_search_documents`).Scan(&out.Projection.ResourceGroups); err != nil {
			return out, err
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records WHERE status='indexed' AND is_dir=0`).Scan(&out.Projection.ExternalAssets); err != nil {
			return out, err
		}
		return out, nil
	}
	if !cfg.Embedding.Enabled || !cfg.VectorSearch.Enabled {
		return out, fmt.Errorf("embedding and vector search must be enabled for apply")
	}
	if strings.TrimSpace(target) == "" {
		return out, fmt.Errorf("--target-collection is required for apply")
	}
	if target == cfg.VectorSearch.CollectionAlias {
		return out, fmt.Errorf("target collection must be versioned and differ from stable alias")
	}
	out.Projection, err = projector.Rebuild(ctx, cfg.VectorSearch.EmbeddingVersion, batch)
	if err != nil {
		return out, err
	}
	embedding := retrievalsvc.NewOpenAICompatibleEmbeddingClient(retrievalsvc.EmbeddingConfig{
		Enabled: cfg.Embedding.Enabled, BaseURL: cfg.Embedding.BaseURL, APIKey: cfg.Embedding.APIKey,
		Model: cfg.Embedding.Model, Dimensions: cfg.Embedding.Dimensions, Timeout: cfg.Embedding.Timeout,
	})
	vectors := retrievalsvc.NewQdrantClient(retrievalsvc.QdrantConfig{
		Enabled: true, BaseURL: cfg.VectorSearch.BaseURL, APIKey: cfg.VectorSearch.APIKey,
		CollectionAlias: target, Timeout: cfg.VectorSearch.Timeout,
	})
	if err := vectors.EnsureCollection(ctx, target, cfg.Embedding.Dimensions); err != nil {
		return out, err
	}
	repository := mysqlrepo.NewAIRetrievalRepo(mdb)
	indexer := retrievalsvc.NewService(repository, embedding, vectors, true, zap.NewNop())
	for cursor := ""; ; {
		ids, err := projector.ListDocuments(ctx, cursor, batch)
		if err != nil {
			return out, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			document, err := repository.GetRetrievalDocument(ctx, id)
			if err != nil {
				return out, err
			}
			item := domain.AIRetrievalOutboxItem{DocumentID: id, Operation: "upsert", ContentHash: document.ContentHash, EmbeddingVersion: document.EmbeddingVersion}
			if err := indexer.IndexDocument(ctx, item); err != nil {
				return out, fmt.Errorf("index %s: %w", id, err)
			}
			if err := projector.MarkIndexed(ctx, id); err != nil {
				return out, err
			}
			out.Indexed++
		}
		cursor = ids[len(ids)-1]
	}
	if int64(out.Indexed) != out.Projection.Documents {
		return out, fmt.Errorf("indexed %d documents, expected %d", out.Indexed, out.Projection.Documents)
	}
	if switchAlias {
		if snapshot {
			if err := vectors.CreateSnapshot(ctx, target); err != nil {
				return out, err
			}
			out.SnapshotCreated = true
		}
		if err := vectors.SwitchAlias(ctx, cfg.VectorSearch.CollectionAlias, target); err != nil {
			return out, err
		}
		out.AliasSwitched = true
	}
	return out, nil
}
