package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

const (
	workflowGroupsSnapshotVersion = 2
	workflowGroupsToolVersion     = "workflow-groups-migrate/v8.2"
	workflowGroupsSchemaVersion   = "124-126"
)

type options struct {
	DSN         string
	DryRun      bool
	Apply       bool
	Rollback    bool
	SnapshotDir string
	BatchSize   int
	MappingFile string
	ReportFile  string
	ConfirmDB   string
}

type mappingFile struct {
	Resources []resourceMapping `json:"resources"`
	Planning  []planningMapping `json:"planning_tasks"`
}

type resourceMapping struct {
	TaskID        int64   `json:"task_id"`
	ScopeKind     string  `json:"scope_kind"`
	ScopeRefID    int64   `json:"scope_ref_id"`
	Mode          string  `json:"mode"`
	SourceAssetID *int64  `json:"source_task_asset_id"`
	FinalAssetIDs []int64 `json:"final_task_asset_ids"`
	ReferenceIDs  []int64 `json:"reference_file_ref_ids"`
	CreatedBy     int64   `json:"created_by"`
	TargetStatus  string  `json:"target_status"`
	Reason        string  `json:"reason"`
}

type planningMapping struct {
	TaskID             int64                 `json:"task_id"`
	CodeRuleRevisionID int64                 `json:"code_rule_revision_id"`
	CreatedBy          int64                 `json:"created_by"`
	Items              []planningItemMapping `json:"items"`
}

type planningItemMapping struct {
	TaskSKUItemID   int64   `json:"task_sku_item_id"`
	DescriptionSpec string  `json:"description_spec"`
	Quantity        int64   `json:"quantity"`
	TargetPrice     *string `json:"target_price"`
	Note            string  `json:"note"`
	ReferenceURL    string  `json:"reference_url"`
	ERPProductIID   string  `json:"erp_product_i_id"`
	ERPProductName  string  `json:"erp_product_name"`
	ImageStorageRef string  `json:"image_storage_ref_id"`
}

type taskSnapshot struct {
	ID               int64    `json:"id"`
	TaskType         string   `json:"task_type"`
	TaskStatus       string   `json:"task_status"`
	WorkflowRevision int64    `json:"workflow_revision"`
	CurrentHandlerID *int64   `json:"current_handler_id"`
	EventIDs         []string `json:"event_ids"`
}

type resourceGroupSnapshot struct {
	ID                  int64   `json:"id"`
	TaskID              int64   `json:"task_id"`
	WorkingRevisionID   *int64  `json:"working_revision_id,omitempty"`
	FinalizedRevisionID *int64  `json:"finalized_revision_id,omitempty"`
	LockVersion         int64   `json:"lock_version"`
	MigrationIncomplete bool    `json:"migration_incomplete"`
	MigrationIssue      string  `json:"migration_issue"`
	RevisionIDs         []int64 `json:"revision_ids"`
}

type assetBindingSnapshot struct {
	ID                         int64      `json:"id"`
	TaskID                     int64      `json:"task_id"`
	BindingState               string     `json:"binding_state"`
	BoundGroupID               *int64     `json:"bound_group_id,omitempty"`
	BoundRole                  *string    `json:"bound_role,omitempty"`
	StagedTaskSKUItemID        *int64     `json:"staged_task_sku_item_id,omitempty"`
	StagedRetouchRequirementID *int64     `json:"staged_retouch_requirement_id,omitempty"`
	StagedRole                 *string    `json:"staged_role,omitempty"`
	StagedBy                   *int64     `json:"staged_by,omitempty"`
	UploadSessionID            *string    `json:"upload_session_id,omitempty"`
	StagedExpiresAt            *time.Time `json:"staged_expires_at,omitempty"`
	AccessRevokedAt            *time.Time `json:"access_revoked_at,omitempty"`
	AccessRevokedReason        string     `json:"access_revoked_reason"`
	ObjectDeletedAt            *time.Time `json:"object_deleted_at,omitempty"`
}

type skuOriginSnapshot struct {
	ID     int64   `json:"id"`
	TaskID int64   `json:"task_id"`
	Origin *string `json:"sku_origin"`
}

type planningDetailSnapshot struct {
	TaskSKUItemID     int64  `json:"task_sku_item_id"`
	CurrentRevisionID *int64 `json:"current_revision_id"`
	LockVersion       int64  `json:"lock_version"`
}

type planningImageSnapshot struct {
	RevisionID   int64  `json:"revision_id"`
	StorageRefID string `json:"storage_ref_id"`
}

type planningStateSnapshot struct {
	TaskID             int64                    `json:"task_id"`
	SettingsExists     bool                     `json:"settings_exists"`
	ERPSyncMode        string                   `json:"erp_sync_mode"`
	CodeRuleRevisionID int64                    `json:"code_rule_revision_id"`
	ClientCreateID     string                   `json:"client_create_id"`
	CreatedBy          int64                    `json:"created_by"`
	Details            []planningDetailSnapshot `json:"details"`
	RevisionIDs        []int64                  `json:"revision_ids"`
	ImageRevisionIDs   []int64                  `json:"image_revision_ids"`
	Images             []planningImageSnapshot  `json:"images"`
}

type planningCreatedSnapshot struct {
	TaskID           int64   `json:"task_id"`
	SettingsCreated  bool    `json:"settings_created"`
	DetailIDs        []int64 `json:"detail_ids"`
	RevisionIDs      []int64 `json:"revision_ids"`
	ImageRevisionIDs []int64 `json:"image_revision_ids"`
}

type snapshot struct {
	Version             int                       `json:"version"`
	ToolVersion         string                    `json:"tool_version"`
	SchemaVersion       string                    `json:"schema_version"`
	Database            string                    `json:"database"`
	MappingSHA256       string                    `json:"mapping_sha256"`
	IntegritySHA256     string                    `json:"integrity_sha256"`
	ApplyState          string                    `json:"apply_state"`
	CreatedAt           time.Time                 `json:"created_at"`
	AppliedAt           *time.Time                `json:"applied_at"`
	Tasks               []taskSnapshot            `json:"tasks_before"`
	AfterTasks          []taskSnapshot            `json:"tasks_after"`
	ResourceGroups      []resourceGroupSnapshot   `json:"resource_groups_before"`
	AfterResourceGroups []resourceGroupSnapshot   `json:"resource_groups_after"`
	AssetBindings       []assetBindingSnapshot    `json:"asset_bindings_before"`
	AfterAssetBindings  []assetBindingSnapshot    `json:"asset_bindings_after"`
	SKUOrigins          []skuOriginSnapshot       `json:"sku_origins_before"`
	AfterSKUOrigins     []skuOriginSnapshot       `json:"sku_origins_after"`
	PlanningBefore      []planningStateSnapshot   `json:"planning_before"`
	PlanningAfter       []planningStateSnapshot   `json:"planning_after"`
	PlanningCreated     []planningCreatedSnapshot `json:"planning_created"`
	InsertedGroupIDs    []int64                   `json:"inserted_group_ids"`
	AppliedRevisionIDs  []int64                   `json:"applied_revision_ids"`
}

type report struct {
	Mode                   string                   `json:"mode"`
	Database               string                   `json:"database"`
	GeneratedAt            time.Time                `json:"generated_at"`
	Counts                 map[string]int64         `json:"counts"`
	StateCounts            map[string]int64         `json:"state_counts"`
	ManualTaskIDs          []int64                  `json:"manual_task_ids"`
	ManualTaskIssues       []taskMigrationIssue     `json:"manual_task_issues"`
	ManualResourceGroupIDs []int64                  `json:"manual_resource_group_ids"`
	ManualAccessUserIDs    []int64                  `json:"manual_access_user_ids"`
	ManualAccessIssues     []accessMigrationIssue   `json:"manual_access_issues"`
	ManualOrgIssues        []orgMigrationIssue      `json:"manual_org_issues"`
	ManualResourceIssues   []resourceMigrationIssue `json:"manual_resource_issues"`
	MappingResourceCount   int                      `json:"mapping_resource_count"`
	MappingPlanningCount   int                      `json:"mapping_planning_count"`
	Warnings               []string                 `json:"warnings"`
}

type orgMigrationIssue struct {
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	Reason      string `json:"reason"`
}

type accessMigrationIssue struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type taskMigrationIssue struct {
	TaskID int64  `json:"task_id"`
	Reason string `json:"reason"`
}

type resourceMigrationIssue struct {
	GroupID int64  `json:"group_id"`
	TaskID  int64  `json:"task_id"`
	Reason  string `json:"reason"`
}

type cutoverBlockers struct {
	Org       []orgMigrationIssue
	Access    []accessMigrationIssue
	Tasks     []taskMigrationIssue
	Resources []resourceMigrationIssue
}

func (b cutoverBlockers) Empty() bool {
	return len(b.Org) == 0 && len(b.Access) == 0 && len(b.Tasks) == 0 && len(b.Resources) == 0
}

func main() {
	var o options
	flag.StringVar(&o.DSN, "dsn", os.Getenv("MYSQL_DSN"), "MySQL DSN")
	flag.BoolVar(&o.DryRun, "dry-run", true, "read-only analysis and report (default)")
	flag.BoolVar(&o.Apply, "apply", false, "apply deterministic state and explicit mapping changes")
	flag.BoolVar(&o.Rollback, "rollback", false, "rollback this tool's apply using its snapshot manifest")
	flag.StringVar(&o.SnapshotDir, "snapshot-dir", "", "required for apply/rollback; external DB/object snapshots remain operator-owned")
	flag.IntVar(&o.BatchSize, "batch-size", 500, "read/report batch size")
	flag.StringVar(&o.MappingFile, "mapping-file", "", "JSON file containing only manually confirmed resource/planning mappings")
	flag.StringVar(&o.ReportFile, "report-file", "", "optional JSON report path")
	flag.StringVar(&o.ConfirmDB, "confirm-database", "", "required for writes and must exactly match SELECT DATABASE()")
	flag.Parse()

	if err := run(context.Background(), o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(v1migrate.ExitCode(err))
	}
}

func run(ctx context.Context, o options) error {
	if err := validateOptions(o); err != nil {
		return err
	}
	db, err := v1migrate.OpenDB(o.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	database, err := currentDatabase(ctx, db)
	if err != nil {
		return err
	}
	if (o.Apply || o.Rollback) && o.ConfirmDB != database {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "write guard: --confirm-database=%q does not match %q", o.ConfirmDB, database)
	}

	mapping, err := readMapping(o.MappingFile)
	if err != nil {
		return err
	}
	if err := validateMapping(mapping); err != nil {
		return err
	}

	switch {
	case o.Rollback:
		return rollback(ctx, db, database, o, mapping)
	case o.Apply:
		if err := apply(ctx, db, database, o, mapping); err != nil {
			return err
		}
	}

	r, err := buildReport(ctx, db, database, mapping)
	if err != nil {
		return err
	}
	if o.Apply {
		r.Mode = "apply"
	} else {
		r.Mode = "dry-run"
	}
	return writeJSONReport(r, o.ReportFile)
}

