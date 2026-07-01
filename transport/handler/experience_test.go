package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestExperienceHandlerMicroQuestionEligibilityPassesQueryAndActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &experienceHandlerServiceStub{
		eligibility: &domain.ExperienceMicroQuestionEligibility{
			Eligible:       true,
			AnswerEventKey: "microq:1",
			RemainingDaily: 2,
		},
	}
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 291, Roles: []domain.Role{domain.RoleOps}}))
	router.GET("/v1/experience/micro-question-eligibility", NewExperienceHandler(stub).MicroQuestionEligibility)

	req := httptest.NewRequest(http.MethodGet, "/v1/experience/micro-question-eligibility?suggestion_event_id=display-1&suggestion_stable_key=stable-1&surface=task_detail&target_type=task&target_id=42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if stub.microEligibilityActor.ID != 291 {
		t.Fatalf("actor id = %d, want 291", stub.microEligibilityActor.ID)
	}
	if stub.microEligibilityReq.SuggestionEventID != "display-1" ||
		stub.microEligibilityReq.SuggestionStableKey != "stable-1" ||
		stub.microEligibilityReq.Surface != "task_detail" ||
		stub.microEligibilityReq.TargetType != "task" ||
		stub.microEligibilityReq.TargetID != "42" {
		t.Fatalf("eligibility req = %+v", stub.microEligibilityReq)
	}
	var body struct {
		Data domain.ExperienceMicroQuestionEligibility `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if !body.Data.Eligible || body.Data.AnswerEventKey != "microq:1" || body.Data.RemainingDaily != 2 {
		t.Fatalf("response data = %+v", body.Data)
	}
}

func TestExperienceHandlerMicroQuestionAnswerBindsBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &experienceHandlerServiceStub{
		microAnswer: &domain.ExperienceMicroQuestionAnswer{
			AnswerEventKey: "microq:answer",
			AnswerValue:    domain.ExperienceMicroQuestionAnswerAnswered,
			ReasonCode:     "missing_context",
		},
	}
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 292, Roles: []domain.Role{domain.RoleDesigner}}))
	router.POST("/v1/experience/micro-question-answers", NewExperienceHandler(stub).MicroQuestionAnswer)

	raw := `{"answer_event_key":"microq:answer","suggestion_event_id":"display-1","suggestion_stable_key":"stable-1","surface":"task_detail","target_type":"task","target_id":"42","answer_value":"answered","reason_code":"missing_context"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/experience/micro-question-answers", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if stub.microAnswerActor.ID != 292 {
		t.Fatalf("actor id = %d, want 292", stub.microAnswerActor.ID)
	}
	if stub.microAnswerReq.AnswerEventKey != "microq:answer" ||
		stub.microAnswerReq.AnswerValue != domain.ExperienceMicroQuestionAnswerAnswered ||
		stub.microAnswerReq.ReasonCode != "missing_context" {
		t.Fatalf("answer req = %+v", stub.microAnswerReq)
	}
}

func TestExperienceHandlerAISuggestionFeedbackRejectsPathBodyMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &experienceHandlerServiceStub{}
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 291, Roles: []domain.Role{domain.RoleOps}}))
	router.POST("/v1/ai-suggestions/:suggestion_event_id/feedback", NewExperienceHandler(stub).AISuggestionFeedback)

	raw := `{"suggestion_event_id":"body-event","feedback_value":"accepted"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ai-suggestions/path-event/feedback", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if stub.aiFeedbackReq.SuggestionEventID != "" {
		t.Fatalf("feedback req = %+v, want service not called", stub.aiFeedbackReq)
	}
}

func TestExperienceHandlerReviewItemsUsesFilterAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &experienceHandlerServiceStub{
		reviewItems: []*domain.ExperienceReviewItem{{
			ItemKey:  "review-1",
			ItemType: "attribution_candidate",
			Status:   domain.ExperienceReviewItemStatusOpen,
			Priority: "high",
		}},
		reviewPagination: domain.PaginationMeta{Page: 2, PageSize: 5, Total: 9},
	}
	router := gin.New()
	router.GET("/v1/reports/experience/review-items", NewExperienceHandler(stub).ReviewItems)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/experience/review-items?status=open&item_type=attribution_candidate&page=2&page_size=5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if stub.reviewFilter.Status != "open" || stub.reviewFilter.ItemType != "attribution_candidate" || stub.reviewFilter.Page != 2 || stub.reviewFilter.PageSize != 5 {
		t.Fatalf("review filter = %+v", stub.reviewFilter)
	}
	var body struct {
		Data       []domain.ExperienceReviewItem `json:"data"`
		Pagination domain.PaginationMeta         `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if len(body.Data) != 1 || body.Data[0].ItemKey != "review-1" || body.Pagination.Total != 9 {
		t.Fatalf("response = %+v", body)
	}
}

