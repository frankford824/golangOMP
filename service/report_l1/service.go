package report_l1

import (
	"context"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const CodeInvalidDateRange = "invalid_date_range"

type Service struct {
	repo     repo.ReportL1Repo
	auditLog repo.PermissionLogRepo
}

type Option func(*Service)

func WithPermissionLogRepo(auditLog repo.PermissionLogRepo) Option {
	return func(s *Service) { s.auditLog = auditLog }
}

func NewService(repo repo.ReportL1Repo, opts ...Option) *Service {
	s := &Service{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Cards(ctx context.Context, actor domain.RequestActor) ([]domain.L1Card, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/cards"); err != nil {
		return nil, err
	}
	cards, err := s.repo.GetCards(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if cards == nil {
		cards = []domain.L1Card{}
	}
	return cards, nil
}

func (s *Service) Throughput(ctx context.Context, actor domain.RequestActor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ThroughputPoint, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/throughput"); err != nil {
		return nil, err
	}
	if from.After(to) {
		return nil, domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	points, err := s.repo.GetThroughput(ctx, repo.ReportL1Filter{From: from, To: to, DepartmentID: deptID, TaskType: taskType})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if points == nil {
		points = []domain.L1ThroughputPoint{}
	}
	return points, nil
}

func (s *Service) ModuleDwell(ctx context.Context, actor domain.RequestActor, from, to time.Time, deptID *int64, taskType *string) ([]domain.L1ModuleDwellPoint, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/l1/module-dwell"); err != nil {
		return nil, err
	}
	if from.After(to) {
		return nil, domain.NewAppError(CodeInvalidDateRange, "from must be before or equal to to", nil)
	}
	points, err := s.repo.GetModuleDwell(ctx, repo.ReportL1Filter{From: from, To: to, DepartmentID: deptID, TaskType: taskType})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if points == nil {
		points = []domain.L1ModuleDwellPoint{}
	}
	return points, nil
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
