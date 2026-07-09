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
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/config"
	"workflow/service/aiagent"
)

type semanticCandidate struct {
	Kind string
	ID   string
	Text string
}

type semanticSummary struct {
	DryRun    bool     `json:"dry_run"`
	Kinds     []string `json:"kinds"`
	Scanned   int      `json:"scanned"`
	Enriched  int      `json:"enriched"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	ElapsedMS int64    `json:"elapsed_ms"`
	Message   string   `json:"message,omitempty"`
}

func main() {
	var dsn string
	var kind string
	var limit int
	var dryRun bool
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.StringVar(&kind, "kind", "both", "documents to enrich: assets, products, or both")
	flag.IntVar(&limit, "limit", 20, "maximum documents to enrich")
	flag.BoolVar(&dryRun, "dry-run", false, "list candidates without calling AI or updating rows")
	flag.DurationVar(&timeout, "timeout", 10*time.Minute, "whole run timeout")
	flag.Parse()

	code := run(context.Background(), semanticOptions{
		DSN:     dsn,
		Kind:    kind,
		Limit:   limit,
		DryRun:  dryRun,
		Timeout: timeout,
	})
	os.Exit(code)
}

type semanticOptions struct {
	DSN     string
	Kind    string
	Limit   int
	DryRun  bool
	Timeout time.Duration
}

func run(parent context.Context, opts semanticOptions) int {
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, positiveDuration(opts.Timeout, 10*time.Minute))
	defer cancel()
	kinds, err := semanticKinds(opts.Kind)
	if err != nil {
		writeSemanticError(err.Error())
		return 2
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	cfg, err := config.Load()
	if err != nil {
		writeSemanticError(fmt.Sprintf("load config: %v", err))
		return 1
	}
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		dsn = cfg.MySQL.DSN
	}
	if dsn == "" {
		writeSemanticError("mysql dsn is required")
		return 2
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		writeSemanticError(fmt.Sprintf("open mysql: %v", err))
		return 1
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		writeSemanticError(fmt.Sprintf("ping mysql: %v", err))
		return 1
	}

	candidates, err := loadSemanticCandidates(ctx, db, kinds, opts.Limit)
	if err != nil {
		writeSemanticError(fmt.Sprintf("load candidates: %v", err))
		return 1
	}
	out := semanticSummary{DryRun: opts.DryRun, Kinds: kinds, Scanned: len(candidates)}
	if opts.DryRun {
		out.Skipped = len(candidates)
		out.Message = "dry run only; no AI calls or row updates"
		out.ElapsedMS = time.Since(start).Milliseconds()
		writeSemanticJSON(out)
		return 0
	}
	client, cleanup, err := buildAIClient(ctx, cfg)
	if err != nil {
		writeSemanticError(err.Error())
		return 1
	}
	defer cleanup()
	for _, candidate := range candidates {
		terms, err := client.GenerateSearchSemanticTerms(ctx, aiagent.SearchSemanticEvidence{
			Kind: candidate.Kind,
			ID:   candidate.ID,
			Text: candidate.Text,
		})
		if err != nil {
			out.Failed++
			continue
		}
		if len(terms.Terms) == 0 {
			out.Skipped++
			continue
		}
		if err := updateSemanticDocument(ctx, db, candidate, strings.Join(terms.Terms, " ")); err != nil {
			out.Failed++
			continue
		}
		out.Enriched++
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	writeSemanticJSON(out)
	if out.Failed > 0 {
		return 1
	}
	return 0
}

func semanticKinds(raw string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "both", "all":
		return []string{"assets", "products"}, nil
	case "assets", "asset":
		return []string{"assets"}, nil
	case "products", "product":
		return []string{"products"}, nil
	default:
		return nil, fmt.Errorf("invalid --kind %q", raw)
	}
}

func loadSemanticCandidates(ctx context.Context, db *sql.DB, kinds []string, limit int) ([]semanticCandidate, error) {
	perKind := limit
	if len(kinds) > 1 {
		perKind = (limit + len(kinds) - 1) / len(kinds)
	}
	out := make([]semanticCandidate, 0, limit)
	for _, kind := range kinds {
		var rows *sql.Rows
		var err error
		switch kind {
		case "assets":
			rows, err = db.QueryContext(ctx, `
				SELECT CAST(asset_id AS CHAR) AS id, COALESCE(search_text, '') AS text_value
				  FROM asset_search_documents
				 WHERE COALESCE(semantic_text, '') = ''
				 ORDER BY updated_at DESC, asset_id DESC
				 LIMIT ?`, perKind)
		case "products":
			rows, err = db.QueryContext(ctx, `
				SELECT sku_code AS id, COALESCE(search_text, '') AS text_value
				  FROM product_search_documents
				 WHERE COALESCE(semantic_text, '') = ''
				 ORDER BY updated_at DESC, sku_code DESC
				 LIMIT ?`, perKind)
		}
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item semanticCandidate
			item.Kind = kind
			if err := rows.Scan(&item.ID, &item.Text); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, item)
			if len(out) >= limit {
				rows.Close()
				return out, nil
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

func updateSemanticDocument(ctx context.Context, db *sql.DB, candidate semanticCandidate, semanticText string) error {
	switch candidate.Kind {
	case "assets":
		_, err := db.ExecContext(ctx, `
			UPDATE asset_search_documents
			   SET semantic_text = ?, semantic_enriched_at = CURRENT_TIMESTAMP
			 WHERE asset_id = ?`, semanticText, candidate.ID)
		return err
	case "products":
		_, err := db.ExecContext(ctx, `
			UPDATE product_search_documents
			   SET semantic_text = ?, semantic_enriched_at = CURRENT_TIMESTAMP
			 WHERE sku_code = ?`, semanticText, candidate.ID)
		return err
	default:
		return fmt.Errorf("unsupported candidate kind %q", candidate.Kind)
	}
}

func buildAIClient(ctx context.Context, cfg *config.Config) (*aiagent.AnthropicCompatibleClient, func(), error) {
	if !aiagent.NewAnthropicCompatibleClient(aiagent.Config{
		Enabled: cfg.AI.Enabled,
		BaseURL: cfg.AI.BaseURL,
		APIKey:  cfg.AI.APIKey,
		Model:   cfg.AI.Model,
	}, zap.NewNop()).Ready() {
		return nil, func() {}, fmt.Errorf("ai search semantic provider is not configured")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, func() {}, fmt.Errorf("redis ping for AI rate limiter: %w", err)
	}
	cleanup := func() { _ = rdb.Close() }
	limiter := aiagent.NewRedisAIRateLimiter(rdb, "omp")
	client := aiagent.NewAnthropicCompatibleClient(aiagent.Config{
		Enabled:         cfg.AI.Enabled,
		Provider:        cfg.AI.Provider,
		BaseURL:         cfg.AI.BaseURL,
		APIKey:          cfg.AI.APIKey,
		Model:           cfg.AI.Model,
		Timeout:         cfg.AI.Timeout,
		MaxTokens:       cfg.AI.MaxTokens,
		RateLimitWindow: cfg.AI.RateLimitWindow,
		RateLimitMax:    cfg.AI.RateLimitMax,
		RateLimiter:     limiter,
	}, zap.NewNop())
	return client, cleanup, nil
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func writeSemanticJSON(out semanticSummary) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode output: %v\n", err)
		os.Exit(1)
	}
}

func writeSemanticError(message string) {
	writeSemanticJSON(semanticSummary{Message: message})
}