func TestExperienceHandlerReviewDecisionUsesPathActorAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &experienceHandlerServiceStub{
		reviewDecision: &domain.ExperienceReviewDecision{
			ReviewItemKey: "review-1",
			Decision:      domain.ExperienceReviewDecisionNeedsMoreData,
		},
	}
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 293, Roles: []domain.Role{domain.RoleSuperAdmin}}))
	router.POST("/v1/reports/experience/review-items/:item_key/decision", NewExperienceHandler(stub).ReviewDecision)

	req := httptest.NewRequest(http.MethodPost, "/v1/reports/experience/review-items/review-1/decision", strings.NewReader(`{"decision":"needs_more_data","reason_code":"low_sample"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 body=%s", rec.Code, rec.Body.String())
	}
	if stub.reviewDecisionActor.ID != 293 {
		t.Fatalf("actor id = %d, want 293", stub.reviewDecisionActor.ID)
	}
	if stub.reviewDecisionItemKey != "review-1" || stub.reviewDecisionReq.Decision != domain.ExperienceReviewDecisionNeedsMoreData || stub.reviewDecisionReq.ReasonCode != "low_sample" {
		t.Fatalf("review decision item=%q req=%+v", stub.reviewDecisionItemKey, stub.reviewDecisionReq)
	}
}

type experienceHandlerServiceStub struct {
	eligibility           *domain.ExperienceMicroQuestionEligibility
	microEligibilityActor domain.RequestActor
	microEligibilityReq   service.ExperienceMicroQuestionEligibilityRequest
	microAnswer           *domain.ExperienceMicroQuestionAnswer
	microAnswerActor      domain.RequestActor
	microAnswerReq        service.ExperienceMicroQuestionAnswerRequest
	reviewItems           []*domain.ExperienceReviewItem
	reviewPagination      domain.PaginationMeta
	reviewFilter          service.ExperienceReviewItemFilter
	reviewDecision        *domain.ExperienceReviewDecision
	reviewDecisionActor   domain.RequestActor
	reviewDecisionItemKey string
	reviewDecisionReq     service.ExperienceReviewDecisionRequest
	aiFeedbackActor       domain.RequestActor
	aiFeedbackReq         service.AISuggestionFeedbackRequest
	behaviorActor         domain.RequestActor
	behaviorReq           service.ExperienceBehaviorBatchRequest
	rateLimitActor        domain.RequestActor
	rateLimitBucketName   string
	rateLimitPeriodStart  time.Time
	rateLimitPeriodEnd    time.Time
	rateLimitLimit        int
}

func (s *experienceHandlerServiceStub) RuntimeFlags() domain.ExperienceRuntimeFlags {
	return domain.ExperienceRuntimeFlags{UIEnabled: true}
}

func (s *experienceHandlerServiceStub) ClientConfig() domain.ExperienceClientConfig {
	return domain.ExperienceClientConfig{}
}

func (s *experienceHandlerServiceStub) ListReasonTags(context.Context, string) ([]*domain.ExperienceReasonTag, *domain.AppError) {
	return nil, nil
}

func (s *experienceHandlerServiceStub) ListClientReasonTags(context.Context, string) ([]*domain.ExperienceClientReasonTag, *domain.AppError) {
	return nil, nil
}

func (s *experienceHandlerServiceStub) ListSamples(context.Context, service.ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *experienceHandlerServiceStub) Stats(context.Context) (*domain.ExperienceStats, *domain.AppError) {
	return &domain.ExperienceStats{}, nil
}

func (s *experienceHandlerServiceStub) ListReviewItems(_ context.Context, filter service.ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, domain.PaginationMeta, *domain.AppError) {
	s.reviewFilter = filter
	return s.reviewItems, s.reviewPagination, nil
}

func (s *experienceHandlerServiceStub) EnqueueEvent(context.Context, *domain.ExperienceOutboxEvent) *domain.AppError {
	return nil
}

func (s *experienceHandlerServiceStub) RecordAISuggestionEvent(context.Context, *domain.AISuggestionEvent) *domain.AppError {
	return nil
}

func (s *experienceHandlerServiceStub) RecordBehaviorEvents(_ context.Context, actor domain.RequestActor, req service.ExperienceBehaviorBatchRequest) (service.ExperienceBehaviorBatchResult, *domain.AppError) {
	s.behaviorActor = actor
	s.behaviorReq = req
	return service.ExperienceBehaviorBatchResult{Received: len(req.Events), Inserted: len(req.Events)}, nil
}

func (s *experienceHandlerServiceStub) RecordAISuggestionFeedback(_ context.Context, actor domain.RequestActor, req service.AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError) {
	s.aiFeedbackActor = actor
	s.aiFeedbackReq = req
	return &domain.AISuggestionFeedback{SuggestionEventID: req.SuggestionEventID, FeedbackValue: req.FeedbackValue}, nil
}

func (s *experienceHandlerServiceStub) MicroQuestionEligibility(_ context.Context, actor domain.RequestActor, req service.ExperienceMicroQuestionEligibilityRequest) (*domain.ExperienceMicroQuestionEligibility, *domain.AppError) {
	s.microEligibilityActor = actor
	s.microEligibilityReq = req
	if s.eligibility != nil {
		return s.eligibility, nil
	}
	return &domain.ExperienceMicroQuestionEligibility{}, nil
}

func (s *experienceHandlerServiceStub) RecordMicroQuestionAnswer(_ context.Context, actor domain.RequestActor, req service.ExperienceMicroQuestionAnswerRequest) (*domain.ExperienceMicroQuestionAnswer, *domain.AppError) {
	s.microAnswerActor = actor
	s.microAnswerReq = req
	if s.microAnswer != nil {
		return s.microAnswer, nil
	}
	return &domain.ExperienceMicroQuestionAnswer{AnswerEventKey: req.AnswerEventKey, AnswerValue: req.AnswerValue}, nil
}

func (s *experienceHandlerServiceStub) RecordReviewDecision(_ context.Context, actor domain.RequestActor, itemKey string, req service.ExperienceReviewDecisionRequest) (*domain.ExperienceReviewDecision, *domain.AppError) {
	s.reviewDecisionActor = actor
	s.reviewDecisionItemKey = itemKey
	s.reviewDecisionReq = req
	if s.reviewDecision != nil {
		return s.reviewDecision, nil
	}
	return &domain.ExperienceReviewDecision{ReviewItemKey: itemKey, Decision: req.Decision}, nil
}

func (s *experienceHandlerServiceStub) ProcessOutcomeObservers(context.Context, int) (domain.ExperienceObserverRun, *domain.AppError) {
	return domain.ExperienceObserverRun{}, nil
}

func (s *experienceHandlerServiceStub) ProcessOutbox(context.Context, int) (domain.ExperienceWorkerRun, *domain.AppError) {
	return domain.ExperienceWorkerRun{}, nil
}

func (s *experienceHandlerServiceStub) ProcessAttributions(context.Context, int) (domain.ExperienceAttributionRun, *domain.AppError) {
	return domain.ExperienceAttributionRun{}, nil
}

func (s *experienceHandlerServiceStub) ProcessRetention(context.Context, time.Time, int) (domain.ExperienceRetentionRun, *domain.AppError) {
	return domain.ExperienceRetentionRun{}, nil
}

func (s *experienceHandlerServiceStub) ReserveRateLimit(_ context.Context, actor domain.RequestActor, bucketName string, periodStart time.Time, periodEnd time.Time, limit int) (*domain.ExperienceRateLimitReservation, *domain.AppError) {
	s.rateLimitActor = actor
	s.rateLimitBucketName = bucketName
	s.rateLimitPeriodStart = periodStart
	s.rateLimitPeriodEnd = periodEnd
	s.rateLimitLimit = limit
	return &domain.ExperienceRateLimitReservation{Allowed: true, Limit: limit, Count: 1}, nil
}
