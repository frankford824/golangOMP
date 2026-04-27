package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type workbenchPreferenceRepo struct{ db *DB }

func NewWorkbenchPreferenceRepo(db *DB) repo.WorkbenchPreferenceRepo {
	return &workbenchPreferenceRepo{db: db}
}

func (r *workbenchPreferenceRepo) GetByActorScope(ctx context.Context, scope repo.WorkbenchPreferenceScope) (*domain.WorkbenchPreferenceRecord, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT actor_id, actor_roles_key, auth_mode, default_queue_key, pinned_queue_keys,
		       default_filters, default_page_size, default_sort
		FROM workbench_preferences
		WHERE actor_id = ? AND actor_roles_key = ? AND auth_mode = ?`,
		scope.ActorID, scope.ActorRolesKey, scope.AuthMode,
	)

	var record domain.WorkbenchPreferenceRecord
	var pinnedJSON []byte
	var filtersJSON []byte
	var defaultSort string
	if err := row.Scan(
		&record.ActorID,
		&record.ActorRolesKey,
		&record.AuthMode,
		&record.Preferences.DefaultQueueKey,
		&pinnedJSON,
		&filtersJSON,
		&record.Preferences.DefaultPageSize,
		&defaultSort,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get workbench preferences: %w", err)
	}

	record.Preferences.DefaultSort = domain.WorkbenchSortKey(defaultSort)
	if err := unmarshalWorkbenchPinnedQueues(pinnedJSON, &record.Preferences.PinnedQueueKeys); err != nil {
		return nil, err
	}
	if err := unmarshalWorkbenchDefaultFilters(filtersJSON, &record.Preferences.DefaultFilters); err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *workbenchPreferenceRepo) UpsertByActorScope(ctx context.Context, record *domain.WorkbenchPreferenceRecord) error {
	if record == nil {
		return fmt.Errorf("upsert workbench preferences: record is nil")
	}

	pinnedJSON, err := json.Marshal(record.Preferences.PinnedQueueKeys)
	if err != nil {
		return fmt.Errorf("marshal pinned queue keys: %w", err)
	}
	filtersJSON, err := json.Marshal(record.Preferences.DefaultFilters)
	if err != nil {
		return fmt.Errorf("marshal default filters: %w", err)
	}

	_, err = r.db.db.ExecContext(ctx, `
		INSERT INTO workbench_preferences (
			actor_id, actor_roles_key, auth_mode, default_queue_key, pinned_queue_keys,
			default_filters, default_page_size, default_sort
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			default_queue_key = VALUES(default_queue_key),
			pinned_queue_keys = VALUES(pinned_queue_keys),
			default_filters = VALUES(default_filters),
			default_page_size = VALUES(default_page_size),
			default_sort = VALUES(default_sort),
			updated_at = NOW()`,
		record.ActorID,
		record.ActorRolesKey,
		record.AuthMode,
		record.Preferences.DefaultQueueKey,
		pinnedJSON,
		filtersJSON,
		record.Preferences.DefaultPageSize,
		string(record.Preferences.DefaultSort),
	)
	if err != nil {
		return fmt.Errorf("upsert workbench preferences: %w", err)
	}
	return nil
}

func unmarshalWorkbenchPinnedQueues(raw []byte, out *[]string) error {
	if len(raw) == 0 {
		*out = []string{}
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal pinned queue keys: %w", err)
	}
	if *out == nil {
		*out = []string{}
	}
	return nil
}

func unmarshalWorkbenchDefaultFilters(raw []byte, out *domain.TaskQueryTemplate) error {
	if len(raw) == 0 {
		*out = domain.TaskQueryTemplate{}
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal default filters: %w", err)
	}
	return nil
}
