package aichat

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
)

type chatTestTx struct{}

func (chatTestTx) IsTx() {}

type chatTestTxRunner struct{}

func (chatTestTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	return fn(chatTestTx{})
}

type chatRepoStub struct {
	mu            sync.Mutex
	conversations map[string]domain.AIConversation
	messages      map[string][]domain.AIMessage
	sources       map[string][]domain.AIMessageSource
	providerCalls []domain.AIProviderCall
	audits        []domain.AIAccessAudit
	purgeNow      time.Time
	purgeCount    int64
}

func newChatRepoStub() *chatRepoStub {
	return &chatRepoStub{conversations: map[string]domain.AIConversation{}, messages: map[string][]domain.AIMessage{}, sources: map[string][]domain.AIMessageSource{}}
}
func (r *chatRepoStub) CreateConversation(_ context.Context, _ repo.Tx, item domain.AIConversation) error {
	r.conversations[item.ID] = item
	return nil
}
func (r *chatRepoStub) ListConversations(_ context.Context, owner *int64, _ domain.AIAdminConversationFilter) ([]domain.AIConversation, int64, error) {
	items := []domain.AIConversation{}
	for _, item := range r.conversations {
		if item.Status != domain.AIConversationStatusDeleted && (owner == nil || item.OwnerUserID == *owner) {
			items = append(items, item)
		}
	}
	return items, int64(len(items)), nil
}
func (r *chatRepoStub) GetConversation(_ context.Context, id string) (*domain.AIConversation, error) {
	item, ok := r.conversations[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return &item, nil
}
func (r *chatRepoStub) ListMessages(_ context.Context, conversationID string, _ int) ([]domain.AIMessage, error) {
	return append([]domain.AIMessage{}, r.messages[conversationID]...), nil
}
func (r *chatRepoStub) FindUserMessageByClientID(_ context.Context, conversationID, clientID string) (*domain.AIMessage, error) {
	for _, item := range r.messages[conversationID] {
		if item.ClientMessageID == clientID {
			copy := item
			return &copy, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (r *chatRepoStub) FindAssistantByReplyTo(_ context.Context, userID string) (*domain.AIMessage, error) {
	for _, items := range r.messages {
		for _, item := range items {
			if item.ReplyToMessageID == userID {
				copy := item
				return &copy, nil
			}
		}
	}
	return nil, sql.ErrNoRows
}
func (r *chatRepoStub) CreateMessage(_ context.Context, _ repo.Tx, item domain.AIMessage) error {
	r.messages[item.ConversationID] = append(r.messages[item.ConversationID], item)
	return nil
}
func (r *chatRepoStub) FinalizeMessage(_ context.Context, _ repo.Tx, item domain.AIMessage) error {
	items := r.messages[item.ConversationID]
	for index := range items {
		if items[index].ID == item.ID {
			items[index] = item
			r.messages[item.ConversationID] = items
			return nil
		}
	}
	return errors.New("message missing")
}
func (r *chatRepoStub) ReplaceMessageSources(_ context.Context, _ repo.Tx, messageID string, sources []domain.AIMessageSource) error {
	r.sources[messageID] = append([]domain.AIMessageSource{}, sources...)
	return nil
}
func (r *chatRepoStub) SoftDeleteConversation(_ context.Context, _ repo.Tx, id string, ownerID int64, now time.Time) (bool, error) {
	item, ok := r.conversations[id]
	if !ok || item.OwnerUserID != ownerID || item.Status != domain.AIConversationStatusActive {
		return false, nil
	}
	item.Status, item.DeletedAt = domain.AIConversationStatusDeleted, &now
	r.conversations[id] = item
	return true, nil
}
func (r *chatRepoStub) PurgeExpiredConversations(_ context.Context, _ repo.Tx, now time.Time, _ int) (int64, error) {
	r.purgeNow = now
	return r.purgeCount, nil
}
func (r *chatRepoStub) InsertProviderCall(_ context.Context, _ repo.Tx, call domain.AIProviderCall) error {
	r.providerCalls = append(r.providerCalls, call)
	return nil
}
func (r *chatRepoStub) InsertAccessAudit(_ context.Context, _ repo.Tx, audit domain.AIAccessAudit) error {
	r.audits = append(r.audits, audit)
	return nil
}

type chatProviderStub struct {
	streamText string
	streamErr  error
	calls      int
}

func (*chatProviderStub) Ready() bool { return true }
func (*chatProviderStub) CompleteText(context.Context, aiagent.ChatRequest) (string, aiagent.ChatStreamResult, error) {
	return `{"tools":[{"name":"global_search","query":"任务"}]}`, aiagent.ChatStreamResult{}, nil
}
func (s *chatProviderStub) Stream(_ context.Context, _ aiagent.ChatRequest, onDelta func(string) error) (aiagent.ChatStreamResult, error) {
	s.calls++
	if s.streamText != "" {
		if err := onDelta(s.streamText); err != nil {
			return aiagent.ChatStreamResult{}, err
		}
	}
	return aiagent.ChatStreamResult{Provider: "test", Model: "model", FinishReason: "end_turn"}, s.streamErr
}

type chatRetrieverStub struct{}

func (chatRetrieverStub) HybridReady() bool { return true }
func (chatRetrieverStub) Search(context.Context, domain.RequestActor, string, int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	return []domain.AIRetrievalHit{{DocumentID: "task:1", EntityType: "task", EntityID: "1", Title: "任务 T-1", InternalRoute: "/tasks/1", Excerpt: "任务已经结单", SourceVersion: "2"}}, domain.AIRetrievalMeta{Mode: "hybrid", Candidates: 1}, nil
}

type allowLimiter struct{ acquired int }

func (l *allowLimiter) Acquire(context.Context, int64) (func(context.Context), error) {
	l.acquired++
	return func(context.Context) {}, nil
}

func TestAIChatOwnerIsolationAndProtectedAdminAudit(t *testing.T) {
	repository := newChatRepoStub()
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	repository.conversations["c1"] = domain.AIConversation{ID: "c1", OwnerUserID: 7, Status: domain.AIConversationStatusActive, ExpiresAt: now.Add(90 * 24 * time.Hour)}
	svc := NewService(repository, chatTestTxRunner{}, &chatProviderStub{}, chatRetrieverStub{}, &allowLimiter{}, Config{Enabled: true}, nil)
	if item, appErr := svc.GetConversation(context.Background(), reportActor(8, false), "c1"); appErr == nil || item != nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("cross-owner read item=%+v err=%+v", item, appErr)
	}
	admin := reportActor(1, true)
	owner := int64(7)
	list, listErr := svc.AdminListConversations(context.Background(), admin, domain.AIAdminConversationFilter{OwnerUserID: &owner})
	if listErr != nil || list == nil || len(repository.audits) != 1 || repository.audits[0].Action != "admin_list_conversations" || repository.audits[0].TargetUserID == nil || *repository.audits[0].TargetUserID != 7 {
		t.Fatalf("admin list=%+v err=%+v audits=%+v", list, listErr, repository.audits)
	}
	item, appErr := svc.AdminGetConversation(context.Background(), admin, "c1")
	if appErr != nil || item == nil || len(repository.audits) != 2 || repository.audits[1].TargetUserID == nil || *repository.audits[1].TargetUserID != 7 {
		t.Fatalf("admin item=%+v err=%+v audits=%+v", item, appErr, repository.audits)
	}
}

func TestAIChatMessageIdempotencyReplaysCompletedAnswer(t *testing.T) {
	repository := newChatRepoStub()
	repository.conversations["c1"] = domain.AIConversation{ID: "c1", OwnerUserID: 7, Status: domain.AIConversationStatusActive}
	completedAt := time.Now()
	repository.messages["c1"] = []domain.AIMessage{
		{ID: "u1", ConversationID: "c1", ClientMessageID: "client-1", Role: domain.AIMessageRoleUser, Status: domain.AIMessageStatusCompleted},
		{ID: "a1", ConversationID: "c1", ReplyToMessageID: "u1", Role: domain.AIMessageRoleAssistant, Content: "已有答案", Status: domain.AIMessageStatusCompleted, CompletedAt: &completedAt},
	}
	provider := &chatProviderStub{}
	limiter := &allowLimiter{}
	svc := NewService(repository, chatTestTxRunner{}, provider, chatRetrieverStub{}, limiter, Config{Enabled: true}, nil)
	events := []domain.AISSEEvent{}
	appErr := svc.StreamMessage(context.Background(), reportActor(7, false), "c1", domain.AIStreamMessageRequest{ClientMessageID: "client-1", Content: "重复问题"}, func(event domain.AISSEEvent) error {
		events = append(events, event)
		return nil
	})
	if appErr != nil || provider.calls != 0 || len(repository.messages["c1"]) != 2 || len(events) != 3 || events[2].Type != "done" {
		t.Fatalf("err=%+v provider=%d messages=%d events=%+v", appErr, provider.calls, len(repository.messages["c1"]), events)
	}
}

func TestAIChatCancellationPersistsPartialAnswer(t *testing.T) {
	repository := newChatRepoStub()
	repository.conversations["c1"] = domain.AIConversation{ID: "c1", OwnerUserID: 7, Status: domain.AIConversationStatusActive}
	provider := &chatProviderStub{streamText: "部分结果", streamErr: context.Canceled}
	svc := NewService(repository, chatTestTxRunner{}, provider, chatRetrieverStub{}, &allowLimiter{}, Config{Enabled: true}, nil)
	appErr := svc.StreamMessage(context.Background(), reportActor(7, false), "c1", domain.AIStreamMessageRequest{ClientMessageID: "client-2", Content: "分析任务"}, func(domain.AISSEEvent) error { return nil })
	if appErr != nil {
		t.Fatal(appErr)
	}
	items := repository.messages["c1"]
	if len(items) != 2 || items[1].Status != domain.AIMessageStatusCancelled || items[1].Content != "部分结果" || items[1].ErrorCode != "client_cancelled" {
		t.Fatalf("messages=%+v", items)
	}
	if len(repository.providerCalls) != 1 || repository.providerCalls[0].Status != "cancelled" {
		t.Fatalf("calls=%+v", repository.providerCalls)
	}
}

func TestAIChatPurgeUsesCurrentClock(t *testing.T) {
	repository := newChatRepoStub()
	repository.purgeCount = 3
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	svc := NewService(repository, chatTestTxRunner{}, &chatProviderStub{}, chatRetrieverStub{}, &allowLimiter{}, Config{Enabled: true}, nil)
	svc.now = func() time.Time { return now }
	count, err := svc.PurgeExpired(context.Background(), 100)
	if err != nil || count != 3 || !repository.purgeNow.Equal(now) {
		t.Fatalf("count=%d err=%v now=%s", count, err, repository.purgeNow)
	}
}

func reportActor(id int64, superAdmin bool) domain.RequestActor {
	assignment := domain.AccessAssignment{UserID: id, RoleID: 1, RoleCode: "reporter", ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
	if superAdmin {
		assignment.RoleCode = "super_admin"
	}
	return domain.RequestActor{ID: id, Permissions: []domain.PermissionCode{domain.PermissionReportView, domain.PermissionTaskView}, EffectiveAccess: &domain.EffectiveAccess{
		UserID: id, Permissions: []domain.PermissionCode{domain.PermissionReportView, domain.PermissionTaskView},
		Assignments: []domain.AccessAssignment{assignment},
		Sources:     []domain.EffectiveAccessNote{{Permission: domain.PermissionReportView, RoleID: 1, RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: domain.AccessScopeGlobal}, {Permission: domain.PermissionTaskView, RoleID: 1, RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: domain.AccessScopeGlobal}},
	}}
}
