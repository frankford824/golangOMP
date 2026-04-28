package task_aggregator

import (
	"context"
	"encoding/json"

	"workflow/domain"
	"workflow/repo"
)

type DetailService struct {
	tasks       repo.TaskRepo
	modules     repo.TaskModuleRepo
	events      repo.TaskModuleEventRepo
	refs        repo.ReferenceFileRefFlatRepo
	refEnricher referenceFileRefEnricher
	statusAgg   *StatusAggregator
}

type Detail struct {
	Task       *domain.Task              `json:"task"`
	TaskDetail *domain.TaskDetail        `json:"task_detail,omitempty"`
	Modules    []ModuleDetail            `json:"modules"`
	Events     []*domain.TaskModuleEvent `json:"events"`
	References []domain.ReferenceFileRef `json:"reference_file_refs"`
}

type ModuleDetail struct {
	*domain.TaskModule
	Visibility     string          `json:"visibility"`
	AllowedActions []string        `json:"allowed_actions"`
	Projection     json.RawMessage `json:"projection"`
}

type detailBundleReader interface {
	GetTaskDetailBundle(ctx context.Context, taskID int64, eventLimit int) (*domain.Task, *domain.TaskDetail, []*domain.TaskModule, []*domain.TaskModuleEvent, []*domain.ReferenceFileRefFlat, error)
}

type referenceFileRefEnricher interface {
	EnrichAll([]domain.ReferenceFileRef) []domain.ReferenceFileRef
}

type DetailServiceOption func(*DetailService)

func WithReferenceFileRefEnricher(enricher referenceFileRefEnricher) DetailServiceOption {
	return func(s *DetailService) {
		s.refEnricher = enricher
	}
}

func NewDetailService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, refs repo.ReferenceFileRefFlatRepo, opts ...DetailServiceOption) *DetailService {
	svc := &DetailService{tasks: tasks, modules: modules, events: events, refs: refs, statusAgg: NewStatusAggregator(modules)}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *DetailService) Get(ctx context.Context, taskID int64) (*Detail, error) {
	if reader, ok := s.tasks.(detailBundleReader); ok {
		task, detail, modules, events, refs, err := reader.GetTaskDetailBundle(ctx, taskID, 50)
		if err == nil {
			if task == nil {
				return nil, nil
			}
			return s.buildDetail(task, detail, modules, events, refs), nil
		}
	}
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, err
	}
	detail, err := s.tasks.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	modules, err := s.modules.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	events, err := s.events.ListRecentByTask(ctx, taskID, 50)
	if err != nil {
		return nil, err
	}
	refs, err := s.refs.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return s.buildDetail(task, detail, modules, events, refs), nil
}

func (s *DetailService) buildDetail(task *domain.Task, detail *domain.TaskDetail, modules []*domain.TaskModule, events []*domain.TaskModuleEvent, refs []*domain.ReferenceFileRefFlat) *Detail {
	moduleDetails := make([]ModuleDetail, 0, len(modules))
	for _, m := range modules {
		moduleDetails = append(moduleDetails, ModuleDetail{TaskModule: m, Visibility: "visible", Projection: json.RawMessage(`{}`)})
	}
	references := buildDetailReferenceFileRefs(detail, refs)
	if s != nil && s.refEnricher != nil {
		references = s.refEnricher.EnrichAll(references)
	}
	if references == nil {
		references = []domain.ReferenceFileRef{}
	}
	if detail != nil {
		if raw, err := json.Marshal(references); err == nil {
			detail.ReferenceFileRefsJSON = string(raw)
		}
	}
	return &Detail{Task: task, TaskDetail: detail, Modules: moduleDetails, Events: events, References: references}
}

func buildDetailReferenceFileRefs(detail *domain.TaskDetail, flatRefs []*domain.ReferenceFileRefFlat) []domain.ReferenceFileRef {
	if detail != nil {
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON); len(refs) > 0 {
			return refs
		}
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON); len(refs) > 0 {
			return refs
		}
	}
	if len(flatRefs) == 0 {
		return nil
	}
	refs := make([]domain.ReferenceFileRef, 0, len(flatRefs))
	for _, flat := range flatRefs {
		if flat == nil || flat.RefID == "" {
			continue
		}
		refs = append(refs, domain.ReferenceFileRef{
			AssetID: flat.RefID,
			RefID:   flat.RefID,
		})
	}
	return domain.NormalizeReferenceFileRefs(refs)
}
