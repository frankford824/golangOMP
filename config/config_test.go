package config

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoadAuthSettingsRejectsBootstrapCredentialsByDefault(t *testing.T) {
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "false")

	_, err := loadAuthSettings("auth_identity.example.json")
	if err == nil {
		t.Fatal("loadAuthSettings() error = nil, want bootstrap credential rejection")
	}
	if !strings.Contains(err.Error(), "example default password") {
		t.Fatalf("loadAuthSettings() error = %v, want default password rejection", err)
	}
}

func TestLoadRejectsIncompleteAIChatAndVectorConfiguration(t *testing.T) {
	setBase := func(t *testing.T) {
		t.Helper()
		t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
		t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
		t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
		t.Setenv("WEB_PUSH_ENABLED", "false")
	}
	t.Run("chat provider required", func(t *testing.T) {
		setBase(t)
		t.Setenv("AI_CHAT_ENABLED", "true")
		t.Setenv("AI_AGENT_ENABLED", "false")
		t.Setenv("AI_AGENT_BASE_URL", "")
		t.Setenv("AI_AGENT_API_KEY", "")
		t.Setenv("AI_AGENT_MODEL", "")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_CHAT_ENABLED=true requires") {
			t.Fatalf("Load() error=%v", err)
		}
	})
	t.Run("chat provider protocol must be supported", func(t *testing.T) {
		setBase(t)
		t.Setenv("AI_CHAT_ENABLED", "true")
		t.Setenv("AI_AGENT_ENABLED", "true")
		t.Setenv("AI_AGENT_PROVIDER", "unsupported")
		t.Setenv("AI_AGENT_BASE_URL", "https://provider.test/v1")
		t.Setenv("AI_AGENT_API_KEY", "secret")
		t.Setenv("AI_AGENT_MODEL", "model")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_AGENT_PROVIDER must be") {
			t.Fatalf("Load() error=%v", err)
		}
	})
	t.Run("vector worker bounds required", func(t *testing.T) {
		setBase(t)
		t.Setenv("AI_CHAT_ENABLED", "false")
		t.Setenv("VECTOR_SEARCH_ENABLED", "true")
		t.Setenv("AI_EMBEDDING_ENABLED", "true")
		t.Setenv("AI_EMBEDDING_BASE_URL", "http://embedding.test")
		t.Setenv("AI_EMBEDDING_API_KEY", "secret")
		t.Setenv("AI_EMBEDDING_MODEL", "embed")
		t.Setenv("AI_EMBEDDING_VERSION", "embed:v1")
		t.Setenv("QDRANT_API_KEY", "secret")
		t.Setenv("AI_RETRIEVAL_WORKER_BATCH_SIZE", "0")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "vector retrieval configuration is invalid") {
			t.Fatalf("Load() error=%v", err)
		}
	})
}

func TestLoadDefaultsERPBridgeToLoopback(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_BRIDGE_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPBridge.BaseURL != "http://127.0.0.1:8081" {
		t.Fatalf("ERPBridge.BaseURL = %s, want http://127.0.0.1:8081", cfg.ERPBridge.BaseURL)
	}
}

func TestLoadCostGovernanceLegacyAliasFallbackDefaultAndOverride(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.CostGovernance.LegacyAliasFallbackEnabled {
		t.Fatal("LegacyAliasFallbackEnabled = false, want default true")
	}

	t.Setenv("COST_RULE_LEGACY_ALIAS_FALLBACK_ENABLED", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with override error = %v", err)
	}
	if cfg.CostGovernance.LegacyAliasFallbackEnabled {
		t.Fatal("LegacyAliasFallbackEnabled = true, want false override")
	}
}

func TestLoadUsesExplicitERPBridgeBaseURL(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_BRIDGE_BASE_URL", "http://223.4.249.11:8081")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPBridge.BaseURL != "http://223.4.249.11:8081" {
		t.Fatalf("ERPBridge.BaseURL = %s, want explicit override", cfg.ERPBridge.BaseURL)
	}
}

func TestLoadUsesERPBridgeInternalToken(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_BRIDGE_INTERNAL_TOKEN", "bridge-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPBridge.InternalToken != "bridge-secret" {
		t.Fatalf("ERPBridge.InternalToken = %q, want configured value", cfg.ERPBridge.InternalToken)
	}
}

func TestLoadUsesERPHistoryCostPath(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_REMOTE_HISTORY_COST_PATH", "/custom/history-cost")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPRemote.HistoryCostPath != "/custom/history-cost" {
		t.Fatalf("ERPRemote.HistoryCostPath = %q", cfg.ERPRemote.HistoryCostPath)
	}
}

