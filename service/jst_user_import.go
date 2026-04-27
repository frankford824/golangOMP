package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// JSTUserImportPreviewResult 导入预览结果。
type JSTUserImportPreviewResult struct {
	FetchedCount      int                        `json:"fetched_count"`
	MatchedCount      int                        `json:"matched_count"`
	ToCreateCount     int                        `json:"to_create_count"`
	ToUpdateCount     int                        `json:"to_update_count"`
	OrgMappingMissed  int                        `json:"org_mapping_missed"`
	RoleMappingMissed int                        `json:"role_mapping_missed"`
	Risks             []string                   `json:"risks,omitempty"`
	PreviewItems      []JSTUserImportPreviewItem `json:"preview_items,omitempty"`
}

// JSTUserImportPreviewItem 单条预览项。
type JSTUserImportPreviewItem struct {
	JstUID      int64    `json:"jst_u_id"`
	Name        string   `json:"name"`
	LoginID     string   `json:"login_id,omitempty"`
	Action      string   `json:"action"` // "create" | "update"
	LocalUserID *int64   `json:"local_user_id,omitempty"`
	Department  string   `json:"department,omitempty"`
	Team        string   `json:"team,omitempty"`
	MappedRoles []string `json:"mapped_roles,omitempty"`
}

// JSTUserImportResult 实际导入结果。
type JSTUserImportResult struct {
	FetchedCount       int      `json:"fetched_count"`
	CreatedCount       int      `json:"created_count"`
	UpdatedCount       int      `json:"updated_count"`
	DisabledCount      int      `json:"disabled_count"`
	TraceUpdatedCount  int      `json:"trace_updated_count"`
	RolesWritten       bool     `json:"roles_written"`
	MappingFailedCount int      `json:"mapping_failed_count"`
	Errors             []string `json:"errors,omitempty"`
}

// JSTUserImportOptions 导入选项。
type JSTUserImportOptions struct {
	WriteRoles  bool                   `json:"write_roles"` // 默认 false，不覆盖本地角色
	OrgMapping  map[string]string      `json:"-"`           // ug_name -> "department:team"
	RoleMapping map[string]domain.Role `json:"-"`           // jst_role -> domain.Role
}

// JSTUserImportService JST 用户导入服务（预埋能力，不接管鉴权）。
type JSTUserImportService interface {
	ImportPreview(ctx context.Context, filter domain.JSTUserListFilter, opts JSTUserImportOptions) (*JSTUserImportPreviewResult, *domain.AppError)
	Import(ctx context.Context, filter domain.JSTUserListFilter, opts JSTUserImportOptions, dryRun bool) (*JSTUserImportResult, *domain.AppError)
}

type jstUserImportService struct {
	erpBridgeSvc ERPBridgeService
	userRepo     repo.UserRepo
	txRunner     repo.TxRunner
	authSettings domain.AuthSettings
}

// NewJSTUserImportService 创建 JST 用户导入服务。
func NewJSTUserImportService(erpBridgeSvc ERPBridgeService, userRepo repo.UserRepo, txRunner repo.TxRunner, authSettings domain.AuthSettings) JSTUserImportService {
	return &jstUserImportService{
		erpBridgeSvc: erpBridgeSvc,
		userRepo:     userRepo,
		txRunner:     txRunner,
		authSettings: authSettings,
	}
}

