package report_l1

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"workflow/domain"
	"workflow/repo"
)

const CodeInvalidDateRange = "invalid_date_range"

type Service struct {
	repo                       repo.ReportL1Repo
	cache                      ReportL1Cache
	kpiAnalysisRepo            repo.KPIAnalysisRepo
	kpiAnalysisGenerator       KPIAnalysisGenerator
	businessTrendRepo          repo.BusinessTrendRepo
	businessTrendGenerator     BusinessTrendAnalysisGenerator
	businessTrendProviders     []TrendProvider
	businessTrendProviderNames []string
	businessTrendJobs          *businessTrendDeepJobStore
	auditLog                   repo.PermissionLogRepo
}

type Option func(*Service)

type ReportL1Cache interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
}

func WithPermissionLogRepo(auditLog repo.PermissionLogRepo) Option {
	return func(s *Service) { s.auditLog = auditLog }
}

func WithReportL1Redis(cache ReportL1Cache) Option {
	return func(s *Service) { s.cache = cache }
}

func WithKPIAnalysisRepo(kpiRepo repo.KPIAnalysisRepo) Option {
	return func(s *Service) { s.kpiAnalysisRepo = kpiRepo }
}

func WithKPIAnalysisGenerator(generator KPIAnalysisGenerator) Option {
	return func(s *Service) { s.kpiAnalysisGenerator = generator }
}

func WithBusinessTrendRepo(trendRepo repo.BusinessTrendRepo) Option {
	return func(s *Service) { s.businessTrendRepo = trendRepo }
}

func WithBusinessTrendGenerator(generator BusinessTrendAnalysisGenerator) Option {
	return func(s *Service) { s.businessTrendGenerator = generator }
}

func WithBusinessTrendProviders(providers []TrendProvider, expectedNames []string) Option {
	return func(s *Service) {
		s.businessTrendProviders = append([]TrendProvider{}, providers...)
		s.businessTrendProviderNames = append([]string{}, expectedNames...)
	}
}

func NewService(repo repo.ReportL1Repo, opts ...Option) *Service {
	s := &Service{repo: repo, businessTrendJobs: newBusinessTrendDeepJobStore()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Cards(ctx context.Context, actor domain.RequestActor) ([]domain.L1Card, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/cards"); err != nil {
		return nil, err
	}
	cacheKey := "omp:perf:report-l1:cards:v1"
	if cards, ok := reportL1CacheGet[[]domain.L1Card](ctx, s.cache, cacheKey); ok {
		if cards == nil {
			cards = []domain.L1Card{}
		}
		return cards, nil
	}
	cards, err := s.repo.GetCards(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if cards == nil {
		cards = []domain.L1Card{}
	}
	reportL1CacheSet(ctx, s.cache, cacheKey, cards, time.Minute)
	return cards, nil
}

func (s *Service) Throughput(ctx context.Context, actor domain.RequestActor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ThroughputPoint, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/throughput"); err != nil {
		return nil, err
	}
	if from.After(to) {
		return nil, domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	filter := repo.ReportL1Filter{From: from, To: to, DepartmentID: deptID, TaskType: taskType}
	cacheKey := reportL1ParameterizedCacheKey("throughput", filter)
	if points, ok := reportL1CacheGet[[]domain.L1ThroughputPoint](ctx, s.cache, cacheKey); ok {
		if points == nil {
			points = []domain.L1ThroughputPoint{}
		}
		return points, nil
	}
	points, err := s.repo.GetThroughput(ctx, filter)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if points == nil {
		points = []domain.L1ThroughputPoint{}
	}
	reportL1CacheSet(ctx, s.cache, cacheKey, points, 10*time.Minute)
	return points, nil
}

func (s *Service) ModuleDwell(ctx context.Context, actor domain.RequestActor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ModuleDwellPoint, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/module-dwell"); err != nil {
		return nil, err
	}
	if from.After(to) {
		return nil, domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	filter := repo.ReportL1Filter{From: from, To: to, DepartmentID: deptID, TaskType: taskType}
	cacheKey := reportL1ParameterizedCacheKey("module-dwell", filter)
	if points, ok := reportL1CacheGet[[]domain.L1ModuleDwellPoint](ctx, s.cache, cacheKey); ok {
		if points == nil {
			points = []domain.L1ModuleDwellPoint{}
		}
		return points, nil
	}
	points, err := s.repo.GetModuleDwell(ctx, filter)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if points == nil {
		points = []domain.L1ModuleDwellPoint{}
	}
	reportL1CacheSet(ctx, s.cache, cacheKey, points, 10*time.Minute)
	return points, nil
}

func reportL1ParameterizedCacheKey(endpoint string, filter repo.ReportL1Filter) string {
	payload := struct {
		From         string  `json:"from"`
		To           string  `json:"to"`
		DepartmentID *int64  `json:"department_id,omitempty"`
		TaskType     *string `json:"task_type,omitempty"`
	}{
		From:         filter.From.UTC().Format(time.RFC3339Nano),
		To:           filter.To.UTC().Format(time.RFC3339Nano),
		DepartmentID: filter.DepartmentID,
		TaskType:     filter.TaskType,
	}
	raw, _ := json.Marshal(payload)
	sum := sha1.Sum(raw)
	return "omp:perf:report-l1:" + endpoint + ":v1:" + hex.EncodeToString(sum[:])
}

func reportL1CacheGet[T any](ctx context.Context, cache ReportL1Cache, key string) (T, bool) {
	var out T
	if cache == nil {
		return out, false
	}
	raw, err := cache.Get(ctx, key).Result()
	if err != nil {
		return out, false
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return out, false
	}
	return out, true
}

func reportL1CacheSet(ctx context.Context, cache ReportL1Cache, key string, value interface{}, ttl time.Duration) {
	if cache == nil || ttl <= 0 {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = cache.Set(ctx, key, raw, ttl).Err()
}

func (s *Service) requireSuperAdmin(ctx context.Context, actor domain.RequestActor, path string) *domain.AppError {
	for _, role := range actor.Roles {
		if role == domain.RoleSuperAdmin {
			return nil
		}
	}
	s.auditDenied(ctx, actor, path)
	return domain.NewAppError(domain.ErrCodePermissionDenied, "reports require super admin", map[string]string{
		"deny_code": domain.ErrDenyCodeReportsSuperAdminOnly,
	})
}

func (s *Service) auditDenied(ctx context.Context, actor domain.RequestActor, path string) {
	if s.auditLog == nil {
		return
	}
	actorID := actor.ID
	_ = s.auditLog.Create(ctx, &domain.PermissionLog{
		ActorID:         &actorID,
		ActorUsername:   actor.Username,
		ActorSource:     actor.Source,
		AuthMode:        actor.AuthMode,
		Readiness:       domain.APIReadinessReadyForFrontend,
		SessionRequired: true,
		DebugCompatible: false,
		ActionType:      "report_access_denied",
		ActorRoles:      actor.Roles,
		Method:          "GET",
		RoutePath:       path,
		RequiredRoles:   []domain.Role{domain.RoleSuperAdmin},
		Granted:         false,
		Reason:          "not_super_admin",
		CreatedAt:       time.Now().UTC(),
	})
}