func validateOptions(o options) error {
	if o.BatchSize < 1 || o.BatchSize > 10000 {
		return fmt.Errorf("--batch-size must be between 1 and 10000")
	}
	if o.Apply && o.Rollback {
		return fmt.Errorf("--apply and --rollback are mutually exclusive")
	}
	if (o.Apply || o.Rollback) && strings.TrimSpace(o.SnapshotDir) == "" {
		return fmt.Errorf("--snapshot-dir is required for apply/rollback")
	}
	if (o.Apply || o.Rollback) && strings.TrimSpace(o.MappingFile) == "" {
		return fmt.Errorf("--mapping-file is required for apply/rollback so the journal stays bound to the reviewed mapping")
	}
	return nil
}

func readMapping(path string) (mappingFile, error) {
	if strings.TrimSpace(path) == "" {
		return mappingFile{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return mappingFile{}, err
	}
	var m mappingFile
	if err := json.Unmarshal(raw, &m); err != nil {
		return mappingFile{}, fmt.Errorf("decode mapping file: %w", err)
	}
	return m, nil
}

func validateMapping(m mappingFile) error {
	seenResource := map[string]struct{}{}
	resourceTasks := map[int64]struct{}{}
	for i, r := range m.Resources {
		if r.TaskID <= 0 || r.CreatedBy <= 0 {
			return fmt.Errorf("resources[%d]: task_id and created_by are required", i)
		}
		if r.ScopeKind != "task" && r.ScopeKind != "sku" && r.ScopeKind != "retouch_requirement" {
			return fmt.Errorf("resources[%d]: invalid scope_kind", i)
		}
		if (r.ScopeKind == "task" && r.ScopeRefID != 0) || (r.ScopeKind != "task" && r.ScopeRefID <= 0) {
			return fmt.Errorf("resources[%d]: invalid scope_ref_id", i)
		}
		if r.TargetStatus == "shell" {
			if r.Mode != "" || r.SourceAssetID != nil || len(r.FinalAssetIDs) != 0 || len(r.ReferenceIDs) != 0 {
				return fmt.Errorf("resources[%d]: shell mappings cannot contain resource files", i)
			}
		} else if (r.Mode == "single" && len(r.FinalAssetIDs) != 1) || (r.Mode == "set" && len(r.FinalAssetIDs) < 2) {
			return fmt.Errorf("resources[%d]: single requires one final and set requires at least two", i)
		}
		if r.TargetStatus != "shell" && r.Mode != "single" && r.Mode != "set" {
			return fmt.Errorf("resources[%d]: invalid mode", i)
		}
		if r.TargetStatus != "shell" && r.TargetStatus != "draft" && r.TargetStatus != "submitted" && r.TargetStatus != "finalized" {
			return fmt.Errorf("resources[%d]: target_status must be shell, draft, submitted, or finalized", i)
		}
		seenFinal := map[int64]struct{}{}
		for _, assetID := range r.FinalAssetIDs {
			if assetID <= 0 {
				return fmt.Errorf("resources[%d]: final asset ids must be positive", i)
			}
			if _, duplicate := seenFinal[assetID]; duplicate {
				return fmt.Errorf("resources[%d]: final asset ids must be unique and ordered", i)
			}
			seenFinal[assetID] = struct{}{}
			if r.SourceAssetID != nil && assetID == *r.SourceAssetID {
				return fmt.Errorf("resources[%d]: source asset cannot also be a final asset", i)
			}
		}
		key := fmt.Sprintf("%d/%s/%d", r.TaskID, r.ScopeKind, r.ScopeRefID)
		if _, ok := seenResource[key]; ok {
			return fmt.Errorf("resources[%d]: duplicate scope %s", i, key)
		}
		seenResource[key] = struct{}{}
		resourceTasks[r.TaskID] = struct{}{}
	}
	seenPlanning := map[int64]struct{}{}
	for i, p := range m.Planning {
		if p.TaskID <= 0 || p.CodeRuleRevisionID <= 0 || p.CreatedBy <= 0 || len(p.Items) == 0 {
			return fmt.Errorf("planning_tasks[%d]: task/rule/creator and at least one item are required", i)
		}
		if _, ok := seenPlanning[p.TaskID]; ok {
			return fmt.Errorf("planning_tasks[%d]: duplicate task_id", i)
		}
		if _, hasResources := resourceTasks[p.TaskID]; hasResources {
			return fmt.Errorf("planning_tasks[%d]: task %d cannot migrate both planning data and design resources", i, p.TaskID)
		}
		seenPlanning[p.TaskID] = struct{}{}
		for j, item := range p.Items {
			if item.TaskSKUItemID <= 0 || strings.TrimSpace(item.DescriptionSpec) == "" || item.Quantity <= 0 {
				return fmt.Errorf("planning_tasks[%d].items[%d]: item id, description and positive quantity are required", i, j)
			}
		}
	}
	return nil
}

func currentDatabase(ctx context.Context, db *sql.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func buildReport(ctx context.Context, db *sql.DB, database string, m mappingFile) (report, error) {
	r := report{Database: database, GeneratedAt: time.Now().UTC(), Counts: map[string]int64{}, StateCounts: map[string]int64{}, MappingResourceCount: len(m.Resources), MappingPlanningCount: len(m.Planning)}
	queries := map[string]string{
		"tasks":                           `SELECT COUNT(*) FROM tasks`,
		"tasks_without_stable_department": `SELECT COUNT(*) FROM tasks WHERE owner_department_id IS NULL`,
		"users_without_stable_department": `SELECT COUNT(*) FROM users WHERE department_id IS NULL`,
		"tasks_without_resource_groups":   `SELECT COUNT(*) FROM tasks t LEFT JOIN task_asset_groups g ON g.task_id=t.id WHERE g.id IS NULL AND t.task_type <> 'sku_planning'`,
		"legacy_planning_candidates":      `SELECT COUNT(*) FROM tasks WHERE task_type='purchase_task'`,
		"migration_incomplete_groups":     `SELECT COUNT(*) FROM task_asset_groups WHERE migration_incomplete=1`,
	}
	for name, query := range queries {
		var count int64
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return r, fmt.Errorf("report %s: %w", name, err)
		}
		r.Counts[name] = count
	}
	rows, err := db.QueryContext(ctx, `SELECT task_status, COUNT(*) FROM tasks GROUP BY task_status ORDER BY task_status`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return r, err
		}
		r.StateCounts[status] = n
	}
	rows.Close()
	blockers, err := queryCutoverBlockers(ctx, db, m)
	if err != nil {
		return r, err
	}
	r.ManualOrgIssues = blockers.Org
	r.ManualAccessIssues = blockers.Access
	r.ManualTaskIssues = blockers.Tasks
	r.ManualResourceIssues = blockers.Resources
	for _, item := range blockers.Tasks {
		r.ManualTaskIDs = append(r.ManualTaskIDs, item.TaskID)
	}
	seenAccessUsers := map[int64]struct{}{}
	for _, item := range blockers.Access {
		if _, exists := seenAccessUsers[item.UserID]; !exists {
			seenAccessUsers[item.UserID] = struct{}{}
			r.ManualAccessUserIDs = append(r.ManualAccessUserIDs, item.UserID)
		}
	}
	for _, item := range blockers.Resources {
		r.ManualResourceGroupIDs = append(r.ManualResourceGroupIDs, item.GroupID)
	}
	if len(r.ManualTaskIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_task_ids require explicit mapping; the tool never infers missing business facts")
	}
	if len(r.ManualAccessUserIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_access_user_ids retain only Member until an administrator confirms explicit assignments")
	}
	if len(r.ManualOrgIssues) > 0 {
		r.Warnings = append(r.Warnings, "manual_org_issues block apply; stable organization ids are never inferred from ambiguous names")
	}
	if len(r.ManualResourceGroupIDs) > 0 {
		r.Warnings = append(r.Warnings, "manual_resource_group_ids lack a confirmed ordered mapping; apply is blocked until every migration marker is resolved")
	}
	return r, nil
}

