package workers

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

type previewProcessorStub struct {
	calls  chan int
	cancel context.CancelFunc
}

func (s *previewProcessorStub) ProcessPendingPreviews(_ context.Context, limit int) (int, *domain.AppError) {
	s.calls <- limit
	s.cancel()
	return 0, nil
}

func TestAssetWorkbenchPreviewWorkerRunsImmediatelyOnStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stub := &previewProcessorStub{
		calls:  make(chan int, 1),
		cancel: cancel,
	}
	worker := NewAssetWorkbenchPreviewWorker(stub, zap.NewNop(), time.Hour, 7)

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	select {
	case limit := <-stub.calls:
		if limit != 7 {
			t.Fatalf("ProcessPendingPreviews limit = %d, want 7", limit)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not process previews immediately")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}
