package main

import (
	"context"
	"database/sql"
)

type taskRow struct {
	ID                          int64
	TaskType                    string
	TaskStatus                  string
	CreatorID                   int64
	DesignerID                  sql.NullInt64
	CurrentHandlerID            sql.NullInt64
	CustomizationRequired       bool
	LastCustomizationOperatorID sql.NullInt64
}

type assetRow struct {
	ID                    int64
	TaskID                int64
	AssetType             string
	TaskType              string
	CustomizationRequired bool
}

func loadTasks(ctx context.Context, db *sql.DB) ([]taskRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, task_type, task_status, creator_id, designer_id, current_handler_id,
		       customization_required, last_customization_operator_id
		  FROM tasks
		 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []taskRow
	for rows.Next() {
		var t taskRow
		var custom int
		if err := rows.Scan(&t.ID, &t.TaskType, &t.TaskStatus, &t.CreatorID, &t.DesignerID, &t.CurrentHandlerID, &custom, &t.LastCustomizationOperatorID); err != nil {
			return nil, err
		}
		t.CustomizationRequired = custom == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadAssets(ctx context.Context, db *sql.DB) ([]assetRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ta.id, ta.task_id, ta.asset_type, t.task_type, t.customization_required
		  FROM task_assets ta
		  JOIN tasks t ON t.id = ta.task_id
		 ORDER BY ta.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []assetRow
	for rows.Next() {
		var a assetRow
		var custom int
		if err := rows.Scan(&a.ID, &a.TaskID, &a.AssetType, &a.TaskType, &custom); err != nil {
			return nil, err
		}
		a.CustomizationRequired = custom == 1
		out = append(out, a)
	}
	return out, rows.Err()
}