func queryCutoverBlockers(ctx context.Context, q snapshotQueryer, m mappingFile) (cutoverBlockers, error) {
	var blockers cutoverBlockers
	var err error
	blockers.Org, err = queryOrgMigrationIssues(ctx, q)
	if err != nil {
		return blockers, err
	}
	rows, err := q.QueryContext(ctx, `
		SELECT ur.user_id,ur.role
		FROM user_roles ur
		JOIN users u ON u.id=ur.user_id AND u.status='active'
		WHERE ur.role NOT IN ('Member','SuperAdmin','Admin','RoleAdmin','Ops','Designer','CustomizationOperator','Audit_A','Audit_B','DesignReviewer','CustomizationReviewer','AssetSubmitter','AssetManager','AssetTemplateAdmin','AssetSettlement','HRAdmin','DepartmentAdmin','TeamLead','DesignDirector','ERP')
		   OR (ur.role IN ('DepartmentAdmin','DesignDirector') AND u.department_id IS NULL)
		   OR (ur.role='TeamLead' AND u.team_id IS NULL)
		ORDER BY ur.user_id,ur.role`)
	if err != nil {
		return blockers, err
	}
	for rows.Next() {
		var item accessMigrationIssue
		if err := rows.Scan(&item.UserID, &item.Role); err != nil {
			rows.Close()
			return blockers, err
		}
		switch item.Role {
		case "DepartmentAdmin", "DesignDirector":
			item.Reason = "role migration requires a stable department id"
		case "TeamLead":
			item.Reason = "role migration requires a stable team id"
		default:
			item.Reason = "active legacy role requires an explicit administrator-reviewed assignment before auth cutover"
		}
		blockers.Access = append(blockers.Access, item)
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	mappedPlanning := map[int64]planningMapping{}
	duplicatePlanningTasks := map[int64]bool{}
	for _, item := range m.Planning {
		if item.TaskID > 0 && item.CodeRuleRevisionID > 0 && item.CreatedBy > 0 && len(item.Items) > 0 {
			if _, exists := mappedPlanning[item.TaskID]; exists {
				duplicatePlanningTasks[item.TaskID] = true
			}
			mappedPlanning[item.TaskID] = item
		}
	}
	rows, err = q.QueryContext(ctx, `SELECT id,task_type,task_status FROM tasks WHERE task_type='purchase_task' OR task_status='RejectedByWarehouse' ORDER BY id`)
	if err != nil {
		return blockers, err
	}
	type taskCandidate struct {
		ID     int64
		Type   string
		Status string
	}
	taskCandidates := []taskCandidate{}
	for rows.Next() {
		var taskID int64
		var taskType, taskStatus string
		if err := rows.Scan(&taskID, &taskType, &taskStatus); err != nil {
			rows.Close()
			return blockers, err
		}
		taskCandidates = append(taskCandidates, taskCandidate{ID: taskID, Type: taskType, Status: taskStatus})
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	taskTypeByID := map[int64]string{}
	for _, candidate := range taskCandidates {
		taskTypeByID[candidate.ID] = candidate.Type
	}
	for _, candidate := range taskCandidates {
		taskID := candidate.ID
		if candidate.Status == "RejectedByWarehouse" {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "warehouse rejection has no deterministic v8 state mapping"})
			continue
		}
		if candidate.Type == "purchase_task" {
			_, mapped := mappedPlanning[taskID]
			if !mapped {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "legacy purchase task has no complete administrator-confirmed planning SKU mapping"})
			}
		}
	}
	planningTaskIDs := make([]int64, 0, len(mappedPlanning))
	for taskID := range mappedPlanning {
		planningTaskIDs = append(planningTaskIDs, taskID)
	}
	sort.Slice(planningTaskIDs, func(i, j int) bool { return planningTaskIDs[i] < planningTaskIDs[j] })
	for _, taskID := range planningTaskIDs {
		mapping := mappedPlanning[taskID]
		taskType, known := taskTypeByID[taskID]
		if !known {
			if err := q.QueryRowContext(ctx, `SELECT task_type FROM tasks WHERE id=?`, taskID).Scan(&taskType); errors.Is(err, sql.ErrNoRows) {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: "planning mapping references a missing task"})
				continue
			} else if err != nil {
				return blockers, err
			}
		}
		if taskType != "purchase_task" && taskType != "sku_planning" {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{TaskID: taskID, Reason: fmt.Sprintf("planning mapping targets task_type=%s, expected purchase_task or an idempotent sku_planning rerun", taskType)})
			continue
		}
		databaseItemIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return blockers, err
		}
		mappingItemIDs := make([]int64, 0, len(mapping.Items))
		seenItemIDs := map[int64]struct{}{}
		duplicate := duplicatePlanningTasks[taskID]
		for _, item := range mapping.Items {
			if _, exists := seenItemIDs[item.TaskSKUItemID]; exists {
				duplicate = true
			}
			seenItemIDs[item.TaskSKUItemID] = struct{}{}
			mappingItemIDs = append(mappingItemIDs, item.TaskSKUItemID)
		}
		sort.Slice(mappingItemIDs, func(i, j int) bool { return mappingItemIDs[i] < mappingItemIDs[j] })
		if duplicate || !equalInt64Slices(databaseItemIDs, mappingItemIDs) {
			blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{
				TaskID: taskID,
				Reason: fmt.Sprintf(
					"planning mapping SKU ids must exactly match task_sku_items (database=%v mapping=%v duplicate=%v)",
					databaseItemIDs, mappingItemIDs, duplicate,
				),
			})
			continue
		}
		if taskType == "sku_planning" {
			if err := verifyPlanningMappingQuery(ctx, q, mapping); err != nil {
				blockers.Tasks = append(blockers.Tasks, taskMigrationIssue{
					TaskID: taskID,
					Reason: fmt.Sprintf("existing sku_planning task does not match the reviewed idempotent mapping: %v", err),
				})
			}
		}
	}
	mappedGroups := map[string]struct{}{}
	for _, item := range m.Resources {
		mappedGroups[fmt.Sprintf("%d/%s/%d", item.TaskID, item.ScopeKind, item.ScopeRefID)] = struct{}{}
	}
	rows, err = q.QueryContext(ctx, `SELECT id,task_id,scope_kind,scope_ref_id FROM task_asset_groups WHERE migration_incomplete=1 ORDER BY id`)
	if err != nil {
		return blockers, err
	}
	for rows.Next() {
		var id, taskID, scopeRefID int64
		var scopeKind string
		if err := rows.Scan(&id, &taskID, &scopeKind, &scopeRefID); err != nil {
			rows.Close()
			return blockers, err
		}
		if _, mapped := mappedGroups[fmt.Sprintf("%d/%s/%d", taskID, scopeKind, scopeRefID)]; !mapped {
			blockers.Resources = append(blockers.Resources, resourceMigrationIssue{
				GroupID: id,
				TaskID:  taskID,
				Reason:  "migration-incomplete resource group has no administrator-confirmed ordered mapping",
			})
		}
	}
	if err := rows.Close(); err != nil {
		return blockers, err
	}
	for _, mapping := range m.Resources {
		issue, err := validateResourceMappingPreflight(ctx, q, mapping)
		if err != nil {
			return blockers, err
		}
		if issue != nil {
			blockers.Resources = append(blockers.Resources, *issue)
		}
	}
	return blockers, nil
}

