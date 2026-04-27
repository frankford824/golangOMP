//go:build integration

package handler

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	"workflow/testsupport/r35"
)

type saBUserFixture struct {
	ID                 int64
	Username           string
	DisplayName        string
	Department         string
	Team               string
	Mobile             string
	Email              string
	Password           string
	Status             string
	Roles              []domain.Role
	ManagedDepartments []string
	ManagedTeams       []string
}

type saBEnvelope struct {
	Data       json.RawMessage `json:"data"`
	Pagination json.RawMessage `json:"pagination"`
	Error      struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details"`
	} `json:"error"`
}

func TestSABI1_GetMe_ReturnsWorkflowUser_WithAvatarNull(t *testing.T) {
	db, svc := saBOpenHandlerTestDB(t)
	userID := int64(30001)
	saBCleanupUsers(t, db, userID)
	defer saBCleanupUsers(t, db, userID)

	saBInsertUser(t, db, saBUserFixture{
		ID:          userID,
		Username:    "sab_i1_me",
		DisplayName: "SA-B I1 Me",
		Department:  string(domain.DepartmentOperations),
		Team:        "淘系一组",
		Password:    "ChangeMeAdmin123",
		Roles:       []domain.Role{domain.RoleMember},
	})
	token := saBCreateSession(t, db, userID, "sab-i1-token")

	router := saBAuthRouter(svc)
	authH := NewAuthHandler(svc)
	router.GET("/v1/me", authH.GetMe)

	rec := saBPerformJSON(router, http.MethodGet, "/v1/me", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Data struct {
			ID          int64   `json:"id"`
			Username    string  `json:"username"`
			DisplayName string  `json:"display_name"`
			Department  string  `json:"department"`
			Team        string  `json:"team"`
			Mobile      string  `json:"mobile"`
			Avatar      *string `json:"avatar"`
		} `json:"data"`
	}
	saBDecode(t, rec, &out)
	if out.Data.ID != userID || out.Data.Username != "sab_i1_me" || out.Data.DisplayName != "SA-B I1 Me" || out.Data.Department != string(domain.DepartmentOperations) || out.Data.Team != "淘系一组" || out.Data.Mobile == "" || out.Data.Avatar != nil {
		t.Fatalf("GET /v1/me data = %+v, want workflow user with nil avatar", out.Data)
	}
}

func saBOpenHandlerTestDB(t *testing.T) (*sql.DB, service.IdentityService) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	wrapped := mysqlrepo.New(db)
	userRepo := mysqlrepo.NewUserRepo(wrapped)
	sessionRepo := mysqlrepo.NewUserSessionRepo(wrapped)
	logRepo := mysqlrepo.NewPermissionLogRepo(wrapped)
	return db, service.NewIdentityService(userRepo, sessionRepo, logRepo, wrapped)
}

func saBCleanupUsers(t *testing.T, db *sql.DB, userIDs ...int64) {
	t.Helper()
	if len(userIDs) == 0 {
		return
	}
	args := make([]interface{}, 0, len(userIDs))
	placeholders := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id < 30000 {
			t.Fatalf("SA-B fixture user id %d is below 30000", id)
		}
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}
	in := strings.Join(placeholders, ",")
	_, _ = db.Exec(`DELETE FROM permission_logs WHERE actor_id IN (`+in+`) OR target_user_id IN (`+in+`)`, append(args, args...)...)
	_, _ = db.Exec(`DELETE FROM org_move_requests WHERE user_id IN (`+in+`) OR requested_by IN (`+in+`) OR resolved_by IN (`+in+`)`, append(append(args, args...), args...)...)
	_, _ = db.Exec(`DELETE FROM user_sessions WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM user_roles WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM users WHERE id IN (`+in+`)`, args...)
}

