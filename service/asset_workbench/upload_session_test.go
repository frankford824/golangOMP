package assetworkbench

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

type serializedUploadTxRunner struct {
	mu sync.Mutex
}

func (r *serializedUploadTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fn(assetWorkbenchTestTx{})
}

type lockedUploadSessionRepo struct {
	repo.AssetWorkbenchRepo

	mu                sync.Mutex
	session           *domain.AssetWorkbenchUploadSession
	expiredCandidates []*domain.AssetWorkbenchUploadSession
	forUpdateCalls    int
	updateCalls       int
	events            []*domain.AssetWorkbenchEvent
}

func (r *lockedUploadSessionRepo) GetUploadSession(_ context.Context, sessionID string) (*domain.AssetWorkbenchUploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.copySession(sessionID)
}

func (r *lockedUploadSessionRepo) GetUploadSessionForUpdate(_ context.Context, _ repo.Tx, sessionID string) (*domain.AssetWorkbenchUploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.forUpdateCalls++
	return r.copySession(sessionID)
}

func (r *lockedUploadSessionRepo) copySession(sessionID string) (*domain.AssetWorkbenchUploadSession, error) {
	if r.session == nil || r.session.SessionID != sessionID {
		return nil, sql.ErrNoRows
	}
	copySession := *r.session
	return &copySession, nil
}

func (r *lockedUploadSessionRepo) UpdateUploadSessionStatus(_ context.Context, _ repo.Tx, sessionID, status string, uploadedAt *time.Time, cancelledAt *time.Time, submittedItemID *int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil || r.session.SessionID != sessionID {
		return sql.ErrNoRows
	}
	r.updateCalls++
	r.session.Status = status
	if uploadedAt != nil {
		value := *uploadedAt
		r.session.UploadedAt = &value
	}
	if cancelledAt != nil {
		value := *cancelledAt
		r.session.CancelledAt = &value
	}
	if submittedItemID != nil {
		value := *submittedItemID
		r.session.SubmittedItemID = &value
	}
	return nil
}

func (r *lockedUploadSessionRepo) ListExpiredUploadSessions(_ context.Context, _ time.Time, _ int) ([]*domain.AssetWorkbenchUploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*domain.AssetWorkbenchUploadSession, 0, len(r.expiredCandidates))
	for _, candidate := range r.expiredCandidates {
		if candidate == nil {
			continue
		}
		copyCandidate := *candidate
		items = append(items, &copyCandidate)
	}
	return items, nil
}

func (r *lockedUploadSessionRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func (r *lockedUploadSessionRepo) snapshot() (*domain.AssetWorkbenchUploadSession, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var session *domain.AssetWorkbenchUploadSession
	if r.session != nil {
		copySession := *r.session
		session = &copySession
	}
	return session, r.forUpdateCalls, r.updateCalls
}

type uploadObjectStoreStub struct {
	*baseservice.OSSDirectService

	mu              sync.Mutex
	contentLength   int64
	exists          bool
	statErr         error
	completeErr     error
	statStarted     chan struct{}
	statRelease     chan struct{}
	statStartedOnce sync.Once
	statCalls       int
	completeCalls   int
	deleteCalls     int
	abortCalls      int
}

func newUploadObjectStoreStub(contentLength int64) *uploadObjectStoreStub {
	return &uploadObjectStoreStub{
		OSSDirectService: testWorkbenchOSSDirect(),
		contentLength:    contentLength,
		exists:           true,
	}
}

func (s *uploadObjectStoreStub) Enabled() bool { return true }

func (s *uploadObjectStoreStub) CompleteMultipartUpload(_ context.Context, _, _ string, _ []baseservice.OSSCompletePart) error {
	s.mu.Lock()
	s.completeCalls++
	err := s.completeErr
	s.mu.Unlock()
	return err
}

func (s *uploadObjectStoreStub) StatObject(_ context.Context, _ string) (*baseservice.OSSObjectInfo, bool, error) {
	s.mu.Lock()
	s.statCalls++
	started := s.statStarted
	release := s.statRelease
	contentLength := s.contentLength
	exists := s.exists
	err := s.statErr
	s.mu.Unlock()
	if started != nil {
		s.statStartedOnce.Do(func() { close(started) })
	}
	if release != nil {
		<-release
	}
	if err != nil || !exists {
		return nil, exists, err
	}
	return &baseservice.OSSObjectInfo{ContentLength: contentLength}, true, nil
}

func (s *uploadObjectStoreStub) DeleteObject(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteCalls++
	return nil
}

func (s *uploadObjectStoreStub) AbortMultipartUpload(_ context.Context, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abortCalls++
	return nil
}

func (s *uploadObjectStoreStub) callCounts() (stat, complete, delete, abort int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statCalls, s.completeCalls, s.deleteCalls, s.abortCalls
}

func withUploadObjectStoreForTest(store objectStore) Option {
	return func(s *Service) { s.oss = store }
}

func newLockedUploadSessionService(session *domain.AssetWorkbenchUploadSession, store objectStore) (*Service, *lockedUploadSessionRepo) {
	workbenchRepo := &lockedUploadSessionRepo{session: session}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, &serializedUploadTxRunner{}),
		withUploadObjectStoreForTest(store),
	)
	return svc, workbenchRepo
}

