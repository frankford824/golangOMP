package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

// ERPProductProvider fetches ERP product records from an external source.
type ERPProductProvider interface {
	FetchProducts(ctx context.Context) ([]domain.ERPProductRecord, error)
}

// ERPSyncOptions configures ERP sync placeholder behavior.
type ERPSyncOptions struct {
	SchedulerEnabled bool
	Interval         time.Duration
	SourceMode       string
	StubFile         string
	Timeout          time.Duration
	// Logger optional; when set, emits erp_sync_run_start/finish for live evidence.
	Logger *zap.Logger
}

// ERPSyncService coordinates ERP product synchronization.
type ERPSyncService interface {
	RunManual(ctx context.Context) (*domain.ERPSyncRunResult, *domain.AppError)
	RunScheduled(ctx context.Context) (*domain.ERPSyncRunResult, *domain.AppError)
	GetStatus(ctx context.Context) (*domain.ERPSyncStatus, *domain.AppError)
}

type erpSyncService struct {
	productRepo repo.ProductRepo
	runRepo     repo.ERPSyncRunRepo
	txRunner    repo.TxRunner
	provider    ERPProductProvider
	options     ERPSyncOptions
}

func NewERPSyncService(
	productRepo repo.ProductRepo,
	runRepo repo.ERPSyncRunRepo,
	txRunner repo.TxRunner,
	provider ERPProductProvider,
	options ERPSyncOptions,
) ERPSyncService {
	return &erpSyncService{
		productRepo: productRepo,
		runRepo:     runRepo,
		txRunner:    txRunner,
		provider:    provider,
		options:     options,
	}
}

// StubERPProductProvider reads placeholder ERP products from a local JSON file.
type StubERPProductProvider struct {
	path string
}

func NewStubERPProductProvider(path string) ERPProductProvider {
	return &StubERPProductProvider{path: path}
}

func (p *StubERPProductProvider) FetchProducts(_ context.Context) ([]domain.ERPProductRecord, error) {
	type stubProductRecord struct {
		ERPProductID    string          `json:"erp_product_id"`
		SKUCode         string          `json:"sku_code"`
		ProductName     string          `json:"product_name"`
		Category        string          `json:"category"`
		SpecJSON        json.RawMessage `json:"spec_json"`
		Status          string          `json:"status"`
		SourceUpdatedAt *time.Time      `json:"source_updated_at"`
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}

	var raw []stubProductRecord
	if err = json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode ERP stub file: %w", err)
	}

	records := make([]domain.ERPProductRecord, 0, len(raw))
	for _, item := range raw {
		specJSON := "{}"
		if len(item.SpecJSON) > 0 {
			specJSON = string(item.SpecJSON)
		}
		status := item.Status
		if status == "" {
			status = "active"
		}
		records = append(records, domain.ERPProductRecord{
			ERPProductID:    item.ERPProductID,
			SKUCode:         item.SKUCode,
			ProductName:     item.ProductName,
			Category:        item.Category,
			SpecJSON:        specJSON,
			Status:          status,
			SourceUpdatedAt: item.SourceUpdatedAt,
		})
	}
	return records, nil
}

func (s *erpSyncService) RunManual(ctx context.Context) (*domain.ERPSyncRunResult, *domain.AppError) {
	return s.run(ctx, domain.ERPSyncTriggerManual)
}

func (s *erpSyncService) RunScheduled(ctx context.Context) (*domain.ERPSyncRunResult, *domain.AppError) {
	return s.run(ctx, domain.ERPSyncTriggerScheduled)
}

func (s *erpSyncService) GetStatus(ctx context.Context) (*domain.ERPSyncStatus, *domain.AppError) {
	latestRun, err := s.runRepo.GetLatest(ctx)
	if err != nil {
		return nil, infraError("get ERP sync status", err)
	}
	resolvedStubFile, stubFileExists := resolveERPSyncStubFile(s.options.StubFile)
	return &domain.ERPSyncStatus{
		Placeholder:      true,
		SchedulerEnabled: s.options.SchedulerEnabled,
		IntervalSeconds:  int64(s.options.Interval / time.Second),
		SourceMode:       s.options.SourceMode,
		StubFile:         s.options.StubFile,
		ResolvedStubFile: resolvedStubFile,
		StubFileExists:   stubFileExists,
		LatestRun:        latestRun,
	}, nil
}

func (s *erpSyncService) run(ctx context.Context, triggerMode domain.ERPSyncTriggerMode) (*domain.ERPSyncRunResult, *domain.AppError) {
	startedAt := time.Now()
	providerType := "stub"
	if strings.EqualFold(strings.TrimSpace(s.options.SourceMode), "jst") ||
		strings.EqualFold(strings.TrimSpace(s.options.SourceMode), "jst_openweb") ||
		strings.EqualFold(strings.TrimSpace(s.options.SourceMode), "remote_jst") {
		providerType = "JSTOpenWebProductProvider"
	}
	if s.options.Logger != nil {
		s.options.Logger.Info("erp_sync_run_start",
			zap.String("source_mode", s.options.SourceMode),
			zap.String("trigger", string(triggerMode)),
			zap.String("provider", providerType),
		)
	}
	fetchCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()

	records, err := s.provider.FetchProducts(fetchCtx)
	switch {
	case err == nil:
		return s.handleFetchedRecords(ctx, triggerMode, startedAt, records)
	case errors.Is(err, os.ErrNotExist):
		return s.persistNonSuccessRun(ctx, &domain.ERPSyncRun{
			TriggerMode:   triggerMode,
			SourceMode:    s.options.SourceMode,
			Status:        domain.ERPSyncStatusNoop,
			TotalReceived: 0,
			TotalUpserted: 0,
			StartedAt:     startedAt,
			FinishedAt:    time.Now(),
		})
	default:
		msg := err.Error()
		return s.persistNonSuccessRun(ctx, &domain.ERPSyncRun{
			TriggerMode:   triggerMode,
			SourceMode:    s.options.SourceMode,
			Status:        domain.ERPSyncStatusFailed,
			TotalReceived: 0,
			TotalUpserted: 0,
			ErrorMessage:  &msg,
			StartedAt:     startedAt,
			FinishedAt:    time.Now(),
		})
	}
}