func saBInsertUser(t *testing.T, db *sql.DB, f saBUserFixture) {
	t.Helper()
	if f.ID < 30000 {
		t.Fatalf("SA-B fixture user id %d is below 30000", f.ID)
	}
	if f.Username == "" {
		f.Username = fmt.Sprintf("sab_user_%d", f.ID)
	}
	if f.DisplayName == "" {
		f.DisplayName = f.Username
	}
	if f.Department == "" {
		f.Department = string(domain.DepartmentOperations)
	}
	if f.Team == "" {
		f.Team = "淘系一组"
	}
	if f.Mobile == "" {
		f.Mobile = fmt.Sprintf("139%08d", f.ID)
	}
	if f.Email == "" {
		f.Email = fmt.Sprintf("%s@example.test", f.Username)
	}
	if f.Password == "" {
		f.Password = "ChangeMeAdmin123"
	}
	if f.Status == "" {
		f.Status = string(domain.UserStatusActive)
	}
	if len(f.Roles) == 0 {
		f.Roles = []domain.Role{domain.RoleMember}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(f.Password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	managedDepartments, _ := json.Marshal(f.ManagedDepartments)
	managedTeams, _ := json.Marshal(f.ManagedTeams)
	if len(f.ManagedDepartments) == 0 {
		managedDepartments = nil
	}
	if len(f.ManagedTeams) == 0 {
		managedTeams = nil
	}
	_, err = db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, managed_departments_json, managed_teams_json,
			 mobile, email, password_hash, status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'full_time', 0, NOW(6), NOW(6))`,
		f.ID, f.Username, f.DisplayName, f.Department, f.Team, nullJSON(managedDepartments), nullJSON(managedTeams),
		f.Mobile, f.Email, string(hash), f.Status)
	if err != nil {
		t.Fatalf("insert user %d: %v", f.ID, err)
	}
	for _, role := range domain.NormalizeRoleValues(f.Roles) {
		if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, f.ID, role); err != nil {
			t.Fatalf("insert role %s for user %d: %v", role, f.ID, err)
		}
	}
}

func nullJSON(raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func saBCreateSession(t *testing.T, db *sql.DB, userID int64, token string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(token))
	_, err := db.Exec(`
		INSERT INTO user_sessions (session_id, user_id, token_hash, expires_at, last_seen_at, created_at)
		VALUES (?, ?, ?, DATE_ADD(NOW(6), INTERVAL 1 DAY), NOW(6), NOW(6))`,
		fmt.Sprintf("sab-%d-%s", userID, token), userID, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("insert session for user %d: %v", userID, err)
	}
	return token
}

func saBAuthRouter(svc service.IdentityService) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if token == raw {
			token = ""
		}
		if token != "" {
			actor, appErr := svc.ResolveRequestActor(c.Request.Context(), token)
			if appErr != nil {
				respondError(c, appErr)
				return
			}
			if actor != nil {
				ctx := domain.WithRequestBearerToken(c.Request.Context(), token)
				ctx = domain.WithRequestActor(ctx, *actor)
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	})
	return router
}

func saBPerformJSON(router *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	reqBody := bytes.NewBufferString(body)
	req := httptest.NewRequest(method, path, reqBody)
	if strings.TrimSpace(body) != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func saBDecode(t *testing.T, rec *httptest.ResponseRecorder, out interface{}) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}

func saBDenyCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env saBEnvelope
	saBDecode(t, rec, &env)
	if env.Error.Details == nil {
		return ""
	}
	if value, ok := env.Error.Details["deny_code"].(string); ok {
		return value
	}
	return ""
}

func saBPasswordHash(t *testing.T, db *sql.DB, userID int64) string {
	t.Helper()
	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		t.Fatalf("select password_hash for user %d: %v", userID, err)
	}
	return hash
}

func saBUserStatus(t *testing.T, db *sql.DB, userID int64) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err != nil {
		t.Fatalf("select status for user %d: %v", userID, err)
	}
	return status
}

func saBCountColumns(t *testing.T, db *sql.DB, names ...string) int {
	t.Helper()
	placeholders := make([]string, 0, len(names))
	args := make([]interface{}, 0, len(names))
	for _, name := range names {
		placeholders = append(placeholders, "?")
		args = append(args, name)
	}
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'users'
		  AND COLUMN_NAME IN (`+strings.Join(placeholders, ",")+`)`, args...).Scan(&count); err != nil {
		t.Fatalf("count users columns: %v", err)
	}
	return count
}

func saBNowMarker() time.Time {
	return time.Now().UTC().Add(-1 * time.Second)
}
