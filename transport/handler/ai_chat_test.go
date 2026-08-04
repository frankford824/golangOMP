package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
	"workflow/service/aichat"
)

type handlerChatTx struct{}

func (handlerChatTx) IsTx() {}

type handlerChatTxRunner struct{}

func (handlerChatTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	return fn(handlerChatTx{})
}

type handlerChatRepo struct {
	conversation domain.AIConversation
	messages     []domain.AIMessage
}

func (r *handlerChatRepo) CreateConversation(context.Context, repo.Tx, domain.AIConversation) error {
	return nil
}
func (r *handlerChatRepo) ListConversations(context.Context, *int64, domain.AIAdminConversationFilter) ([]domain.AIConversation, int64, error) {
	return nil, 0, nil
}
func (r *handlerChatRepo) GetConversation(context.Context, string) (*domain.AIConversation, error) {
	item := r.conversation
	return &item, nil
}
func (r *handlerChatRepo) ListMessages(context.Context, string, int) ([]domain.AIMessage, error) {
	return append([]domain.AIMessage{}, r.messages...), nil
}
func (r *handlerChatRepo) FindUserMessageByClientID(context.Context, string, string) (*domain.AIMessage, error) {
	return nil, sql.ErrNoRows
}
func (r *handlerChatRepo) FindAssistantByReplyTo(context.Context, string) (*domain.AIMessage, error) {
	return nil, sql.ErrNoRows
}
func (r *handlerChatRepo) CreateMessage(_ context.Context, _ repo.Tx, item domain.AIMessage) error {
	r.messages = append(r.messages, item)
	return nil
}
func (r *handlerChatRepo) FinalizeMessage(_ context.Context, _ repo.Tx, item domain.AIMessage) error {
	for index := range r.messages {
		if r.messages[index].ID == item.ID {
			r.messages[index] = item
			return nil
		}
	}
	return errors.New("message not found")
}
func (*handlerChatRepo) ReplaceMessageSources(context.Context, repo.Tx, string, []domain.AIMessageSource) error {
	return nil
}
func (*handlerChatRepo) SoftDeleteConversation(context.Context, repo.Tx, string, int64, time.Time) (bool, error) {
	return true, nil
}
func (*handlerChatRepo) PurgeExpiredConversations(context.Context, repo.Tx, time.Time, int) (int64, error) {
	return 0, nil
}
func (*handlerChatRepo) InsertProviderCall(context.Context, repo.Tx, domain.AIProviderCall) error {
	return nil
}
func (*handlerChatRepo) InsertAccessAudit(context.Context, repo.Tx, domain.AIAccessAudit) error {
	return nil
}

type blockingHandlerChatProvider struct{ entered chan struct{} }

func (*blockingHandlerChatProvider) Ready() bool { return true }
func (*blockingHandlerChatProvider) CompleteText(context.Context, aiagent.ChatRequest) (string, aiagent.ChatStreamResult, error) {
	return `{}`, aiagent.ChatStreamResult{}, nil
}
func (p *blockingHandlerChatProvider) Stream(ctx context.Context, _ aiagent.ChatRequest, _ func(string) error) (aiagent.ChatStreamResult, error) {
	close(p.entered)
	<-ctx.Done()
	return aiagent.ChatStreamResult{Provider: "test", Model: "blocking"}, ctx.Err()
}

type handlerChatRetriever struct{}

func (handlerChatRetriever) HybridReady() bool { return true }
func (handlerChatRetriever) Search(context.Context, domain.RequestActor, string, int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "hybrid"}, nil
}

type handlerChatLimiter struct{}

func (handlerChatLimiter) Acquire(context.Context, int64) (func(context.Context), error) {
	return func(context.Context) {}, nil
}

func TestAIChatHandlerConfigRequiresExplicitReportCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := aichat.NewService(nil, nil, nil, nil, nil, aichat.Config{}, nil)
	handler := NewAIChatHandler(service, time.Second)

	for _, test := range []struct {
		name   string
		actor  domain.RequestActor
		status int
	}{
		{name: "legacy role alone is denied", actor: domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAdmin}}, status: http.StatusForbidden},
		{name: "explicit report capability is accepted", actor: handlerReportActor(7), status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			request := httptest.NewRequest(http.MethodGet, "/v1/ai/chat/config", nil)
			ctx.Request = request.WithContext(domain.WithRequestActor(request.Context(), test.actor))
			handler.Config(ctx)
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAIChatHandlerStreamSendsHeartbeatAndHonorsClientCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &handlerChatRepo{conversation: domain.AIConversation{ID: "conversation-1", OwnerUserID: 7, Status: domain.AIConversationStatusActive}}
	provider := &blockingHandlerChatProvider{entered: make(chan struct{})}
	service := aichat.NewService(repository, handlerChatTxRunner{}, provider, handlerChatRetriever{}, handlerChatLimiter{}, aichat.Config{Enabled: true}, nil)
	handler := NewAIChatHandler(service, 2*time.Millisecond)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/v1/ai/chat/conversations/conversation-1/messages:stream", strings.NewReader(`{"client_message_id":"client-1","content":"分析交付问题"}`)).WithContext(requestContext)
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(domain.WithRequestActor(request.Context(), handlerReportActor(7)))
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "conversation_id", Value: "conversation-1"}}

	done := make(chan struct{})
	go func() {
		handler.StreamMessage(ctx)
		close(done)
	}()
	select {
	case <-provider.entered:
	case <-time.After(time.Second):
		t.Fatal("stream provider was not reached")
	}
	time.Sleep(8 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not stop after request cancellation")
	}
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" || recorder.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: meta") || !strings.Contains(body, ": heartbeat") {
		t.Fatalf("stream body does not contain initial event and heartbeat: %s", body)
	}
	if len(repository.messages) != 2 || repository.messages[1].Status != domain.AIMessageStatusCancelled {
		t.Fatalf("messages=%+v", repository.messages)
	}
}

func handlerReportActor(id int64) domain.RequestActor {
	assignment := domain.AccessAssignment{UserID: id, RoleID: 5, RoleCode: "reporter", ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
	return domain.RequestActor{ID: id, Permissions: []domain.PermissionCode{domain.PermissionReportView}, EffectiveAccess: &domain.EffectiveAccess{
		UserID: id, Permissions: []domain.PermissionCode{domain.PermissionReportView}, Assignments: []domain.AccessAssignment{assignment},
		Sources: []domain.EffectiveAccessNote{{Permission: domain.PermissionReportView, RoleID: 5, RoleCode: "reporter", SourceType: "direct", ScopeMode: domain.AccessScopeGlobal}},
	}}
}
