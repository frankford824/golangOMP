package service

import (
	"context"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type integrationCallLogRepoStub struct {
	nextID int64
	logs   map[int64]*domain.IntegrationCallLog
}

func newIntegrationCallLogRepoStub() *integrationCallLogRepoStub {
	return &integrationCallLogRepoStub{nextID: 1, logs: map[int64]*domain.IntegrationCallLog{}}
}

func (r *integrationCallLogRepoStub) Create(_ context.Context, _ repo.Tx, log *domain.IntegrationCallLog) (int64, error) {
	if log == nil {
		return 0, fmt.Errorf("log is nil")
	}
	copyLog := *log
	copyLog.CallLogID = r.nextID
	r.logs[r.nextID] = &copyLog
	r.nextID++
	return copyLog.CallLogID, nil
}

func (r *integrationCallLogRepoStub) GetByID(_ context.Context, id int64) (*domain.IntegrationCallLog, error) {
	log, ok := r.logs[id]
	if !ok {
		return nil, nil
	}
	copyLog := *log
	return &copyLog, nil
}

func (r *integrationCallLogRepoStub) List(_ context.Context, filter repo.IntegrationCallLogListFilter) ([]*domain.IntegrationCallLog, int64, error) {
	out := make([]*domain.IntegrationCallLog, 0, len(r.logs))
	for _, log := range r.logs {
		if filter.ConnectorKey != nil && log.ConnectorKey != *filter.ConnectorKey {
			continue
		}
		if filter.Status != nil && log.Status != *filter.Status {
			continue
		}
		copyLog := *log
		out = append(out, &copyLog)
	}
	return out, int64(len(out)), nil
}

func (r *integrationCallLogRepoStub) Update(_ context.Context, _ repo.Tx, update repo.IntegrationCallLogUpdate) error {
	log, ok := r.logs[update.CallLogID]
	if !ok {
		return fmt.Errorf("log not found")
	}
	log.Status = update.Status
	log.LatestStatusAt = update.LatestStatusAt
	log.StartedAt = update.StartedAt
	log.FinishedAt = update.FinishedAt
	log.ResponsePayload = append(log.ResponsePayload[:0], update.ResponsePayload...)
	log.ErrorMessage = update.ErrorMessage
	log.Remark = update.Remark
	return nil
}
