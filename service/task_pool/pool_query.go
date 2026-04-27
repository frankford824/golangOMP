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
	TaskType     string    `json:"task_type"`
	TaskNo       string    `json:"task_no"`
	Title        string    `json:"title"`
	ProductCode  string    `json:"product_code"`
}

type PoolQueryService struct {
	db *mysqlrepo.DB
}

func NewPoolQueryService(db *mysqlrepo.DB) *PoolQueryService { return &PoolQueryService{db: db} }

func (s *PoolQueryService) List(ctx context.Context, actor domain.RequestActor, moduleKey, poolTeamCode string, limit, offset int) ([]PoolEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	teams := actorTeams(actor)
	if poolTeamCode != "" {
		if !contains(teams, poolTeamCode) {
			return []PoolEntry{}, nil
		}
		teams = []string{poolTeamCode}
	}
	if len(teams) == 0 {
		return []PoolEntry{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(teams)), ",")
	args := make([]interface{}, 0, len(teams)+4)
	for _, team := range teams {
		args = append(args, team)
	}
	where := []string{
		"tm.state = 'pending_claim'",
		"tm.pool_team_code IN (" + placeholders + ")",
		"COALESCE(JSON_EXTRACT(tm.data, '$.backfill_placeholder'), CAST('false' AS JSON)) != CAST('true' AS JSON)",
	}
	if moduleKey != "" {
		where = append(where, "tm.module_key = ?")
		args = append(args, moduleKey)
	}
	args = append(args, limit, offset)
	query := `
		SELECT tm.task_id, tm.module_key, COALESCE(tm.pool_team_code, ''),
		       t.priority, t.created_at, t.task_type, t.task_no,
		       COALESCE(NULLIF(t.product_name_snapshot, ''), td.design_requirement, td.demand_text, ''),
		       COALESCE(t.sku_code, '')
		  FROM task_modules tm
		  JOIN tasks t ON t.id = tm.task_id
		  LEFT JOIN task_details td ON td.task_id = t.id
		 WHERE ` + strings.Join(where, " AND ") + `
		 ORDER BY FIELD(t.priority, 'critical', 'high', 'normal', 'low') ASC, t.created_at ASC
		 LIMIT ? OFFSET ?`
	rows, err := s.dbRaw().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task pool: %w", err)
	}
	defer rows.Close()
	var out []PoolEntry
	for rows.Next() {
		var item PoolEntry
		if err := rows.Scan(&item.TaskID, &item.ModuleKey, &item.PoolTeamCode, &item.Priority, &item.CreatedAt, &item.TaskType, &item.TaskNo, &item.Title, &item.ProductCode); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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