func TestCreateUploadSessionRejectsNonPositiveFileSize(t *testing.T) {
	workbenchRepo := &uploadDirectorySessionRepo{}
	store := newUploadObjectStoreStub(1)
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		withUploadObjectStoreForTest(store),
	)
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	for _, fileSize := range []int64{0, -1} {
		_, appErr := svc.CreateUploadSession(context.Background(), actor, CreateUploadSessionParams{
			OriginalFilename: "empty.png",
			FileSize:         fileSize,
			MimeType:         "image/png",
		})
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest || appErr.Message != "file_size must be positive." {
			t.Fatalf("CreateUploadSession(file_size=%d) error = %+v", fileSize, appErr)
		}
	}
}

func TestCompleteUploadSessionRejectsObjectSizeMismatchForSingleAndMultipart(t *testing.T) {
	for _, tc := range []struct {
		name     string
		uploadID string
		parts    []baseservice.OSSCompletePart
	}{
		{name: "single"},
		{name: "multipart", uploadID: "upload-1", parts: []baseservice.OSSCompletePart{{PartNumber: 1, ETag: "etag-1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newUploadObjectStoreStub(1025)
			svc, workbenchRepo := newLockedUploadSessionService(&domain.AssetWorkbenchUploadSession{
				ID:          11,
				SessionID:   "session-1",
				OwnerUserID: 77,
				Status:      domain.AssetWorkbenchUploadStatusCreated,
				ObjectKey:   "asset-workbench/uploads/session-1/file.png",
				FileSize:    1024,
				UploadID:    tc.uploadID,
			}, store)

			_, appErr := svc.CompleteUploadSession(context.Background(), domain.RequestActor{ID: 77}, "session-1", CompleteUploadSessionParams{Parts: tc.parts})
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest || appErr.Message != "Completed OSS upload size does not match upload session." {
				t.Fatalf("CompleteUploadSession() error = %+v", appErr)
			}
			session, lockCalls, updateCalls := workbenchRepo.snapshot()
			if session == nil || session.Status != domain.AssetWorkbenchUploadStatusCreated || lockCalls != 1 || updateCalls != 0 {
				t.Fatalf("session=%+v lockCalls=%d updateCalls=%d", session, lockCalls, updateCalls)
			}
			statCalls, completeCalls, _, _ := store.callCounts()
			if statCalls != 1 {
				t.Fatalf("statCalls = %d, want 1", statCalls)
			}
			if tc.uploadID == "" && completeCalls != 0 {
				t.Fatalf("single completeCalls = %d, want 0", completeCalls)
			}
			if tc.uploadID != "" && completeCalls != 1 {
				t.Fatalf("multipart completeCalls = %d, want 1", completeCalls)
			}
		})
	}
}

func TestCompleteAndCancelUploadSessionSerializeOnLockedRow(t *testing.T) {
	store := newUploadObjectStoreStub(1024)
	store.statStarted = make(chan struct{})
	store.statRelease = make(chan struct{})
	svc, workbenchRepo := newLockedUploadSessionService(&domain.AssetWorkbenchUploadSession{
		ID:          12,
		SessionID:   "session-race",
		OwnerUserID: 77,
		Status:      domain.AssetWorkbenchUploadStatusCreated,
		ObjectKey:   "asset-workbench/uploads/session-race/file.png",
		FileSize:    1024,
	}, store)
	actor := domain.RequestActor{ID: 77}

	completeDone := make(chan *domain.AppError, 1)
	go func() {
		_, appErr := svc.CompleteUploadSession(context.Background(), actor, "session-race", CompleteUploadSessionParams{})
		completeDone <- appErr
	}()

	select {
	case <-store.statStarted:
	case <-time.After(time.Second):
		t.Fatal("complete did not reach StatObject")
	}

	cancelDone := make(chan *domain.AppError, 1)
	go func() {
		_, appErr := svc.CancelUploadSession(context.Background(), actor, "session-race")
		cancelDone <- appErr
	}()

	select {
	case appErr := <-cancelDone:
		t.Fatalf("cancel completed before locked complete transaction: %+v", appErr)
	case <-time.After(50 * time.Millisecond):
	}
	_, _, deleteCalls, _ := store.callCounts()
	if deleteCalls != 0 {
		t.Fatalf("deleteCalls while complete holds lock = %d, want 0", deleteCalls)
	}

	close(store.statRelease)
	select {
	case appErr := <-completeDone:
		if appErr != nil {
			t.Fatalf("CompleteUploadSession() error = %+v", appErr)
		}
	case <-time.After(time.Second):
		t.Fatal("complete did not finish")
	}
	select {
	case appErr := <-cancelDone:
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
			t.Fatalf("CancelUploadSession() error = %+v, want invalid state", appErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not finish after complete released lock")
	}

	session, lockCalls, updateCalls := workbenchRepo.snapshot()
	if session == nil || session.Status != domain.AssetWorkbenchUploadStatusUploaded || lockCalls != 2 || updateCalls != 1 {
		t.Fatalf("session=%+v lockCalls=%d updateCalls=%d", session, lockCalls, updateCalls)
	}
	_, _, deleteCalls, _ = store.callCounts()
	if deleteCalls != 0 {
		t.Fatalf("deleteCalls = %d, want 0", deleteCalls)
	}
}

func TestExpireUploadSessionsRechecksStatusUnderLock(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	store := newUploadObjectStoreStub(1024)
	svc, workbenchRepo := newLockedUploadSessionService(&domain.AssetWorkbenchUploadSession{
		ID:          13,
		SessionID:   "session-expire-race",
		OwnerUserID: 77,
		Status:      domain.AssetWorkbenchUploadStatusUploaded,
		ObjectKey:   "asset-workbench/uploads/session-expire-race/file.png",
		FileSize:    1024,
		ExpiresAt:   now.Add(-time.Minute),
	}, store)
	svc.nowFn = func() time.Time { return now }
	workbenchRepo.expiredCandidates = []*domain.AssetWorkbenchUploadSession{{
		ID:          13,
		SessionID:   "session-expire-race",
		OwnerUserID: 77,
		Status:      domain.AssetWorkbenchUploadStatusCreated,
		ObjectKey:   "asset-workbench/uploads/session-expire-race/file.png",
		FileSize:    1024,
		ExpiresAt:   now.Add(-time.Minute),
	}}

	expired, appErr := svc.ExpireUploadSessions(context.Background(), 10)
	if appErr != nil || expired != 0 {
		t.Fatalf("ExpireUploadSessions() = (%d, %+v), want (0, nil)", expired, appErr)
	}
	session, lockCalls, updateCalls := workbenchRepo.snapshot()
	if session == nil || session.Status != domain.AssetWorkbenchUploadStatusUploaded || lockCalls != 1 || updateCalls != 0 {
		t.Fatalf("session=%+v lockCalls=%d updateCalls=%d", session, lockCalls, updateCalls)
	}
	_, _, deleteCalls, abortCalls := store.callCounts()
	if deleteCalls != 0 || abortCalls != 0 {
		t.Fatalf("cleanup calls delete=%d abort=%d, want zero", deleteCalls, abortCalls)
	}
}