func (s *jstUserImportService) ImportPreview(ctx context.Context, filter domain.JSTUserListFilter, opts JSTUserImportOptions) (*JSTUserImportPreviewResult, *domain.AppError) {
	resp, appErr := s.erpBridgeSvc.ListJSTUsers(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	if resp == nil || len(resp.Datas) == 0 {
		return &JSTUserImportPreviewResult{FetchedCount: 0}, nil
	}
	result := &JSTUserImportPreviewResult{
		FetchedCount: len(resp.Datas),
		PreviewItems: make([]JSTUserImportPreviewItem, 0, len(resp.Datas)),
	}
	for _, jst := range resp.Datas {
		item := JSTUserImportPreviewItem{JstUID: jst.UID, Name: jst.Name, LoginID: jst.LoginID}
		dept, team := mapJSTOrgToLocal(jst.UGNames, opts.OrgMapping)
		profile := s.resolveImportedUserProfile(jst, dept, team)
		matchedConfigured := s.hasConfiguredAssignment(jst)
		dept = string(profile.Department)
		team = profile.Team
		item.Department = dept
		item.Team = team
		if len(jst.UGNames) > 0 && dept == string(domain.DepartmentUnassigned) && team == "未分配池" {
			result.OrgMappingMissed++
		}
		roles := mapJSTRolesToLocal(jst.Roles, jst.RoleIDs, opts.RoleMapping)
		if matchedConfigured {
			roles = profile.Roles
		}
		for _, r := range domain.NormalizeRoleValues(roles) {
			item.MappedRoles = append(item.MappedRoles, string(r))
		}
		if (jst.Roles != "" || jst.RoleIDs != "") && len(roles) == 0 && opts.WriteRoles {
			result.RoleMappingMissed++
		}
		existing := s.findExistingUser(ctx, jst)
		if existing != nil {
			item.Action = "update"
			item.LocalUserID = &existing.ID
			result.MatchedCount++
			result.ToUpdateCount++
		} else {
			item.Action = "create"
			result.ToCreateCount++
		}
		result.PreviewItems = append(result.PreviewItems, item)
	}
	if result.ToCreateCount > 0 {
		result.Risks = append(result.Risks, "new_users_will_be_disabled_no_password")
	}
	if opts.WriteRoles {
		result.Risks = append(result.Risks, "roles_will_be_written_to_user_roles")
	}
	return result, nil
}

func (s *jstUserImportService) Import(ctx context.Context, filter domain.JSTUserListFilter, opts JSTUserImportOptions, dryRun bool) (*JSTUserImportResult, *domain.AppError) {
	resp, appErr := s.erpBridgeSvc.ListJSTUsers(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	if resp == nil || len(resp.Datas) == 0 {
		return &JSTUserImportResult{FetchedCount: 0}, nil
	}
	result := &JSTUserImportResult{FetchedCount: len(resp.Datas)}
	if dryRun {
		return result, nil
	}
	for _, jst := range resp.Datas {
		if err := s.importOne(ctx, jst, opts, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("u_id=%d: %v", jst.UID, err))
			result.MappingFailedCount++
		}
	}
	return result, nil
}

func (s *jstUserImportService) importOne(ctx context.Context, jst *domain.JSTUser, opts JSTUserImportOptions, result *JSTUserImportResult) error {
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		existing := s.findExistingUser(ctx, jst)
		dept, team := mapJSTOrgToLocal(jst.UGNames, opts.OrgMapping)
		profile := s.resolveImportedUserProfile(jst, dept, team)
		matchedConfigured := s.hasConfiguredAssignment(jst)
		dept = string(profile.Department)
		team = profile.Team
		status := domain.UserStatusActive
		if !jst.Enabled {
			status = domain.UserStatusDisabled
			result.DisabledCount++
		}
		if profile.Status.Valid() {
			status = profile.Status
		}
		rawSnapshot, _ := json.Marshal(jst)
		jstUID := jst.UID
		var lastLoginAt *time.Time
		if jst.LastLoginTime != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", jst.LastLoginTime); err == nil {
				lastLoginAt = &t
			}
		}
		if existing != nil {
			if err := s.userRepo.UpdateJstFields(ctx, tx, existing.ID, jst.Name, string(status), dept, team, profile.ManagedDepartments, profile.ManagedTeams, string(rawSnapshot), &jstUID, lastLoginAt); err != nil {
				return err
			}
			result.UpdatedCount++
			result.TraceUpdatedCount++
			roles := profile.Roles
			if len(roles) == 0 {
				roles = mapJSTRolesToLocal(jst.Roles, jst.RoleIDs, opts.RoleMapping)
			}
			if matchedConfigured || opts.WriteRoles {
				if len(roles) > 0 {
					if err := s.userRepo.ReplaceRoles(ctx, tx, existing.ID, roles); err != nil {
						return err
					}
					result.RolesWritten = true
				}
			}
			return nil
		}
		username := jst.LoginID
		if username == "" {
			username = fmt.Sprintf("jst_%d", jst.UID)
		}
		if existingU, _ := s.userRepo.GetByUsername(ctx, username); existingU != nil {
			return fmt.Errorf("username %s already exists", username)
		}
		hash, err := randomPasswordHash()
		if err != nil {
			return err
		}
		user := &domain.User{
			Username:           username,
			DisplayName:        jst.Name,
			Department:         domain.Department(dept),
			Team:               team,
			ManagedDepartments: append([]string{}, profile.ManagedDepartments...),
			ManagedTeams:       append([]string{}, profile.ManagedTeams...),
			Status:             domain.UserStatusDisabled,
			PasswordHash:       hash,
			JstUID:             &jstUID,
			JstRawSnapshotJSON: string(rawSnapshot),
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
		}
		id, err := s.userRepo.Create(ctx, tx, user)
		if err != nil {
			return err
		}
		_ = id
		result.CreatedCount++
		result.TraceUpdatedCount++
		roles := profile.Roles
		if len(roles) == 0 {
			roles = mapJSTRolesToLocal(jst.Roles, jst.RoleIDs, opts.RoleMapping)
		}
		if matchedConfigured || opts.WriteRoles || len(profile.Roles) > 0 {
			if len(roles) > 0 {
				if err := s.userRepo.ReplaceRoles(ctx, tx, id, roles); err != nil {
					return err
				}
				result.RolesWritten = true
			}
		}
		return nil
	})
}