func TestLoadIncludesUploadServiceDefaults(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("UPLOAD_SERVICE_BASE_URL", "")
	t.Setenv("UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL", "")
	t.Setenv("UPLOAD_SERVICE_BROWSER_DOWNLOAD_BASE_URL", "")
	t.Setenv("UPLOAD_SERVICE_TIMEOUT", "")
	t.Setenv("UPLOAD_STORAGE_PROVIDER", "")
	t.Setenv("UPLOAD_SERVICE_INTERNAL_TOKEN", "")
	t.Setenv("UPLOAD_SERVICE_AUTH_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.UploadService.Enabled {
		t.Fatal("UploadService.Enabled = false, want true")
	}
	if cfg.UploadService.BaseURL != "http://127.0.0.1:8092" {
		t.Fatalf("UploadService.BaseURL = %s, want default", cfg.UploadService.BaseURL)
	}
	if cfg.UploadService.BrowserMultipartBaseURL != "" {
		t.Fatalf("UploadService.BrowserMultipartBaseURL = %q, want empty until explicitly configured", cfg.UploadService.BrowserMultipartBaseURL)
	}
	if cfg.UploadService.BrowserDownloadBaseURL != "" {
		t.Fatalf("UploadService.BrowserDownloadBaseURL = %q, want empty until explicitly configured", cfg.UploadService.BrowserDownloadBaseURL)
	}
	if cfg.UploadService.Timeout.String() != "15s" {
		t.Fatalf("UploadService.Timeout = %s, want 15s", cfg.UploadService.Timeout)
	}
	if cfg.UploadService.InternalToken != "" {
		t.Fatalf("UploadService.InternalToken = %q, want empty", cfg.UploadService.InternalToken)
	}
	if cfg.UploadService.StorageProvider != "oss" {
		t.Fatalf("UploadService.StorageProvider = %s, want oss", cfg.UploadService.StorageProvider)
	}
}

func TestLoadExternalAssetsAListTimeoutOverride(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("EXTERNAL_ASSETS_ALIST_TIMEOUT", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ExternalAssets.AListTimeout != 90*time.Second {
		t.Fatalf("ExternalAssets.AListTimeout = %s, want 90s", cfg.ExternalAssets.AListTimeout)
	}
}

func TestLoadRejectsEnabledWebPushWithoutVAPIDConfig(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("WEB_PUSH_ENABLED", "true")
	t.Setenv("WEB_PUSH_VAPID_PUBLIC_KEY", "")
	t.Setenv("WEB_PUSH_VAPID_PRIVATE_KEY", "")
	t.Setenv("WEB_PUSH_SUBJECT", "   ")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing VAPID config rejection")
	}
	for _, want := range []string{
		"WEB_PUSH_ENABLED=true requires",
		"WEB_PUSH_VAPID_PUBLIC_KEY",
		"WEB_PUSH_VAPID_PRIVATE_KEY",
		"WEB_PUSH_SUBJECT",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Load() error = %v, want substring %q", err, want)
		}
	}
}

func TestLoadPrefersUploadServiceInternalToken(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("UPLOAD_SERVICE_INTERNAL_TOKEN", "internal-token")
	t.Setenv("UPLOAD_SERVICE_AUTH_TOKEN", "legacy-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UploadService.InternalToken != "internal-token" {
		t.Fatalf("UploadService.InternalToken = %q, want internal-token", cfg.UploadService.InternalToken)
	}
}

func TestLoadIncludesERPImageProxyDefaultsAndSecretFallback(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_IMAGE_PROXY_PUBLIC_BASE_URL", "")
	t.Setenv("ERP_IMAGE_PROXY_TOKEN_TTL", "")
	t.Setenv("ERP_IMAGE_PROXY_SIGNING_SECRET", "")
	t.Setenv("PRODUCT_MANAGEMENT_IMAGE_PROXY_SIGNING_SECRET", "")
	t.Setenv("UPLOAD_SERVICE_INTERNAL_TOKEN", "internal-token")
	t.Setenv("UPLOAD_SERVICE_AUTH_TOKEN", "legacy-token")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "oss-secret")
	t.Setenv("ERP_REMOTE_APP_SECRET", "erp-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPImageProxy.PublicBaseURL != "https://yongbo.cloud" {
		t.Fatalf("ERPImageProxy.PublicBaseURL = %q", cfg.ERPImageProxy.PublicBaseURL)
	}
	if cfg.ERPImageProxy.TokenTTL != 8760*time.Hour {
		t.Fatalf("ERPImageProxy.TokenTTL = %s, want 8760h", cfg.ERPImageProxy.TokenTTL)
	}
	if cfg.ERPImageProxy.SigningSecret != "internal-token" {
		t.Fatalf("ERPImageProxy.SigningSecret = %q, want internal token fallback", cfg.ERPImageProxy.SigningSecret)
	}
}

func TestLoadPrefersExplicitERPImageProxySigningSecret(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")
	t.Setenv("ERP_IMAGE_PROXY_SIGNING_SECRET", "explicit-secret")
	t.Setenv("UPLOAD_SERVICE_INTERNAL_TOKEN", "internal-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ERPImageProxy.SigningSecret != "explicit-secret" {
		t.Fatalf("ERPImageProxy.SigningSecret = %q, want explicit-secret", cfg.ERPImageProxy.SigningSecret)
	}
}

