package task_pool

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo/mysql"
)

type PoolEntry struct {
	TaskID       int64     `json:"task_id"`
	ModuleKey    string    `json:"module_key"`
	PoolTeamCode string    `json:"pool_team_code"`
	Priority     string    `json:"priority"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	TaskType     string    `json:"task_type"`
	TaskNo       string    `json:"task_no"`
	Title        string    `json:"title"`
	ProductCode  string    `json:"product_code"`
}

type PoolQueryService struct {
	db *mysqlrepo.DB
}

func NewPoolQueryService(db *mysqlrepo.DB) *PoolQueryService { return &PoolQueryService{db: db} }

func (s *PoolQueryService) List(ctx context.Context, actor domain.RequestActor, moduleKey, poolTeamCode string, limit, offset int, sort string) ([]PoolEntry, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	teams := actorTeams(actor)
	if poolTeamCode != "" {
		if !contains(teams, poolTeamCode) {
			return []PoolEntry{}, 0, nil
		}
		teams = []string{poolTeamCode}
	}
	if len(teams) == 0 {
		return []PoolEntry{}, 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(teams)), ",")
	filterArgs := make([]interface{}, 0, len(teams)+2)
	for _, team := range teams {
		filterArgs = append(filterArgs, team)
	}
	where := []string{
		"tm.state = 'pending_claim'",
		"tm.pool_team_code IN (" + placeholders + ")",
		"COALESCE(JSON_EXTRACT(tm.data, '$.backfill_placeholder'), CAST('false' AS JSON)) != CAST('true' AS JSON)",
	}
	if moduleKey != "" {
		where = append(where, "tm.module_key = ?")
		filterArgs = append(filterArgs, moduleKey)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	countQuery := `
		SELECT COUNT(*)
		  FROM task_modules tm
		  JOIN tasks t ON t.id = tm.task_id
		  LEFT JOIN task_details td ON td.task_id = t.id
		 WHERE ` + whereSQL
	if err := s.dbRaw().QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count task pool: %w", err)
	}
	args := append([]interface{}{}, filterArgs...)
	args = append(args, limit, offset)
	query := `
		SELECT tm.task_id, tm.module_key, COALESCE(tm.pool_team_code, ''),
		       t.priority, t.created_at, t.updated_at, t.task_type, t.task_no,
		       COALESCE(NULLIF(t.product_name_snapshot, ''), td.design_requirement, td.demand_text, ''),
		       COALESCE(t.sku_code, '')
		  FROM task_modules tm
		  JOIN tasks t ON t.id = tm.task_id
		  LEFT JOIN task_details td ON td.task_id = t.id
		 WHERE ` + whereSQL + `
		 ORDER BY ` + poolOrderBy(sort) + `
		 LIMIT ? OFFSET ?`
	rows, err := s.dbRaw().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list task pool: %w", err)
	}
	defer rows.Close()
	out := make([]PoolEntry, 0)
	for rows.Next() {
		var item PoolEntry
		if err := rows.Scan(&item.TaskID, &item.ModuleKey, &item.PoolTeamCode, &item.Priority, &item.CreatedAt, &item.UpdatedAt, &item.TaskType, &item.TaskNo, &item.Title, &item.ProductCode); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s *PoolQueryService) dbRaw() *sql.DB {
	return mysqlrepo.RawDB(s.db)
}

func actorTeams(actor domain.RequestActor) []string {
	seen := map[string]struct{}{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			seen[v] = struct{}{}
		}
	}
	add(actor.Team)
	for _, v := range actor.ManagedTeams {
		add(v)
	}
	for _, v := range actor.FrontendAccess.TeamCodes {
		add(v)
	}
	for _, v := range actor.FrontendAccess.ManagedTeams {
		add(v)
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	return out
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func poolOrderBy(sort string) string {
	switch strings.TrimSpace(sort) {
	case "-updated_at":
		return "t.updated_at DESC, t.id DESC"
	case "updated_at":
		return "t.updated_at ASC, t.id ASC"
	case "-created_at":
		return "t.created_at DESC, t.id DESC"
	case "created_at":
		return "t.created_at ASC, t.id ASC"
	default:
		return "FIELD(t.priority, 'critical', 'high', 'normal', 'low') ASC, t.created_at ASC, t.id ASC"
	}
}