func (s *jstUserImportService) findExistingUser(ctx context.Context, jst *domain.JSTUser) *domain.User {
	if u, _ := s.userRepo.GetByJstUID(ctx, jst.UID); u != nil {
		return u
	}
	if jst.LoginID != "" {
		if u, _ := s.userRepo.GetByUsername(ctx, jst.LoginID); u != nil {
			return u
		}
	}
	return nil
}

func mapJSTOrgToLocal(ugNames []string, mapping map[string]string) (department, team string) {
	if mapping == nil || len(ugNames) == 0 {
		return "", ""
	}
	for _, ug := range ugNames {
		ug = strings.TrimSpace(ug)
		if ug == "" {
			continue
		}
		if v, ok := mapping[ug]; ok {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) >= 1 {
				department = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				team = strings.TrimSpace(parts[1])
			}
			return
		}
	}
	return "", ""
}

func mapJSTRolesToLocal(rolesStr, roleIDsStr string, mapping map[string]domain.Role) []domain.Role {
	if mapping == nil {
		return nil
	}
	var result []domain.Role
	for _, r := range strings.Split(rolesStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if role, ok := mapping[r]; ok {
			result = append(result, role)
		}
	}
	for _, r := range strings.Split(roleIDsStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if role, ok := mapping[r]; ok {
			result = append(result, role)
		}
	}
	return domain.NormalizeRoleValues(result)
}

func randomPasswordHash() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *jstUserImportService) resolveImportedUserProfile(jst *domain.JSTUser, department, team string) domain.ConfiguredUserAssignment {
	if jst != nil {
		loginID := strings.TrimSpace(jst.LoginID)
		name := strings.TrimSpace(jst.Name)
		for _, entry := range s.authSettings.ConfiguredAssignments {
			if entry.Username != "" && strings.EqualFold(strings.TrimSpace(entry.Username), loginID) {
				return normalizeConfiguredAssignment(entry)
			}
			if entry.DisplayName != "" && strings.TrimSpace(entry.DisplayName) == name {
				return normalizeConfiguredAssignment(entry)
			}
		}
	}
	if strings.TrimSpace(department) != "" && strings.TrimSpace(team) != "" {
		return domain.ConfiguredUserAssignment{
			Department: domain.Department(department),
			Team:       team,
			Roles:      []domain.Role{domain.RoleMember},
		}
	}
	return domain.ConfiguredUserAssignment{
		Department: domain.DepartmentUnassigned,
		Team:       "未分配池",
		Roles:      []domain.Role{domain.RoleMember},
	}
}

func normalizeConfiguredAssignment(entry domain.ConfiguredUserAssignment) domain.ConfiguredUserAssignment {
	entry.ManagedDepartments = append([]string{}, entry.ManagedDepartments...)
	entry.ManagedTeams = append([]string{}, entry.ManagedTeams...)
	entry.Roles = domain.NormalizeRoleValues(entry.Roles)
	if len(entry.Roles) == 0 {
		entry.Roles = []domain.Role{domain.RoleMember}
	}
	if entry.Status == "" {
		entry.Status = domain.UserStatusActive
	}
	return entry
}

func (s *jstUserImportService) hasConfiguredAssignment(jst *domain.JSTUser) bool {
	if jst == nil {
		return false
	}
	loginID := strings.TrimSpace(jst.LoginID)
	name := strings.TrimSpace(jst.Name)
	for _, entry := range s.authSettings.ConfiguredAssignments {
		if entry.Username != "" && strings.EqualFold(strings.TrimSpace(entry.Username), loginID) {
			return true
		}
		if entry.DisplayName != "" && strings.TrimSpace(entry.DisplayName) == name {
			return true
		}
	}
	return false
}