func validateResourceMappingPreflight(ctx context.Context, q snapshotQueryer, m resourceMapping) (*resourceMigrationIssue, error) {
	issue := func(groupID int64, format string, args ...interface{}) *resourceMigrationIssue {
		return &resourceMigrationIssue{GroupID: groupID, TaskID: m.TaskID, Reason: fmt.Sprintf(format, args...)}
	}
	var taskType, taskStatus string
	if err := q.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=?`, m.TaskID).Scan(&taskType, &taskStatus); errors.Is(err, sql.ErrNoRows) {
		return issue(0, "resource mapping references a missing task"), nil
	} else if err != nil {
		return nil, err
	}
	var creatorCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=?`, m.CreatedBy).Scan(&creatorCount); err != nil {
		return nil, err
	}
	if creatorCount != 1 {
		return issue(0, "resource mapping creator %d does not exist", m.CreatedBy), nil
	}
	if m.ScopeKind == "sku" || m.ScopeKind == "retouch_requirement" {
		table := "task_sku_items"
		if m.ScopeKind == "retouch_requirement" {
			table = "task_retouch_requirements"
		}
		var scopeCount int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id=? AND task_id=?`, m.ScopeRefID, m.TaskID).Scan(&scopeCount); err != nil {
			return nil, err
		}
		if scopeCount != 1 {
			return issue(0, "%s scope %d does not belong to task", m.ScopeKind, m.ScopeRefID), nil
		}
	}
	var groupID int64
	err := q.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=?`, m.TaskID, m.ScopeKind, m.ScopeRefID).Scan(&groupID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		var revisionCount int
		var incomplete bool
		if err := q.QueryRowContext(ctx, `SELECT COUNT(r.id),migration_incomplete FROM task_asset_group_revisions r RIGHT JOIN task_asset_groups g ON g.id=r.group_id WHERE g.id=? GROUP BY g.id,migration_incomplete`, groupID).Scan(&revisionCount, &incomplete); err != nil {
			return nil, err
		}
		if revisionCount > 0 {
			if incomplete {
				return issue(groupID, "resource group has revisions but remains migration_incomplete"), nil
			}
			if err := verifyResourceMappingQuery(ctx, q, groupID, m); err != nil {
				return issue(groupID, "existing resource revision differs from reviewed mapping: %v", err), nil
			}
			return nil, nil
		}
	}
	effectiveStatus := migratedTaskStatus(taskStatus)
	if m.TargetStatus == "shell" {
		if effectiveStatus == "PendingAudit" || effectiveStatus == "Completed" {
			return issue(groupID, "task status %s cannot be migrated as an empty shell", effectiveStatus), nil
		}
		return nil, nil
	}
	if taskType != "retouch_task" && m.SourceAssetID == nil {
		return issue(groupID, "design/customization resource mapping has no confirmed source file"), nil
	}
	if effectiveStatus == "Completed" && m.TargetStatus != "finalized" {
		return issue(groupID, "completed task requires a finalized resource mapping"), nil
	}
	if effectiveStatus == "PendingAudit" && m.TargetStatus != "submitted" {
		return issue(groupID, "pending-audit task requires a submitted resource mapping"), nil
	}
	assetIDs := append([]int64(nil), m.FinalAssetIDs...)
	if m.SourceAssetID != nil {
		assetIDs = append(assetIDs, *m.SourceAssetID)
	}
	for _, assetID := range assetIDs {
		var ownerTaskID int64
		if err := q.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id=?`, assetID).Scan(&ownerTaskID); errors.Is(err, sql.ErrNoRows) {
			return issue(groupID, "task asset %d is missing", assetID), nil
		} else if err != nil {
			return nil, err
		}
		if ownerTaskID != m.TaskID {
			return issue(groupID, "task asset %d belongs to task %d", assetID, ownerTaskID), nil
		}
	}
	for _, referenceID := range m.ReferenceIDs {
		var ownerTaskID int64
		var skuItemID, retouchRequirementID sql.NullInt64
		if err := q.QueryRowContext(ctx, `SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs WHERE id=?`, referenceID).Scan(&ownerTaskID, &skuItemID, &retouchRequirementID); errors.Is(err, sql.ErrNoRows) {
			return issue(groupID, "reference file %d is missing", referenceID), nil
		} else if err != nil {
			return nil, err
		}
		if ownerTaskID != m.TaskID || !referenceMatchesResourceScope(m.ScopeKind, m.ScopeRefID, skuItemID, retouchRequirementID) {
			return issue(groupID, "reference file %d does not belong to the mapped task scope", referenceID), nil
		}
	}
	return nil, nil
}

func referenceMatchesResourceScope(scopeKind string, scopeRefID int64, skuItemID, retouchRequirementID sql.NullInt64) bool {
	// Task-level references may be inherited by one concrete group. Once a
	// reference declares a SKU or retouch requirement, it must never cross into
	// another discriminator. A row declaring both is invalid for every group.
	if skuItemID.Valid && retouchRequirementID.Valid {
		return false
	}
	switch scopeKind {
	case "task":
		return !skuItemID.Valid && !retouchRequirementID.Valid
	case "sku":
		return !retouchRequirementID.Valid && (!skuItemID.Valid || skuItemID.Int64 == scopeRefID)
	case "retouch_requirement":
		return !skuItemID.Valid && (!retouchRequirementID.Valid || retouchRequirementID.Int64 == scopeRefID)
	default:
		return false
	}
}

func migratedTaskStatus(status string) string {
	switch status {
	case "PendingAuditA", "PendingAuditB", "PendingCustomizationReview", "PendingOutsourceReview", "PendingEffectReview":
		return "PendingAudit"
	case "RejectedByAuditA", "RejectedByAuditB", "PendingCustomizationProduction", "PendingEffectRevision", "PendingOutsource", "Outsourcing":
		return "InProgress"
	case "PendingWarehouseQC", "PendingWarehouseReceive", "PendingProductionTransfer", "PendingClose":
		return "Completed"
	default:
		return status
	}
}

func requireNoCutoverBlockers(blockers cutoverBlockers) error {
	if blockers.Empty() {
		return nil
	}
	return v1migrate.NewHardAbort(
		v1migrate.ExitCodeHardAbort,
		"cutover preflight blocked: org=%d access=%d tasks=%d resource_groups=%d; run --dry-run and resolve every listed ID before enabling auth/workflow v8",
		len(blockers.Org), len(blockers.Access), len(blockers.Tasks), len(blockers.Resources),
	)
}

func queryOrgMigrationIssues(ctx context.Context, q snapshotQueryer) ([]orgMigrationIssue, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT subject_type,subject_id,reason FROM (
		  SELECT 'user' AS subject_type,u.id AS subject_id,
		    CASE
		      WHEN NULLIF(TRIM(u.department),'') IS NOT NULL AND u.department_id IS NULL THEN 'department name missing or ambiguous'
		      WHEN NULLIF(TRIM(u.team),'') IS NOT NULL AND u.team_id IS NULL THEN 'team name missing or ambiguous within stable department'
		      WHEN u.team_id IS NOT NULL AND (u.department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=u.team_id AND ot.department_id=u.department_id)) THEN 'department/team stable ids are inconsistent'
		    END AS reason
		  FROM users u
		  WHERE (NULLIF(TRIM(u.department),'') IS NOT NULL AND u.department_id IS NULL)
		     OR (NULLIF(TRIM(u.team),'') IS NOT NULL AND u.team_id IS NULL)
		     OR (u.team_id IS NOT NULL AND (u.department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=u.team_id AND ot.department_id=u.department_id)))
		  UNION ALL
		  SELECT 'task',t.id,
		    CASE
		      WHEN NULLIF(TRIM(t.owner_department),'') IS NOT NULL AND t.owner_department_id IS NULL THEN 'owner department name missing or ambiguous'
		      WHEN COALESCE(NULLIF(TRIM(t.owner_org_team),''),NULLIF(TRIM(t.owner_team),'')) IS NOT NULL AND t.owner_team_id IS NULL THEN 'owner team name missing, ambiguous, or conflicting'
		      WHEN t.owner_team_id IS NOT NULL AND (t.owner_department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=t.owner_team_id AND ot.department_id=t.owner_department_id)) THEN 'owner department/team stable ids are inconsistent'
		    END
		  FROM tasks t
		  WHERE (NULLIF(TRIM(t.owner_department),'') IS NOT NULL AND t.owner_department_id IS NULL)
		     OR (COALESCE(NULLIF(TRIM(t.owner_org_team),''),NULLIF(TRIM(t.owner_team),'')) IS NOT NULL AND t.owner_team_id IS NULL)
		     OR (t.owner_team_id IS NOT NULL AND (t.owner_department_id IS NULL OR NOT EXISTS (SELECT 1 FROM org_teams ot WHERE ot.id=t.owner_team_id AND ot.department_id=t.owner_department_id)))
		) issues ORDER BY subject_type,subject_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []orgMigrationIssue{}
	for rows.Next() {
		var item orgMigrationIssue
		if err := rows.Scan(&item.SubjectType, &item.SubjectID, &item.Reason); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}
	return issues, rows.Err()
}

func queryIDs(ctx context.Context, db *sql.DB, query string) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type snapshotQueryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func queryInt64IDs(ctx context.Context, q snapshotQueryer, query string, args ...interface{}) ([]int64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func queryStringIDs(ctx context.Context, q snapshotQueryer, query string, args ...interface{}) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func planningTaskIDs(m mappingFile) []int64 {
	ids := make([]int64, 0, len(m.Planning))
	for _, item := range m.Planning {
		ids = append(ids, item.TaskID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func uniqueSortedInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func lockInt64Rows(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func lockCutoverTargets(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	taskIDs, err := lockInt64Rows(ctx, tx, `
		SELECT id FROM tasks
		WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')
		   OR task_type='purchase_task'
		ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	for _, item := range m.Resources {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.Planning {
		taskIDs = append(taskIDs, item.TaskID)
	}
	taskIDs = uniqueSortedInt64(taskIDs)
	for _, taskID := range taskIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM tasks WHERE id=? FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	for _, taskID := range taskIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE migration_incomplete=1 ORDER BY id FOR UPDATE`); err != nil {
		return err
	}
	resources := append([]resourceMapping(nil), m.Resources...)
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].TaskID != resources[j].TaskID {
			return resources[i].TaskID < resources[j].TaskID
		}
		if resources[i].ScopeKind != resources[j].ScopeKind {
			return resources[i].ScopeKind < resources[j].ScopeKind
		}
		return resources[i].ScopeRefID < resources[j].ScopeRefID
	})
	groupIDs := []int64{}
	for _, taskID := range taskIDs {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	for _, item := range resources {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`, item.TaskID, item.ScopeKind, item.ScopeRefID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	groupIDs = uniqueSortedInt64(groupIDs)
	for _, groupID := range groupIDs {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id FOR UPDATE`, groupID); err != nil {
			return err
		}
	}
	assetIDs := []int64{}
	for _, taskID := range taskIDs {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		assetIDs = append(assetIDs, ids...)
	}
	for _, item := range resources {
		if item.SourceAssetID != nil {
			assetIDs = append(assetIDs, *item.SourceAssetID)
		}
		assetIDs = append(assetIDs, item.FinalAssetIDs...)
	}
	for _, assetID := range uniqueSortedInt64(assetIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE id=? FOR UPDATE`, assetID); err != nil {
			return err
		}
	}
	referenceIDs := []int64{}
	for _, item := range resources {
		referenceIDs = append(referenceIDs, item.ReferenceIDs...)
	}
	for _, referenceID := range uniqueSortedInt64(referenceIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM reference_file_refs WHERE id=? FOR UPDATE`, referenceID); err != nil {
			return err
		}
	}
	for _, taskID := range planningTaskIDs(m) {
		queries := []string{
			`SELECT task_id FROM task_planning_settings WHERE task_id=? FOR UPDATE`,
			`SELECT d.task_sku_item_id FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=? ORDER BY d.task_sku_item_id FOR UPDATE`,
			`SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id FOR UPDATE`,
			`SELECT i.revision_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id FOR UPDATE`,
		}
		for _, query := range queries {
			if _, err := lockInt64Rows(ctx, tx, query, taskID); err != nil {
				return err
			}
		}
	}
	for _, taskID := range taskIDs {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func captureSKUOrigins(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]skuOriginSnapshot, error) {
	items := []skuOriginSnapshot{}
	for _, taskID := range taskIDs {
		rows, err := q.QueryContext(ctx, `SELECT id,task_id,sku_origin FROM task_sku_items WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item skuOriginSnapshot
			var origin sql.NullString
			if err := rows.Scan(&item.ID, &item.TaskID, &origin); err != nil {
				rows.Close()
				return nil, err
			}
			if origin.Valid {
				value := origin.String
				item.Origin = &value
			}
			items = append(items, item)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func capturePlanningStates(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]planningStateSnapshot, error) {
	states := make([]planningStateSnapshot, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		state := planningStateSnapshot{TaskID: taskID, Details: []planningDetailSnapshot{}, RevisionIDs: []int64{}, ImageRevisionIDs: []int64{}, Images: []planningImageSnapshot{}}
		err := q.QueryRowContext(ctx, `SELECT erp_sync_mode,code_rule_revision_id,client_create_id,created_by FROM task_planning_settings WHERE task_id=?`, taskID).
			Scan(&state.ERPSyncMode, &state.CodeRuleRevisionID, &state.ClientCreateID, &state.CreatedBy)
		if errors.Is(err, sql.ErrNoRows) {
			state.SettingsExists = false
		} else if err != nil {
			return nil, err
		} else {
			state.SettingsExists = true
		}
		rows, err := q.QueryContext(ctx, `
			SELECT d.task_sku_item_id,d.current_revision_id,d.lock_version
			FROM task_planning_sku_details d
			JOIN task_sku_items si ON si.id=d.task_sku_item_id
			WHERE si.task_id=? ORDER BY d.task_sku_item_id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var detail planningDetailSnapshot
			var revisionID sql.NullInt64
			if err := rows.Scan(&detail.TaskSKUItemID, &revisionID, &detail.LockVersion); err != nil {
				rows.Close()
				return nil, err
			}
			if revisionID.Valid {
				value := revisionID.Int64
				detail.CurrentRevisionID = &value
			}
			state.Details = append(state.Details, detail)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		state.RevisionIDs, err = queryInt64IDs(ctx, q, `SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id`, taskID)
		if err != nil {
			return nil, err
		}
		rows, err = q.QueryContext(ctx, `SELECT i.revision_id,i.storage_ref_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id`, taskID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var image planningImageSnapshot
			if err := rows.Scan(&image.RevisionID, &image.StorageRefID); err != nil {
				rows.Close()
				return nil, err
			}
			state.ImageRevisionIDs = append(state.ImageRevisionIDs, image.RevisionID)
			state.Images = append(state.Images, image)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func loadTaskSnapshot(ctx context.Context, q snapshotQueryer, id int64) (taskSnapshot, error) {
	var item taskSnapshot
	var handlerID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT id,task_type,task_status,workflow_revision,current_handler_id FROM tasks WHERE id=?`, id).
		Scan(&item.ID, &item.TaskType, &item.TaskStatus, &item.WorkflowRevision, &handlerID); err != nil {
		return item, err
	}
	if handlerID.Valid {
		value := handlerID.Int64
		item.CurrentHandlerID = &value
	}
	var err error
	item.EventIDs, err = queryStringIDs(ctx, q, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id`, id)
	return item, err
}

func loadResourceGroupSnapshot(ctx context.Context, q snapshotQueryer, id int64) (resourceGroupSnapshot, error) {
	var item resourceGroupSnapshot
	var workingID, finalizedID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT id,task_id,working_revision_id,finalized_revision_id,lock_version,migration_incomplete,migration_issue FROM task_asset_groups WHERE id=?`, id).
		Scan(&item.ID, &item.TaskID, &workingID, &finalizedID, &item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue); err != nil {
		return item, err
	}
	if workingID.Valid {
		value := workingID.Int64
		item.WorkingRevisionID = &value
	}
	if finalizedID.Valid {
		value := finalizedID.Int64
		item.FinalizedRevisionID = &value
	}
	var err error
	item.RevisionIDs, err = queryInt64IDs(ctx, q, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id`, id)
	return item, err
}

func captureResourceGroupsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]resourceGroupSnapshot, error) {
	items := []resourceGroupSnapshot{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		groupIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for _, groupID := range groupIDs {
			item, err := loadResourceGroupSnapshot(ctx, q, groupID)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func loadAssetBindingSnapshot(ctx context.Context, q snapshotQueryer, id int64) (assetBindingSnapshot, error) {
	var item assetBindingSnapshot
	var boundGroupID, stagedSKUItemID, stagedRetouchID, stagedBy sql.NullInt64
	var boundRole, stagedRole, uploadSessionID sql.NullString
	var stagedExpiresAt, revokedAt, objectDeletedAt sql.NullTime
	if err := q.QueryRowContext(ctx, `
		SELECT id,task_id,binding_state,bound_group_id,bound_role,
		       staged_task_sku_item_id,staged_retouch_requirement_id,staged_role,staged_by,upload_session_id,staged_expires_at,
		       access_revoked_at,access_revoked_reason,object_deleted_at
		FROM task_assets WHERE id=?`, id).
		Scan(&item.ID, &item.TaskID, &item.BindingState, &boundGroupID, &boundRole,
			&stagedSKUItemID, &stagedRetouchID, &stagedRole, &stagedBy, &uploadSessionID, &stagedExpiresAt,
			&revokedAt, &item.AccessRevokedReason, &objectDeletedAt); err != nil {
		return item, err
	}
	item.BoundGroupID = nullInt64Pointer(boundGroupID)
	item.BoundRole = nullStringPointer(boundRole)
	item.StagedTaskSKUItemID = nullInt64Pointer(stagedSKUItemID)
	item.StagedRetouchRequirementID = nullInt64Pointer(stagedRetouchID)
	item.StagedRole = nullStringPointer(stagedRole)
	item.StagedBy = nullInt64Pointer(stagedBy)
	item.UploadSessionID = nullStringPointer(uploadSessionID)
	item.StagedExpiresAt = nullTimePointer(stagedExpiresAt)
	item.AccessRevokedAt = nullTimePointer(revokedAt)
	item.ObjectDeletedAt = nullTimePointer(objectDeletedAt)
	return item, nil
}

func captureAssetBindingsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]assetBindingSnapshot, error) {
	items := []assetBindingSnapshot{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		assetIDs, err := queryInt64IDs(ctx, q, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id`, taskID)
		if err != nil {
			return nil, err
		}
		for _, assetID := range assetIDs {
			item, err := loadAssetBindingSnapshot(ctx, q, assetID)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func populateAfterSnapshot(ctx context.Context, tx *sql.Tx, s *snapshot, m mappingFile) error {
	s.AfterTasks = make([]taskSnapshot, 0, len(s.Tasks))
	for _, before := range s.Tasks {
		item, err := loadTaskSnapshot(ctx, tx, before.ID)
		if err != nil {
			return err
		}
		s.AfterTasks = append(s.AfterTasks, item)
	}
	taskIDs := make([]int64, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	var err error
	s.AfterResourceGroups, err = captureResourceGroupsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.AfterAssetBindings, err = captureAssetBindingsForTasks(ctx, tx, taskIDs)
	if err != nil {
		return err
	}
	s.AfterSKUOrigins, err = captureSKUOrigins(ctx, tx, planningTaskIDs(m))
	if err != nil {
		return err
	}
	s.PlanningAfter, err = capturePlanningStates(ctx, tx, planningTaskIDs(m))
	if err != nil {
		return err
	}
	s.PlanningCreated = diffPlanningCreated(s.PlanningBefore, s.PlanningAfter)
	now := time.Now().UTC()
	s.AppliedAt = &now
	return nil
}

func diffPlanningCreated(before, after []planningStateSnapshot) []planningCreatedSnapshot {
	beforeByTask := map[int64]planningStateSnapshot{}
	for _, item := range before {
		beforeByTask[item.TaskID] = item
	}
	created := make([]planningCreatedSnapshot, 0, len(after))
	for _, item := range after {
		previous := beforeByTask[item.TaskID]
		detailBefore := make([]int64, 0, len(previous.Details))
		for _, detail := range previous.Details {
			detailBefore = append(detailBefore, detail.TaskSKUItemID)
		}
		entry := planningCreatedSnapshot{
			TaskID: item.TaskID, SettingsCreated: !previous.SettingsExists && item.SettingsExists,
			DetailIDs:        differenceInt64(planningDetailIDs(item.Details), detailBefore),
			RevisionIDs:      differenceInt64(item.RevisionIDs, previous.RevisionIDs),
			ImageRevisionIDs: differenceInt64(item.ImageRevisionIDs, previous.ImageRevisionIDs),
		}
		if entry.SettingsCreated || len(entry.DetailIDs) > 0 || len(entry.RevisionIDs) > 0 || len(entry.ImageRevisionIDs) > 0 {
			created = append(created, entry)
		}
	}
	return created
}

func planningDetailIDs(items []planningDetailSnapshot) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskSKUItemID)
	}
	return ids
}

func differenceInt64(after, before []int64) []int64 {
	seen := make(map[int64]struct{}, len(before))
	for _, id := range before {
		seen[id] = struct{}{}
	}
	result := []int64{}
	for _, id := range after {
		if _, exists := seen[id]; !exists {
			result = append(result, id)
		}
	}
	return result
}

func apply(ctx context.Context, db *sql.DB, database string, o options, m mappingFile) error {
	if err := validateMapping(m); err != nil {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "invalid reviewed mapping: %v", err)
	}
	blockers, err := queryCutoverBlockers(ctx, db, m)
	if err != nil {
		return err
	}
	if err := requireNoCutoverBlockers(blockers); err != nil {
		return err
	}
	if err := os.MkdirAll(o.SnapshotDir, 0o750); err != nil {
		return err
	}
	mappingSHA256, err := mappingDigest(m)
	if err != nil {
		return err
	}
	path := filepath.Join(o.SnapshotDir, "workflow-groups-snapshot.json")
	if _, err := os.Stat(path); err == nil {
		retry, err := recoverExistingApply(ctx, db, database, path, m)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockCutoverTargets(ctx, tx, m); err != nil {
		return err
	}
	if err := lockPreflightRows(ctx, tx); err != nil {
		return err
	}
	blockers, err = queryCutoverBlockers(ctx, tx, m)
	if err != nil {
		return err
	}
	if err := requireNoCutoverBlockers(blockers); err != nil {
		return err
	}
	snap, err := captureSnapshot(ctx, tx, database, m)
	if err != nil {
		return err
	}
	snap.MappingSHA256 = mappingSHA256
	snap.ApplyState = "prepared"
	if err := writeSnapshot(path, snap); err != nil {
		return err
	}
	if err := migrateStates(ctx, tx); err != nil {
		return err
	}
	for _, item := range m.Resources {
		id, revisionID, inserted, applied, err := applyResource(ctx, tx, item)
		if err != nil {
			return err
		}
		if inserted {
			snap.InsertedGroupIDs = append(snap.InsertedGroupIDs, id)
		}
		if applied {
			snap.AppliedRevisionIDs = append(snap.AppliedRevisionIDs, revisionID)
		}
	}
	for _, item := range m.Planning {
		inserted, err := applyPlanning(ctx, tx, item)
		if err != nil {
			return err
		}
		_ = inserted
	}
	if err := validateCutoverState(ctx, tx, m); err != nil {
		return err
	}
	if err := populateAfterSnapshot(ctx, tx, &snap, m); err != nil {
		return err
	}
	snap.ApplyState = "commit_pending"
	if err := writeSnapshot(path, snap); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	snap.ApplyState = "applied"
	if err := writeSnapshot(path, snap); err != nil {
		return fmt.Errorf("database apply committed but final manifest marker failed; preserve commit_pending manifest and recover before continuing: %w", err)
	}
	return nil
}

func lockPreflightRows(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`SELECT id FROM tasks ORDER BY id FOR UPDATE`,
		`SELECT id FROM org_departments ORDER BY id FOR UPDATE`,
		`SELECT id FROM org_teams ORDER BY id FOR UPDATE`,
		`SELECT id FROM users ORDER BY id FOR UPDATE`,
	}
	for _, query := range queries {
		if _, err := lockInt64Rows(ctx, tx, query); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT user_id,role FROM user_roles ORDER BY user_id,role FOR UPDATE`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var userID int64
		var role string
		if err := rows.Scan(&userID, &role); err != nil {
			rows.Close()
			return err
		}
	}
	return rows.Close()
}

func recoverExistingApply(ctx context.Context, db *sql.DB, database, path string, m mappingFile) (bool, error) {
	s, err := readSnapshot(path, database, m)
	if err != nil {
		return false, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := lockRollbackTargets(ctx, tx, s); err != nil {
		return false, err
	}
	beforeMatches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return false, err
	}
	afterMatches := false
	if s.AppliedAt != nil && s.AfterTasks != nil {
		afterMatches, err = snapshotStateMatches(ctx, tx, s, true)
		if err != nil {
			return false, err
		}
	}
	switch {
	case s.ApplyState == "applied" && afterMatches:
		return false, tx.Commit()
	case s.ApplyState == "commit_pending" && afterMatches:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		s.ApplyState = "applied"
		return false, writeSnapshot(path, s)
	case (s.ApplyState == "prepared" || s.ApplyState == "commit_pending" || s.ApplyState == "rolled_back") && beforeMatches:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		if err := removeSnapshot(path); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "apply recovery refused: manifest state %q does not match the current database before/after state", s.ApplyState)
	}
}

func captureSnapshot(ctx context.Context, q snapshotQueryer, database string, m mappingFile) (snapshot, error) {
	ids := map[int64]struct{}{}
	for _, r := range m.Resources {
		ids[r.TaskID] = struct{}{}
	}
	for _, p := range m.Planning {
		ids[p.TaskID] = struct{}{}
	}
	rows, err := q.QueryContext(ctx, `SELECT id, task_type, task_status, workflow_revision, current_handler_id FROM tasks WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview') OR task_type='purchase_task' ORDER BY id`)
	if err != nil {
		return snapshot{}, err
	}
	defer rows.Close()
	s := snapshot{
		Version: workflowGroupsSnapshotVersion, ToolVersion: workflowGroupsToolVersion,
		SchemaVersion: workflowGroupsSchemaVersion, Database: database, CreatedAt: time.Now().UTC(),
	}
	for rows.Next() {
		var t taskSnapshot
		var handlerID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.TaskType, &t.TaskStatus, &t.WorkflowRevision, &handlerID); err != nil {
			return s, err
		}
		if handlerID.Valid {
			value := handlerID.Int64
			t.CurrentHandlerID = &value
		}
		s.Tasks = append(s.Tasks, t)
		delete(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	for id := range ids {
		var t taskSnapshot
		var handlerID sql.NullInt64
		if err := q.QueryRowContext(ctx, `SELECT id, task_type, task_status, workflow_revision, current_handler_id FROM tasks WHERE id=?`, id).Scan(&t.ID, &t.TaskType, &t.TaskStatus, &t.WorkflowRevision, &handlerID); err != nil {
			return s, err
		}
		if handlerID.Valid {
			value := handlerID.Int64
			t.CurrentHandlerID = &value
		}
		s.Tasks = append(s.Tasks, t)
	}
	sort.Slice(s.Tasks, func(i, j int) bool { return s.Tasks[i].ID < s.Tasks[j].ID })
	for i := range s.Tasks {
		eventIDs, err := queryStringIDs(ctx, q, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id`, s.Tasks[i].ID)
		if err != nil {
			return s, err
		}
		s.Tasks[i].EventIDs = eventIDs
	}
	taskIDs := make([]int64, 0, len(s.Tasks))
	for _, task := range s.Tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	s.ResourceGroups, err = captureResourceGroupsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.AssetBindings, err = captureAssetBindingsForTasks(ctx, q, taskIDs)
	if err != nil {
		return s, err
	}
	s.SKUOrigins, err = captureSKUOrigins(ctx, q, planningTaskIDs(m))
	if err != nil {
		return s, err
	}
	s.PlanningBefore, err = capturePlanningStates(ctx, q, planningTaskIDs(m))
	if err != nil {
		return s, err
	}
	return s, nil
}

func migrateStates(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE tasks SET task_status=CASE
		WHEN task_status IN ('PendingAuditA','PendingAuditB','PendingCustomizationReview','PendingOutsourceReview','PendingEffectReview') THEN 'PendingAudit'
		WHEN task_status IN ('RejectedByAuditA','RejectedByAuditB','PendingCustomizationProduction','PendingEffectRevision','PendingOutsource','Outsourcing') THEN 'InProgress'
		WHEN task_status IN ('PendingWarehouseQC','PendingWarehouseReceive','PendingProductionTransfer','PendingClose') THEN 'Completed'
		ELSE task_status END,
		workflow_revision=workflow_revision+1
	WHERE task_status IN ('PendingAuditA','PendingAuditB','PendingCustomizationReview','PendingOutsourceReview','PendingEffectReview','RejectedByAuditA','RejectedByAuditB','PendingCustomizationProduction','PendingEffectRevision','PendingOutsource','Outsourcing','PendingWarehouseQC','PendingWarehouseReceive','PendingProductionTransfer','PendingClose')`)
	return err
}

func applyResource(ctx context.Context, tx *sql.Tx, m resourceMapping) (int64, int64, bool, bool, error) {
	var groupID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`, m.TaskID, m.ScopeKind, m.ScopeRefID).Scan(&groupID)
	inserted := false
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		var skuID, retouchID any
		if m.ScopeKind == "sku" {
			skuID = m.ScopeRefID
		}
		if m.ScopeKind == "retouch_requirement" {
			retouchID = m.ScopeRefID
		}
		res, insertErr := tx.ExecContext(ctx, `INSERT INTO task_asset_groups (task_id,scope_kind,task_sku_item_id,retouch_requirement_id,migration_incomplete,migration_issue) VALUES (?,?,?,?,1,'legacy resource revision pending cutover mapping')`, m.TaskID, m.ScopeKind, skuID, retouchID)
		if insertErr != nil {
			return 0, 0, false, false, insertErr
		}
		groupID, _ = res.LastInsertId()
		inserted = true
	}
	var taskType, taskStatus string
	if err := tx.QueryRowContext(ctx, `SELECT task_type, task_status FROM tasks WHERE id=? FOR UPDATE`, m.TaskID).Scan(&taskType, &taskStatus); err != nil {
		return 0, 0, inserted, false, err
	}
	var revisionCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_revisions WHERE group_id=?`, groupID).Scan(&revisionCount); err != nil {
		return 0, 0, inserted, false, err
	}
	if revisionCount > 0 {
		var incomplete bool
		if err := tx.QueryRowContext(ctx, `SELECT migration_incomplete FROM task_asset_groups WHERE id=?`, groupID).Scan(&incomplete); err != nil {
			return 0, 0, inserted, false, err
		}
		if incomplete {
			return 0, 0, inserted, false, fmt.Errorf("group %d has revisions but is still migration_incomplete; manual reconciliation required", groupID)
		}
		if err := verifyResourceMappingQuery(ctx, tx, groupID, m); err != nil {
			return 0, 0, inserted, false, err
		}
		return groupID, 0, inserted, false, nil
	}
	status := m.TargetStatus
	if status == "shell" {
		if taskStatus == "PendingAudit" || taskStatus == "Completed" {
			return 0, 0, inserted, false, fmt.Errorf("task %d in %s cannot be migrated as an empty shell", m.TaskID, taskStatus)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE task_asset_groups SET migration_incomplete=0,migration_issue='' WHERE id=?`, groupID); err != nil {
			return 0, 0, inserted, false, err
		}
		return groupID, 0, inserted, false, nil
	}
	if status != "draft" && status != "submitted" && status != "finalized" {
		return 0, 0, inserted, false, fmt.Errorf("invalid target_status %q", status)
	}
	if taskType != "retouch_task" && m.SourceAssetID == nil {
		return 0, 0, inserted, false, fmt.Errorf("task %d scope %s/%d has no confirmed source file", m.TaskID, m.ScopeKind, m.ScopeRefID)
	}
	if taskStatus == "Completed" && status != "finalized" {
		return 0, 0, inserted, false, fmt.Errorf("completed task %d requires finalized resource mappings", m.TaskID)
	}
	if taskStatus == "PendingAudit" && status != "submitted" {
		return 0, 0, inserted, false, fmt.Errorf("pending-audit task %d requires submitted resource mappings", m.TaskID)
	}
	assetIDs := append([]int64(nil), m.FinalAssetIDs...)
	if m.SourceAssetID != nil {
		assetIDs = append(assetIDs, *m.SourceAssetID)
	}
	for _, assetID := range assetIDs {
		var ownerTaskID int64
		if err := tx.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id=? FOR UPDATE`, assetID).Scan(&ownerTaskID); err != nil {
			return 0, 0, inserted, false, fmt.Errorf("task asset %d: %w", assetID, err)
		}
		if ownerTaskID != m.TaskID {
			return 0, 0, inserted, false, fmt.Errorf("task asset %d belongs to task %d, not task %d", assetID, ownerTaskID, m.TaskID)
		}
	}
	for _, referenceID := range m.ReferenceIDs {
		var ownerTaskID int64
		var skuItemID, retouchRequirementID sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs WHERE id=? FOR UPDATE`, referenceID).Scan(&ownerTaskID, &skuItemID, &retouchRequirementID); err != nil {
			return 0, 0, inserted, false, fmt.Errorf("reference file %d: %w", referenceID, err)
		}
		if ownerTaskID != m.TaskID || !referenceMatchesResourceScope(m.ScopeKind, m.ScopeRefID, skuItemID, retouchRequirementID) {
			return 0, 0, inserted, false, fmt.Errorf("reference file %d does not belong to the mapped task scope", referenceID)
		}
	}
	stage := "migration"
	res, err := tx.ExecContext(ctx, `INSERT INTO task_asset_group_revisions (group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at) VALUES (?,1,?,?,?,?,?,?,IF(? IN ('submitted','finalized'),NOW(),NULL),IF(?='finalized',NOW(),NULL))`, groupID, status, m.Mode, m.SourceAssetID, stage, m.CreatedBy, firstNonEmpty(m.Reason, "confirmed migration mapping"), status, status)
	if err != nil {
		return 0, 0, inserted, false, err
	}
	revisionID, _ := res.LastInsertId()
	for i, assetID := range m.FinalAssetIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO task_asset_group_revision_items (revision_id,task_asset_id,sort_order) VALUES (?,?,?)`, revisionID, assetID, i); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	for i, refID := range m.ReferenceIDs {
		if err = insertMigratedReferenceSnapshot(ctx, tx, revisionID, refID, i); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	pointer := "working_revision_id"
	if status == "finalized" {
		pointer = "finalized_revision_id"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE task_asset_groups SET `+pointer+`=?, lock_version=lock_version+1,migration_incomplete=0,migration_issue='' WHERE id=?`, revisionID, groupID); err != nil {
		return 0, 0, inserted, false, err
	}
	if m.SourceAssetID != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='source' WHERE id=?`, groupID, *m.SourceAssetID); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	for _, assetID := range m.FinalAssetIDs {
		if _, err = tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='final' WHERE id=?`, groupID, assetID); err != nil {
			return 0, 0, inserted, false, err
		}
	}
	return groupID, revisionID, inserted, true, nil
}

func insertMigratedReferenceSnapshot(ctx context.Context, tx *sql.Tx, revisionID, referenceID int64, sortOrder int) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revision_references
		  (revision_id,reference_file_ref_id,sort_order,ref_id_snapshot,file_name_snapshot,scope_snapshot)
		SELECT ?,rfr.id,?,rfr.ref_id,COALESCE(asr.file_name,''),
		       CASE
		         WHEN rfr.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:',rfr.retouch_requirement_id)
		         WHEN rfr.sku_item_id IS NOT NULL THEN CONCAT('sku:',rfr.sku_item_id)
		         ELSE 'task'
		       END
		FROM reference_file_refs rfr
		LEFT JOIN asset_storage_refs asr ON asr.ref_id=rfr.ref_id
		WHERE rfr.id=?`, revisionID, sortOrder, referenceID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return fmt.Errorf("reference file %d disappeared before snapshot insert", referenceID)
	}
	return nil
}

func verifyResourceMappingQuery(ctx context.Context, q snapshotQueryer, groupID int64, m resourceMapping) error {
	if m.TargetStatus == "shell" {
		return fmt.Errorf("group %d already has resource revisions and cannot match a shell mapping", groupID)
	}
	var revisionID sql.NullInt64
	column := "working_revision_id"
	if m.TargetStatus == "finalized" {
		column = "finalized_revision_id"
	}
	if err := q.QueryRowContext(ctx, `SELECT `+column+` FROM task_asset_groups WHERE id=?`, groupID).Scan(&revisionID); err != nil {
		return err
	}
	if !revisionID.Valid {
		return fmt.Errorf("group %d has revisions but no %s pointer", groupID, column)
	}
	var status, mode, sourceStage string
	var sourceID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT status,mode,source_task_asset_id,source_stage FROM task_asset_group_revisions WHERE id=? AND group_id=?`, revisionID.Int64, groupID).Scan(&status, &mode, &sourceID, &sourceStage); err != nil {
		return err
	}
	if status != m.TargetStatus || mode != m.Mode || nullableInt64Value(sourceID) != pointerInt64Value(m.SourceAssetID) {
		return fmt.Errorf("group %d existing revision does not match requested status/mode/source", groupID)
	}
	rows, err := q.QueryContext(ctx, `SELECT task_asset_id FROM task_asset_group_revision_items WHERE revision_id=? ORDER BY sort_order,id`, revisionID.Int64)
	if err != nil {
		return err
	}
	var finals []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		finals = append(finals, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !equalInt64Slices(finals, m.FinalAssetIDs) {
		return fmt.Errorf("group %d existing final-file order differs from mapping", groupID)
	}
	rows, err = q.QueryContext(ctx, `
		SELECT rr.reference_file_ref_id,rr.ref_id_snapshot,rr.file_name_snapshot,rr.scope_snapshot,
		       rfr.ref_id,COALESCE(asr.file_name,''),
		       CASE
		         WHEN rfr.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:',rfr.retouch_requirement_id)
		         WHEN rfr.sku_item_id IS NOT NULL THEN CONCAT('sku:',rfr.sku_item_id)
		         ELSE 'task'
		       END AS expected_scope_snapshot
		FROM task_asset_group_revision_references rr
		JOIN reference_file_refs rfr ON rfr.id=rr.reference_file_ref_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id=rfr.ref_id
		WHERE rr.revision_id=?
		ORDER BY rr.sort_order,rr.id`, revisionID.Int64)
	if err != nil {
		return err
	}
	var references []int64
	for rows.Next() {
		var id int64
		var refIDSnapshot, fileNameSnapshot, scopeSnapshot string
		var expectedRefID, expectedFileName, expectedScope string
		if err := rows.Scan(&id, &refIDSnapshot, &fileNameSnapshot, &scopeSnapshot, &expectedRefID, &expectedFileName, &expectedScope); err != nil {
			rows.Close()
			return err
		}
		if sourceStage == "migration" && (refIDSnapshot != expectedRefID || fileNameSnapshot != expectedFileName || scopeSnapshot != expectedScope) {
			rows.Close()
			return fmt.Errorf("group %d migrated reference %d has an invalid frozen snapshot", groupID, id)
		}
		references = append(references, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !equalInt64Slices(references, m.ReferenceIDs) {
		return fmt.Errorf("group %d existing references differ from mapping", groupID)
	}
	return nil
}

func nullableInt64Value(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

func pointerInt64Value(value *int64) int64 {
	if value != nil {
		return *value
	}
	return 0
}

func equalInt64Slices(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func applyPlanning(ctx context.Context, tx *sql.Tx, m planningMapping) (bool, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_planning_settings WHERE task_id=?`, m.TaskID).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		if err := verifyPlanningMappingQuery(ctx, tx, m); err != nil {
			return false, err
		}
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO task_planning_settings (task_id,erp_sync_mode,code_rule_revision_id,client_create_id,created_by) VALUES (?,'none',?,?,?)`, m.TaskID, m.CodeRuleRevisionID, fmt.Sprintf("migration-%d", m.TaskID), m.CreatedBy); err != nil {
		return false, err
	}
	for _, item := range m.Items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_planning_sku_details (task_sku_item_id) VALUES (?)`, item.TaskSKUItemID); err != nil {
			return false, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO task_planning_sku_revisions (task_sku_item_id,version_no,description_spec,quantity,target_price,currency,note,reference_url,erp_product_i_id,erp_product_name,reason,created_by) VALUES (?,1,?,?,?,'CNY',?,?,?,?,'confirmed legacy planning migration',?)`, item.TaskSKUItemID, item.DescriptionSpec, item.Quantity, item.TargetPrice, item.Note, item.ReferenceURL, item.ERPProductIID, item.ERPProductName, m.CreatedBy)
		if err != nil {
			return false, err
		}
		revisionID, _ := res.LastInsertId()
		if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=? WHERE task_sku_item_id=?`, revisionID, item.TaskSKUItemID); err != nil {
			return false, err
		}
		if item.ImageStorageRef != "" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO task_planning_sku_revision_images (revision_id,storage_ref_id) VALUES (?,?)`, revisionID, item.ImageStorageRef); err != nil {
				return false, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET task_type='sku_planning',task_status='Completed',workflow_revision=workflow_revision+1 WHERE id=?`, m.TaskID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE task_sku_items SET sku_origin='legacy_migration' WHERE task_id=?`, m.TaskID); err != nil {
		return false, err
	}
	return true, nil
}

func verifyPlanningMappingQuery(ctx context.Context, q snapshotQueryer, m planningMapping) error {
	var taskType string
	var ruleRevisionID, createdBy int64
	if err := q.QueryRowContext(ctx, `
		SELECT t.task_type,s.code_rule_revision_id,s.created_by
		FROM task_planning_settings s JOIN tasks t ON t.id=s.task_id
		WHERE s.task_id=?`, m.TaskID).Scan(&taskType, &ruleRevisionID, &createdBy); err != nil {
		return err
	}
	if taskType != "sku_planning" || ruleRevisionID != m.CodeRuleRevisionID || createdBy != m.CreatedBy {
		return fmt.Errorf("planning task %d settings differ from the confirmed mapping", m.TaskID)
	}
	var itemCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_sku_items WHERE task_id=?`, m.TaskID).Scan(&itemCount); err != nil {
		return err
	}
	if itemCount != len(m.Items) {
		return fmt.Errorf("planning task %d has %d SKU rows but mapping has %d", m.TaskID, itemCount, len(m.Items))
	}
	for _, item := range m.Items {
		var description string
		var quantity int64
		var targetPrice sql.NullString
		var note, referenceURL, erpIID, erpName string
		var imageRef sql.NullString
		err := q.QueryRowContext(ctx, `
			SELECT r.description_spec,r.quantity,CAST(r.target_price AS CHAR),r.note,r.reference_url,
			       r.erp_product_i_id,r.erp_product_name,img.storage_ref_id
			FROM task_sku_items si
			JOIN task_planning_sku_details d ON d.task_sku_item_id=si.id AND d.current_revision_id IS NOT NULL
			JOIN task_planning_sku_revisions r ON r.id=d.current_revision_id AND r.task_sku_item_id=d.task_sku_item_id
			LEFT JOIN task_planning_sku_revision_images img ON img.revision_id=r.id
			WHERE si.task_id=? AND si.id=?`, m.TaskID, item.TaskSKUItemID).
			Scan(&description, &quantity, &targetPrice, &note, &referenceURL, &erpIID, &erpName, &imageRef)
		if err != nil {
			return fmt.Errorf("planning task %d item %d is incomplete: %w", m.TaskID, item.TaskSKUItemID, err)
		}
		if description != item.DescriptionSpec || quantity != item.Quantity || nullableStringValue(targetPrice) != stringPointerValue(item.TargetPrice) || note != item.Note || referenceURL != item.ReferenceURL || erpIID != item.ERPProductIID || erpName != item.ERPProductName || nullableStringValue(imageRef) != item.ImageStorageRef {
			return fmt.Errorf("planning task %d item %d differs from the confirmed mapping", m.TaskID, item.TaskSKUItemID)
		}
	}
	return nil
}

func validateCutoverState(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	blockers, err := queryCutoverBlockers(ctx, tx, m)
	if err != nil {
		return err
	}
	if err := requireNoCutoverBlockers(blockers); err != nil {
		return err
	}
	checks := []struct {
		name  string
		query string
	}{
		{name: "resource groups still migration-incomplete", query: `SELECT COUNT(*) FROM task_asset_groups WHERE migration_incomplete=1`},
		{name: "legacy purchase_task rows remain", query: `SELECT COUNT(*) FROM tasks WHERE task_type='purchase_task'`},
		{name: "retired active task statuses remain", query: `SELECT COUNT(*) FROM tasks WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview','RejectedByWarehouse')`},
		{name: "planning settings/detail/current revision parity failures", query: `
			SELECT COUNT(*) FROM tasks t
			WHERE t.task_type='sku_planning' AND (
			  NOT EXISTS (SELECT 1 FROM task_planning_settings s WHERE s.task_id=t.id)
			  OR EXISTS (
			    SELECT 1 FROM task_sku_items si
			    LEFT JOIN task_planning_sku_details d ON d.task_sku_item_id=si.id
			    LEFT JOIN task_planning_sku_revisions r ON r.id=d.current_revision_id AND r.task_sku_item_id=d.task_sku_item_id
			    WHERE si.task_id=t.id AND (d.task_sku_item_id IS NULL OR d.current_revision_id IS NULL OR r.id IS NULL)
			  )
			  OR NOT EXISTS (SELECT 1 FROM task_sku_items si WHERE si.task_id=t.id)
			)`},
		{name: "sku_planning tasks still own design resource groups", query: `SELECT COUNT(*) FROM tasks t JOIN task_asset_groups g ON g.task_id=t.id WHERE t.task_type='sku_planning'`},
	}
	for _, check := range checks {
		var count int64
		if err := tx.QueryRowContext(ctx, check.query).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("cutover blocked: %s (%d)", check.name, count)
		}
	}
	return nil
}

func nullableStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func stringPointerValue(value *string) string {
	if value != nil {
		return *value
	}
	return ""
}

func rollback(ctx context.Context, db *sql.DB, database string, o options, m mappingFile) error {
	path := filepath.Join(o.SnapshotDir, "workflow-groups-snapshot.json")
	s, err := readSnapshot(path, database, m)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockRollbackTargets(ctx, tx, s); err != nil {
		return err
	}
	beforeMatches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return err
	}
	afterMatches := false
	if s.AppliedAt != nil && s.AfterTasks != nil {
		afterMatches, err = snapshotStateMatches(ctx, tx, s, true)
		if err != nil {
			return err
		}
	}
	if s.ApplyState == "rolled_back" {
		if !beforeMatches {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: manifest is rolled_back but database no longer matches the before state")
		}
		return tx.Commit()
	}
	if !afterMatches {
		if (s.ApplyState == "prepared" || s.ApplyState == "commit_pending") && beforeMatches {
			if err := tx.Commit(); err != nil {
				return err
			}
			s.ApplyState = "rolled_back"
			return writeSnapshot(path, s)
		}
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: current database differs from both the recorded before and apply-after states; preserve forward writes and investigate")
	}
	if s.ApplyState != "applied" && s.ApplyState != "commit_pending" {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback refused: unsupported manifest apply_state %q", s.ApplyState)
	}
	for _, created := range s.PlanningCreated {
		beforeDetails := map[int64]planningDetailSnapshot{}
		for _, state := range s.PlanningBefore {
			if state.TaskID != created.TaskID {
				continue
			}
			for _, detail := range state.Details {
				beforeDetails[detail.TaskSKUItemID] = detail
			}
		}
		for _, detailID := range created.DetailIDs {
			if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=NULL WHERE task_sku_item_id=?`, detailID); err != nil {
				return err
			}
		}
		for detailID, detail := range beforeDetails {
			if _, err = tx.ExecContext(ctx, `UPDATE task_planning_sku_details SET current_revision_id=?,lock_version=? WHERE task_sku_item_id=?`, nullableInt64Pointer(detail.CurrentRevisionID), detail.LockVersion, detailID); err != nil {
				return err
			}
		}
		for _, revisionID := range created.ImageRevisionIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_revision_images WHERE revision_id=?`, revisionID); err != nil {
				return err
			}
		}
		for _, revisionID := range created.RevisionIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_revisions WHERE id=?`, revisionID); err != nil {
				return err
			}
		}
		for _, detailID := range created.DetailIDs {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_sku_details WHERE task_sku_item_id=?`, detailID); err != nil {
				return err
			}
		}
		if created.SettingsCreated {
			if _, err = tx.ExecContext(ctx, `DELETE FROM task_planning_settings WHERE task_id=?`, created.TaskID); err != nil {
				return err
			}
		}
	}
	for _, origin := range s.SKUOrigins {
		if _, err = tx.ExecContext(ctx, `UPDATE task_sku_items SET sku_origin=? WHERE id=?`, nullableStringPointer(origin.Origin), origin.ID); err != nil {
			return err
		}
	}
	for _, asset := range s.AssetBindings {
		if _, err = tx.ExecContext(ctx, `
			UPDATE task_assets
			SET binding_state=?,bound_group_id=?,bound_role=?,
			    staged_task_sku_item_id=?,staged_retouch_requirement_id=?,staged_role=?,staged_by=?,upload_session_id=?,staged_expires_at=?,
			    access_revoked_at=?,access_revoked_reason=?,object_deleted_at=?
			WHERE id=?`,
			asset.BindingState, nullableInt64Pointer(asset.BoundGroupID), nullableStringPointer(asset.BoundRole),
			nullableInt64Pointer(asset.StagedTaskSKUItemID), nullableInt64Pointer(asset.StagedRetouchRequirementID), nullableStringPointer(asset.StagedRole), nullableInt64Pointer(asset.StagedBy), nullableStringPointer(asset.UploadSessionID), nullableTimePointer(asset.StagedExpiresAt),
			nullableTimePointer(asset.AccessRevokedAt), asset.AccessRevokedReason, nullableTimePointer(asset.ObjectDeletedAt), asset.ID); err != nil {
			return err
		}
	}
	for _, group := range s.ResourceGroups {
		if _, err = tx.ExecContext(ctx, `
			UPDATE task_asset_groups
			SET working_revision_id=?,finalized_revision_id=?,lock_version=?,migration_incomplete=?,migration_issue=?
			WHERE id=?`, nullableInt64Pointer(group.WorkingRevisionID), nullableInt64Pointer(group.FinalizedRevisionID), group.LockVersion, group.MigrationIncomplete, group.MigrationIssue, group.ID); err != nil {
			return err
		}
	}
	for _, groupID := range s.InsertedGroupIDs {
		if _, err = tx.ExecContext(ctx, `UPDATE task_asset_groups SET working_revision_id=NULL,finalized_revision_id=NULL WHERE id=?`, groupID); err != nil {
			return err
		}
	}
	for _, revisionID := range s.AppliedRevisionIDs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revision_references WHERE revision_id=?`, revisionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revision_items WHERE revision_id=?`, revisionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_group_revisions WHERE id=?`, revisionID); err != nil {
			return err
		}
	}
	for _, groupID := range s.InsertedGroupIDs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM task_asset_groups WHERE id=?`, groupID); err != nil {
			return err
		}
	}
	for _, t := range s.Tasks {
		if _, err = tx.ExecContext(ctx, `UPDATE tasks SET task_type=?,task_status=?,workflow_revision=?,current_handler_id=? WHERE id=?`, t.TaskType, t.TaskStatus, t.WorkflowRevision, nullableInt64Pointer(t.CurrentHandlerID), t.ID); err != nil {
			return err
		}
	}
	matches, err := snapshotStateMatches(ctx, tx, s, false)
	if err != nil {
		return err
	}
	if !matches {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "rollback verification failed before commit")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.ApplyState = "rolled_back"
	return writeSnapshot(path, s)
}

