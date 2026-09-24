package assetworkbench

import (
	"context"
	"time"
	"workflow/domain"
)

func workbenchPreviewContext(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) context.Context {
	options := domain.AssetMediaOptionsFromContext(ctx)
	options.WorkbenchPreviewWorker = file.PreviewWorkerID
	options.WorkbenchPreviewObject = file.ObjectKey
	return domain.WithAssetMediaOptions(ctx, options)
}

func (s *Service) processPreviewWithLease(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) error {
	jobCtx, cancel := context.WithTimeout(workbenchPreviewContext(ctx, file), 10*time.Minute)
	defer cancel()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		renewer, ok := s.repo.(interface {
			RenewPreviewLease(context.Context, int64, string, string) error
		})
		if !ok || file.PreviewWorkerID == "" {
			return
		}
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-jobCtx.Done():
				return
			case <-tick.C:
				if err := renewer.RenewPreviewLease(jobCtx, file.ID, file.PreviewWorkerID, file.ObjectKey); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	err := s.processPreviewFile(jobCtx, file)
	close(done)
	<-stopped
	return err
}
