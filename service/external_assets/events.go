package externalassets

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"workflow/domain"
)

const maxExternalAssetEventBatch = 500

// ApplyFilesystemEvents validates the whole batch before applying it. A retry
// may repeat an already-applied prefix of a failed batch: origin_path uniqueness,
// source fingerprints, and idempotent missing transitions make that safe.
func (s *Service) ApplyFilesystemEvents(ctx context.Context, batch domain.ExternalAssetFilesystemEventBatch) (*domain.ExternalAssetFilesystemEventResult, *domain.AppError) {
	result := &domain.ExternalAssetFilesystemEventResult{Received: len(batch.Events)}
	if s == nil || !s.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "external assets are disabled", nil)
	}
	if strings.TrimSpace(batch.AgentID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "agent_id is required", nil)
	}
	if len(batch.Events) == 0 || len(batch.Events) > maxExternalAssetEventBatch {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events must contain 1-%d items", maxExternalAssetEventBatch), nil)
	}

	type preparedEvent struct {
		event domain.ExternalAssetFilesystemEvent
		mount MountConfig
	}
	prepared := make([]preparedEvent, 0, len(batch.Events))
	seen := make(map[string]struct{}, len(batch.Events))
	for idx, raw := range batch.Events {
		eventID := strings.TrimSpace(raw.EventID)
		if eventID == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events[%d].event_id is required", idx), nil)
		}
		if _, ok := seen[eventID]; ok {
			result.Duplicates++
			continue
		}
		seen[eventID] = struct{}{}

		raw.EventID = eventID
		raw.MountPath = cleanAListPath(raw.MountPath)
		raw.OriginPath = cleanAListPath(raw.OriginPath)
		if raw.ObservedAt.IsZero() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events[%d].observed_at is required", idx), nil)
		}
		raw.ObservedAt = raw.ObservedAt.UTC().Truncate(time.Second)
		if raw.Type != domain.ExternalAssetFilesystemEventUpsert && raw.Type != domain.ExternalAssetFilesystemEventDelete {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events[%d].type must be upsert or delete", idx), nil)
		}
		mount, ok := s.eventMount(raw.MountPath, raw.OriginPath)
		if !ok {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events[%d] path is outside configured NAS event roots", idx), nil)
		}
		if raw.Type == domain.ExternalAssetFilesystemEventUpsert {
			if raw.FileSize < 0 || raw.ModifiedAt == nil || raw.ModifiedAt.IsZero() {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("events[%d] upsert requires non-negative file_size and modified_at", idx), nil)
			}
			modified := raw.ModifiedAt.UTC().Truncate(time.Second)
			raw.ModifiedAt = &modified
		}
		prepared = append(prepared, preparedEvent{event: raw, mount: mount})
	}

	for _, item := range prepared {
		event := item.event
		switch event.Type {
		case domain.ExternalAssetFilesystemEventDelete:
			if err := s.repo.MarkOriginPathMissing(ctx, "alist", item.mount.Path, event.OriginPath); err != nil {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "apply external asset delete event: "+err.Error(), nil)
			}
			result.Deleted++
		case domain.ExternalAssetFilesystemEventUpsert:
			name := path.Base(event.OriginPath)
			ext := strings.ToLower(filepath.Ext(name))
			driver := item.mount.Driver
			if driver == "" {
				driver = driverForMountKind(item.mount.Kind, item.mount.Path)
			}
			_, err := s.repo.Upsert(ctx, domain.ExternalAssetUpsert{
				Provider:         "alist",
				Kind:             domain.ExternalAssetKindNASLocal,
				Driver:           driver,
				MountPath:        item.mount.Path,
				OriginPath:       event.OriginPath,
				ParentPath:       path.Dir(event.OriginPath),
				FileName:         name,
				FileExt:          ext,
				MimeType:         normalizeMimeType(name, ""),
				FileSize:         event.FileSize,
				SourceModifiedAt: event.ModifiedAt,
				SearchableText:   strings.Join([]string{event.OriginPath, path.Dir(event.OriginPath), name, driver, string(domain.ExternalAssetKindNASLocal)}, " "),
				ScannedAt:        event.ObservedAt,
			})
			if err != nil {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "apply external asset upsert event: "+err.Error(), nil)
			}
			result.Upserted++
		}
		result.Applied++
	}

	if result.Upserted > 0 {
		if _, err := s.EnsureOSSRequiredPrefixesPending(ctx); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "queue external asset event uploads: "+err.Error(), nil)
		}
		s.wakeOSSPrepare()
	}
	return result, nil
}

func (s *Service) eventMount(mountPath, originPath string) (MountConfig, bool) {
	mountPath = cleanAListPath(mountPath)
	originPath = cleanAListPath(originPath)
	for _, root := range s.cfg.EventRoots {
		root = cleanAListPath(root)
		if originPath != root && !strings.HasPrefix(originPath, root+"/") {
			continue
		}
		for _, mount := range s.cfg.Mounts {
			if mount.Kind == domain.ExternalAssetKindNASLocal && mount.Path == mountPath &&
				(root == mount.Path || strings.HasPrefix(root, mount.Path+"/")) {
				return mount, true
			}
		}
	}
	return MountConfig{}, false
}