func lockRollbackTargets(ctx context.Context, tx *sql.Tx, s snapshot) error {
	taskIDs := make([]int64, 0, len(s.Tasks)+len(s.AfterTasks))
	for _, item := range s.Tasks {
		taskIDs = append(taskIDs, item.ID)
	}
	for _, item := range s.AfterTasks {
		taskIDs = append(taskIDs, item.ID)
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM tasks WHERE id=? FOR UPDATE`, taskID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_sku_items WHERE task_id=? ORDER BY id FOR UPDATE`, taskID); err != nil {
			return err
		}
	}
	groupIDs := []int64{}
	for _, item := range s.ResourceGroups {
		groupIDs = append(groupIDs, item.ID)
	}
	for _, item := range s.AfterResourceGroups {
		groupIDs = append(groupIDs, item.ID)
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	for _, groupID := range uniqueSortedInt64(groupIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE id=? FOR UPDATE`, groupID); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_group_revisions WHERE group_id=? ORDER BY id FOR UPDATE`, groupID); err != nil {
			return err
		}
	}
	assetIDs := []int64{}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		ids, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE task_id=? ORDER BY id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		assetIDs = append(assetIDs, ids...)
	}
	for _, item := range s.AssetBindings {
		assetIDs = append(assetIDs, item.ID)
	}
	for _, item := range s.AfterAssetBindings {
		assetIDs = append(assetIDs, item.ID)
	}
	for _, assetID := range uniqueSortedInt64(assetIDs) {
		if _, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_assets WHERE id=? FOR UPDATE`, assetID); err != nil {
			return err
		}
	}
	planningTaskIDs := []int64{}
	for _, item := range s.PlanningBefore {
		planningTaskIDs = append(planningTaskIDs, item.TaskID)
	}
	for _, item := range s.PlanningAfter {
		planningTaskIDs = append(planningTaskIDs, item.TaskID)
	}
	for _, taskID := range uniqueSortedInt64(planningTaskIDs) {
		queries := []string{
			`SELECT task_id FROM task_planning_settings WHERE task_id=? FOR UPDATE`,
			`SELECT d.task_sku_item_id FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id=? ORDER BY d.task_sku_item_id FOR UPDATE`,
			`SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY r.id FOR UPDATE`,
			`SELECT i.revision_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id=? ORDER BY i.revision_id FOR UPDATE`,
		}
		for _, query := range queries {
			if _, err := lockInt64Rows(ctx, tx, query, taskID); err != nil {
				return err
			}
		}
	}
	for _, taskID := range uniqueSortedInt64(taskIDs) {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_event_logs WHERE task_id=? ORDER BY sequence,id FOR UPDATE`, taskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func snapshotStateMatches(ctx context.Context, q snapshotQueryer, s snapshot, after bool) (bool, error) {
	tasks := s.Tasks
	groups := s.ResourceGroups
	assets := s.AssetBindings
	origins := s.SKUOrigins
	planning := s.PlanningBefore
	if after {
		tasks = s.AfterTasks
		groups = s.AfterResourceGroups
		assets = s.AfterAssetBindings
		origins = s.AfterSKUOrigins
		planning = s.PlanningAfter
	}
	for _, expected := range tasks {
		actual, err := loadTaskSnapshot(ctx, q, expected.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !reflect.DeepEqual(actual, expected) {
			return false, nil
		}
	}
	taskIDsForGroups := make([]int64, 0, len(tasks))
	for _, item := range tasks {
		taskIDsForGroups = append(taskIDsForGroups, item.ID)
	}
	actualGroups, err := captureResourceGroupsForTasks(ctx, q, taskIDsForGroups)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actualGroups, groups) {
		return false, nil
	}
	actualAssets, err := captureAssetBindingsForTasks(ctx, q, taskIDsForGroups)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actualAssets, assets) {
		return false, nil
	}
	taskIDs := make([]int64, 0, len(planning))
	for _, item := range planning {
		taskIDs = append(taskIDs, item.TaskID)
	}
	actualOrigins, err := captureSKUOrigins(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actualOrigins, origins) {
		return false, nil
	}
	actualPlanning, err := capturePlanningStates(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(actualPlanning, planning), nil
}

func nullableInt64Pointer(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStringPointer(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTimePointer(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func mappingDigest(m mappingFile) (string, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func snapshotDigest(s snapshot) (string, error) {
	s.IntegritySHA256 = ""
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func readSnapshot(path, database string, m mappingFile) (snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return snapshot{}, err
	}
	var s snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return snapshot{}, err
	}
	if s.Version != workflowGroupsSnapshotVersion || s.ToolVersion != workflowGroupsToolVersion || s.SchemaVersion != workflowGroupsSchemaVersion {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot tool/schema version mismatch")
	}
	if s.Database != database {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot database %q does not match %q", s.Database, database)
	}
	expectedMappingSHA256, err := mappingDigest(m)
	if err != nil {
		return snapshot{}, err
	}
	if s.MappingSHA256 != expectedMappingSHA256 {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot mapping digest does not match the reviewed mapping file")
	}
	expectedIntegrity, err := snapshotDigest(s)
	if err != nil {
		return snapshot{}, err
	}
	if s.IntegritySHA256 == "" || s.IntegritySHA256 != expectedIntegrity {
		return snapshot{}, v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "snapshot integrity verification failed")
	}
	return s, nil
}

func writeSnapshot(path string, s snapshot) error {
	digest, err := snapshotDigest(s)
	if err != nil {
		return err
	}
	s.IntegritySHA256 = digest
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".workflow-groups-snapshot-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}

func removeSnapshot(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dirHandle, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}
func writeJSONReport(v any, path string) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if path != "" {
		return os.WriteFile(path, raw, 0o600)
	}
	_, err = os.Stdout.Write(raw)
	return err
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
