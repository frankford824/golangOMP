package task_aggregator

import (
	"context"
	"encoding/json"

	"workflow/domain"
	"workflow/repo"
)

type DetailService struct {
	tasks     repo.TaskRepo
	modules   repo.TaskModuleRepo
	events    repo.TaskModuleEventRepo
	refs      repo.ReferenceFileRefFlatRepo
	statusAgg *StatusAggregator
}

type Detail struct {
	Task       *domain.Task                   `json:"task"`
	TaskDetail *domain.TaskDetail             `json:"task_detail,omitempty"`
	Modules    []ModuleDetail                 `json:"modules"`
	Events     []*domain.TaskModuleEvent      `json:"events"`
	References []*domain.ReferenceFileRefFlat `json:"reference_file_refs"`
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

func NewDetailService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, refs repo.ReferenceFileRefFlatRepo) *DetailService {
	return &DetailService{tasks: tasks, modules: modules, events: events, refs: refs, statusAgg: NewStatusAggregator(modules)}
}

func (s *DetailService) Get(ctx context.Context, taskID int64) (*Detail, error) {
	if reader, ok := s.tasks.(detailBundleReader); ok {
		task, detail, modules, events, refs, err := reader.GetTaskDetailBundle(ctx, taskID, 50)
		if err == nil {
			if task == nil {
				return nil, nil
			}
			return buildDetail(task, detail, modules, events, refs), nil
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
	return buildDetail(task, detail, modules, events, refs), nil
}

func buildDetail(task *domain.Task, detail *domain.TaskDetail, modules []*domain.TaskModule, events []*domain.TaskModuleEvent, refs []*domain.ReferenceFileRefFlat) *Detail {
	moduleDetails := make([]ModuleDetail, 0, len(modules))
	for _, m := range modules {
		moduleDetails = append(moduleDetails, ModuleDetail{TaskModule: m, Visibility: "visible", Projection: json.RawMessage(`{}`)})
	}
	return &Detail{Task: task, TaskDetail: detail, Modules: moduleDetails, Events: events, References: refs}
}
