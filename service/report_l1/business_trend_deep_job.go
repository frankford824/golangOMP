package report_l1

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/service/aiagent"
)

const (
	CodeBusinessTrendDeepAnalysisUnavailable = "business_trend_deep_analysis_unavailable"

	BusinessTrendDeepJobQueued    = "queued"
	BusinessTrendDeepJobRunning   = "running"
	BusinessTrendDeepJobSucceeded = "succeeded"
	BusinessTrendDeepJobFailed    = "failed"
)

type BusinessTrendDeepAnalysisJob struct {
	JobID        string                         `json:"job_id"`
	Status       string                         `json:"status"`
	Message      string                         `json:"message"`
	Analysis     *aiagent.BusinessTrendAnalysis `json:"analysis,omitempty"`
	ErrorMessage string                         `json:"error_message,omitempty"`
	CreatedAt    time.Time                      `json:"created_at"`
	UpdatedAt    time.Time                      `json:"updated_at"`
}

type businessTrendDeepJobStore struct {
	mu   sync.Mutex
	jobs map[string]*BusinessTrendDeepAnalysisJob
}

func newBusinessTrendDeepJobStore() *businessTrendDeepJobStore {
	return &businessTrendDeepJobStore{jobs: map[string]*BusinessTrendDeepAnalysisJob{}}
}

func (s *Service) StartBusinessTrendDeepAnalysis(ctx context.Context, actor domain.RequestActor, params BusinessTrendAnalysisParams) (*BusinessTrendDeepAnalysisJob, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/business-trends/deep-analysis-jobs"); err != nil {
		return nil, err
	}
	if appErr := s.validateBusinessTrendAnalysisParams(params); appErr != nil {
		return nil, appErr
	}
	if s.businessTrendRepo == nil {
		return nil, domain.NewAppError(CodeBusinessTrendNotConfigured, "业务热点分析服务尚未配置", nil)
	}
	if s.businessTrendGenerator == nil {
		return nil, domain.NewAppError(CodeBusinessTrendDeepAnalysisUnavailable, "业务热点深度分析暂未启用", nil)
	}
	store := s.ensureBusinessTrendDeepJobStore()
	job := store.create()
	go s.runBusinessTrendDeepAnalysisJob(job.JobID, params)
	return job, nil
}

func (s *Service) GetBusinessTrendDeepAnalysisJob(ctx context.Context, actor domain.RequestActor, jobID string) (*BusinessTrendDeepAnalysisJob, *domain.AppError) {
	if err := s.requireSuperAdmin(ctx, actor, "/v1/reports/business-trends/deep-analysis-jobs/:job_id"); err != nil {
		return nil, err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid business trend analysis job id", nil)
	}
	job, ok := s.ensureBusinessTrendDeepJobStore().get(jobID)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "业务热点深度分析任务不存在或已过期", nil)
	}
	return job, nil
}

func (s *Service) runBusinessTrendDeepAnalysisJob(jobID string, params BusinessTrendAnalysisParams) {
	store := s.ensureBusinessTrendDeepJobStore()
	store.markRunning(jobID, "正在结合近期任务与可用热点做深度判断")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	evidence, err := s.collectBusinessTrendEvidence(ctx, params)
	if err != nil {
		store.markFailed(jobID, "近期任务暂时不可读，请稍后再试")
		return
	}
	analysis, err := s.businessTrendGenerator.GenerateBusinessTrendAnalysis(ctx, evidence)
	if err != nil || analysis == nil {
		store.markFailed(jobID, "深度分析暂时不可用，基础分析仍可使用")
		return
	}
	if strings.TrimSpace(analysis.RawText) != "" {
		store.markFailed(jobID, "深度分析返回内容不稳定，基础分析仍可使用")
		return
	}
	mergeBusinessTrendFallbackContent(analysis, evidence)
	store.markSucceeded(jobID, analysis)
}

func (s *Service) ensureBusinessTrendDeepJobStore() *businessTrendDeepJobStore {
	if s.businessTrendJobs != nil {
		return s.businessTrendJobs
	}
	s.businessTrendJobs = newBusinessTrendDeepJobStore()
	return s.businessTrendJobs
}

func (s *businessTrendDeepJobStore) create() *BusinessTrendDeepAnalysisJob {
	now := time.Now().UTC()
	job := &BusinessTrendDeepAnalysisJob{
		JobID:     uuid.NewString(),
		Status:    BusinessTrendDeepJobQueued,
		Message:   "深度分析已开始",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	s.jobs[job.JobID] = job
	return cloneBusinessTrendDeepJob(job)
}

func (s *businessTrendDeepJobStore) get(jobID string) (*BusinessTrendDeepAnalysisJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return nil, false
	}
	return cloneBusinessTrendDeepJob(job), true
}

func (s *businessTrendDeepJobStore) markRunning(jobID, message string) {
	s.update(jobID, func(job *BusinessTrendDeepAnalysisJob) {
		job.Status = BusinessTrendDeepJobRunning
		job.Message = message
	})
}

func (s *businessTrendDeepJobStore) markSucceeded(jobID string, analysis *aiagent.BusinessTrendAnalysis) {
	s.update(jobID, func(job *BusinessTrendDeepAnalysisJob) {
		job.Status = BusinessTrendDeepJobSucceeded
		job.Message = "深度分析已完成"
		job.Analysis = analysis
		job.ErrorMessage = ""
	})
}

func (s *businessTrendDeepJobStore) markFailed(jobID, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "深度分析暂时不可用，基础分析仍可使用"
	}
	s.update(jobID, func(job *BusinessTrendDeepAnalysisJob) {
		job.Status = BusinessTrendDeepJobFailed
		job.Message = message
		job.ErrorMessage = message
	})
}

func (s *businessTrendDeepJobStore) update(jobID string, mutate func(*BusinessTrendDeepAnalysisJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[jobID]
	if job == nil {
		return
	}
	mutate(job)
	job.UpdatedAt = time.Now().UTC()
}

func (s *businessTrendDeepJobStore) cleanupLocked(now time.Time) {
	for id, job := range s.jobs {
		if now.Sub(job.UpdatedAt) > 6*time.Hour {
			delete(s.jobs, id)
		}
	}
}

func cloneBusinessTrendDeepJob(job *BusinessTrendDeepAnalysisJob) *BusinessTrendDeepAnalysisJob {
	if job == nil {
		return nil
	}
	copy := *job
	return &copy
}