func TestLoadIncludesAuthAndFrontendAccessSettings(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Auth.Departments) == 0 {
		t.Fatal("Auth.Departments is empty")
	}
	if !cfg.Auth.PhoneUnique {
		t.Fatal("Auth.PhoneUnique = false, want true")
	}
	if len(cfg.Auth.SuperAdmins) == 0 || cfg.Auth.SuperAdmins[0].Username != "admin" {
		t.Fatalf("Auth.SuperAdmins = %+v", cfg.Auth.SuperAdmins)
	}
	hasTeam := false
	for _, teams := range cfg.Auth.DepartmentTeams {
		if len(teams) > 0 {
			hasTeam = true
			break
		}
	}
	if !hasTeam {
		t.Fatalf("Auth.DepartmentTeams = %+v, want at least one configured team", cfg.Auth.DepartmentTeams)
	}
	if cfg.FrontendAccess.Version == "" {
		t.Fatal("FrontendAccess.Version is empty")
	}
	if len(cfg.FrontendAccess.Roles) != 0 {
		t.Fatalf("FrontendAccess.Roles = %+v, legacy role grants must be empty", cfg.FrontendAccess.Roles)
	}
	if _, ok := cfg.FrontendAccess.MenuCatalog["cost_rules"]; !ok {
		t.Fatal("FrontendAccess.MenuCatalog.cost_rules is missing")
	}
	if len(cfg.FrontendAccess.Defaults.AllAuthenticated.Pages) == 0 {
		t.Fatalf("FrontendAccess.Defaults.AllAuthenticated = %+v", cfg.FrontendAccess.Defaults.AllAuthenticated)
	}
	if len(cfg.FrontendAccess.MenuCatalog) == 0 {
		t.Fatal("FrontendAccess.MenuCatalog is empty")
	}
}

// TestLoadAuthSettingsV1_0OfficialBaseline pins the v1.0 org master data
// convergence: config/auth_identity.json must expose exactly the official
// departments and teams documented in docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md
// "Organization And Permission Boundary Truth". Legacy departments/teams and
// legacy operations groups 1-7 must not be part of the runtime org source.
func TestLoadAuthSettingsV1_0OfficialBaseline(t *testing.T) {
	t.Setenv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("AUTH_ALLOW_EMBEDDED_SETTINGS", "true")
	t.Setenv("AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantDepartments := map[string][]string{
		"人事部":   {"人事管理组"},
		"运营部":   {"淘系一组", "淘系二组", "天猫一组", "天猫二组", "拼多多南京组", "拼多多池州组"},
		"设计研发部": {"默认组"},
		"定制美工部": {"默认组"},
		"审核部":   {"普通审核组", "定制审核组"},
		"云仓部":   {"默认组"},
		"未分配":   {"未分配池"},
	}

	if len(cfg.Auth.Departments) != len(wantDepartments) {
		t.Fatalf("Auth.Departments count = %d, want %d (entries=%v)", len(cfg.Auth.Departments), len(wantDepartments), cfg.Auth.Departments)
	}
	for _, dept := range cfg.Auth.Departments {
		if _, ok := wantDepartments[string(dept)]; !ok {
			t.Fatalf("Auth.Departments contains unexpected entry %q; legacy/compatibility departments must not be active in v1.0 baseline", dept)
		}
	}

	legacyDepartments := []string{"设计部", "采购部", "仓储部", "烘焙仓储部"}
	for _, dept := range legacyDepartments {
		if _, ok := cfg.Auth.DepartmentTeams[dept]; ok {
			t.Fatalf("Auth.DepartmentTeams still carries legacy department %q", dept)
		}
	}

	legacyOperationsGroups := []string{"运营一组", "运营二组", "运营三组", "运营四组", "运营五组", "运营六组", "运营七组"}
	for dept, teams := range cfg.Auth.DepartmentTeams {
		want, ok := wantDepartments[dept]
		if !ok {
			t.Fatalf("Auth.DepartmentTeams contains unexpected department %q", dept)
		}
		gotSorted := append([]string{}, teams...)
		wantSorted := append([]string{}, want...)
		sort.Strings(gotSorted)
		sort.Strings(wantSorted)
		if len(gotSorted) != len(wantSorted) {
			t.Fatalf("Auth.DepartmentTeams[%s] = %v, want %v", dept, teams, want)
		}
		for i := range gotSorted {
			if gotSorted[i] != wantSorted[i] {
				t.Fatalf("Auth.DepartmentTeams[%s] = %v, want %v", dept, teams, want)
			}
		}
		for _, legacy := range legacyOperationsGroups {
			for _, team := range teams {
				if team == legacy {
					t.Fatalf("Auth.DepartmentTeams[%s] still contains legacy operations group %q", dept, legacy)
				}
			}
		}
	}
}