func (s *erpSyncService) handleFetchedRecords(
	ctx context.Context,
	triggerMode domain.ERPSyncTriggerMode,
	startedAt time.Time,
	records []domain.ERPProductRecord,
) (*domain.ERPSyncRunResult, *domain.AppError) {
	products := make([]*domain.Product, 0, len(records))
	for _, record := range records {
		if record.ERPProductID == "" || record.SKUCode == "" || record.ProductName == "" {
			msg := "erp_product_id, sku_code, and product_name are required in ERP records"
			return s.persistNonSuccessRun(ctx, &domain.ERPSyncRun{
				TriggerMode:   triggerMode,
				SourceMode:    s.options.SourceMode,
				Status:        domain.ERPSyncStatusFailed,
				TotalReceived: int64(len(records)),
				TotalUpserted: 0,
				ErrorMessage:  &msg,
				StartedAt:     startedAt,
				FinishedAt:    time.Now(),
			})
		}
		status := record.Status
		if status == "" {
			status = "active"
		}
		specJSON := record.SpecJSON
		if specJSON == "" {
			specJSON = "{}"
		}
		products = append(products, &domain.Product{
			ERPProductID:    record.ERPProductID,
			SKUCode:         record.SKUCode,
			ProductName:     record.ProductName,
			Category:        record.Category,
			SpecJSON:        specJSON,
			Status:          status,
			SourceUpdatedAt: record.SourceUpdatedAt,
		})
	}

	run := &domain.ERPSyncRun{
		TriggerMode:   triggerMode,
		SourceMode:    s.options.SourceMode,
		Status:        domain.ERPSyncStatusSuccess,
		TotalReceived: int64(len(records)),
		TotalUpserted: int64(len(products)),
		StartedAt:     startedAt,
		FinishedAt:    time.Now(),
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.productRepo.UpsertBatch(ctx, tx, products); err != nil {
			return err
		}
		_, err := s.runRepo.Create(ctx, tx, run)
		return err
	}); err != nil {
		msg := err.Error()
		return s.persistNonSuccessRun(ctx, &domain.ERPSyncRun{
			TriggerMode:   triggerMode,
			SourceMode:    s.options.SourceMode,
			Status:        domain.ERPSyncStatusFailed,
			TotalReceived: int64(len(records)),
			TotalUpserted: 0,
			ErrorMessage:  &msg,
			StartedAt:     startedAt,
			FinishedAt:    time.Now(),
		})
	}

	if s.options.Logger != nil {
		sampleSKU := ""
		if len(records) > 0 {
			sampleSKU = records[0].SKUCode
		}
		s.options.Logger.Info("erp_sync_run_finish",
			zap.String("status", "success"),
			zap.String("source_mode", s.options.SourceMode),
			zap.Int64("total_received", run.TotalReceived),
			zap.Int64("total_upserted", run.TotalUpserted),
			zap.String("sample_sku", sampleSKU),
		)
	}
	return toERPSyncRunResult(run), nil
}

func (s *erpSyncService) persistNonSuccessRun(ctx context.Context, run *domain.ERPSyncRun) (*domain.ERPSyncRunResult, *domain.AppError) {
	if s.options.Logger != nil {
		fields := []zap.Field{
			zap.String("status", string(run.Status)),
			zap.String("source_mode", s.options.SourceMode),
			zap.Int64("total_received", run.TotalReceived),
			zap.Int64("total_upserted", run.TotalUpserted),
		}
		if run.ErrorMessage != nil {
			fields = append(fields, zap.String("error", *run.ErrorMessage))
		}
		s.options.Logger.Info("erp_sync_run_finish", fields...)
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		_, err := s.runRepo.Create(ctx, tx, run)
		return err
	}); err != nil {
		return nil, infraError("persist ERP sync run", err)
	}
	return toERPSyncRunResult(run), nil
}

func toERPSyncRunResult(run *domain.ERPSyncRun) *domain.ERPSyncRunResult {
	if run == nil {
		return nil
	}
	return &domain.ERPSyncRunResult{
		TriggerMode:   run.TriggerMode,
		SourceMode:    run.SourceMode,
		Status:        run.Status,
		TotalReceived: run.TotalReceived,
		TotalUpserted: run.TotalUpserted,
		ErrorMessage:  run.ErrorMessage,
		StartedAt:     run.StartedAt,
		FinishedAt:    run.FinishedAt,
	}
}

func resolveERPSyncStubFile(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}

	resolved := path
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
			resolved = filepath.Join(cwd, path)
		}
	}

	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return resolved, false
	}
	return resolved, true
}
