package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

type customizationFlowJobRepo struct {
	nextID int64
	jobs   map[int64]*domain.CustomizationJob
}

func newCustomizationFlowJobRepo(jobs ...*domain.CustomizationJob) *customizationFlowJobRepo {
	store := make(map[int64]*domain.CustomizationJob, len(jobs))
	var maxID int64
	for _, job := range jobs {
		if job == nil {
			continue
		}
		copied := *job
		store[copied.ID] = &copied
		if copied.ID > maxID {
			maxID = copied.ID
		}
	}
	return &customizationFlowJobRepo{nextID: maxID + 1, jobs: store}
}

func (r *customizationFlowJobRepo) Create(_ context.Context, _ repo.Tx, job *domain.CustomizationJob) (int64, error) {
	if r.jobs == nil {
		r.jobs = map[int64]*domain.CustomizationJob{}
	}
	id := r.nextID
	if id == 0 {
		id = 1
	}
	r.nextID = id + 1
	copied := *job
	copied.ID = id
	r.jobs[id] = &copied
	return id, nil
}

func (r *customizationFlowJobRepo) GetByID(_ context.Context, id int64) (*domain.CustomizationJob, error) {
	item := r.jobs[id]
	if item == nil {
		return nil, nil
	}
	copied := *item
	return &copied, nil
}

func (r *customizationFlowJobRepo) GetLatestByTaskID(_ context.Context, taskID int64) (*domain.CustomizationJob, error) {
	return r.latest(taskID), nil
}

func (r *customizationFlowJobRepo) GetLatestByTaskIDForUpdate(_ context.Context, _ repo.Tx, taskID int64) (*domain.CustomizationJob, error) {
	return r.latest(taskID), nil
}

func (r *customizationFlowJobRepo) latest(taskID int64) *domain.CustomizationJob {
	var latest *domain.CustomizationJob
	for _, item := range r.jobs {
		if item == nil || item.TaskID != taskID || (latest != nil && item.ID <= latest.ID) {
			continue
		}
		copied := *item
		latest = &copied
	}
	return latest
}

func (r *customizationFlowJobRepo) List(_ context.Context, filter repo.CustomizationJobListFilter) ([]*domain.CustomizationJob, int64, error) {
	items := make([]*domain.CustomizationJob, 0)
	for _, item := range r.jobs {
		if item == nil || (filter.TaskID != nil && item.TaskID != *filter.TaskID) || (filter.Status != nil && item.Status != *filter.Status) {
			continue
		}
		copied := *item
		items = append(items, &copied)
	}
	return items, int64(len(items)), nil
}

func (r *customizationFlowJobRepo) Update(_ context.Context, _ repo.Tx, job *domain.CustomizationJob) error {
	if r.jobs == nil {
		r.jobs = map[int64]*domain.CustomizationJob{}
	}
	copied := *job
	r.jobs[job.ID] = &copied
	return nil
}
