package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"workflow/domain"
)

//go:embed auth_identity.example.json
var embeddedAuthSettings []byte

//go:embed frontend_access.json
var embeddedFrontendAccess []byte

type Config struct {
	Server         ServerConfig
	MySQL          MySQLConfig
	Redis          RedisConfig
	ERP            ERPSyncConfig
	ERPBridge      ERPBridgeConfig
	ERPRemote      ERPRemoteConfig
	UploadService  UploadServiceConfig
	OSSDirect      OSSDirectConfig
	AssetCleanup   AssetCleanupConfig
	Log            LogConfig
	Auth           domain.AuthSettings
	FrontendAccess domain.FrontendAccessSettings
}

type OSSDirectConfig struct {
	Enabled         bool
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PresignExpiry   time.Duration
	PublicEndpoint  string
	PartSize        int64
}

type AssetCleanupConfig struct {
	Enabled bool
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type MySQLConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ERPSyncConfig struct {
	Enabled    bool
	Interval   time.Duration
	SourceMode string
	StubFile   string
	Timeout    time.Duration
}

type ERPBridgeConfig struct {
	BaseURL string
	Timeout time.Duration
}

type ERPRemoteConfig struct {
	Mode                     string
	BaseURL                  string
	UpsertPath               string
	ItemStyleUpdatePath      string
	ShelveBatchPath          string
	UnshelveBatchPath        string
	VirtualQtyPath           string
	SyncLogsPath             string
	GetCompanyUsersPath      string
	SkuQueryPath             string
	OpenWebCharset           string
	OpenWebVersion           string
	Timeout                  time.Duration
	RetryMax                 int
	RetryBackoff             time.Duration
	AuthMode                 string
	AuthHeaderToken          string
	AppKey                   string
	AppSecret                string
	AccessToken              string
	HeaderAppKey             string
	HeaderAccessToken        string
	HeaderTimestamp          string
	HeaderNonce              string
	HeaderSignature          string
	SignatureIncludeBodyHash bool
	FallbackToLocalOnError   bool
}

type UploadServiceConfig struct {
	Enabled                 bool
	BaseURL                 string
	BrowserMultipartBaseURL string
	BrowserDownloadBaseURL  string
	Timeout                 time.Duration
	InternalToken           string
	StorageProvider         string
}

type LogConfig struct {
	Level string
}

func Load() (*Config, error) {
	authSettings, err := loadAuthSettings(getEnv("AUTH_SETTINGS_FILE", "config/auth_identity.json"))
	if err != nil {
		return nil, err
	}
	frontendAccess, err := loadFrontendAccessSettings(getEnv("FRONTEND_ACCESS_SETTINGS_FILE", "config/frontend_access.json"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  mustParseDuration(getEnv("SERVER_READ_TIMEOUT", "30s")),
			WriteTimeout: mustParseDuration(getEnv("SERVER_WRITE_TIMEOUT", "30s")),
		},
		MySQL: MySQLConfig{
			DSN:             getEnv("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local"),
			MaxOpenConns:    mustParseInt(getEnv("MYSQL_MAX_OPEN_CONNS", "25")),
			MaxIdleConns:    mustParseInt(getEnv("MYSQL_MAX_IDLE_CONNS", "10")),
			ConnMaxLifetime: mustParseDuration(getEnv("MYSQL_CONN_MAX_LIFETIME", "5m")),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       mustParseInt(getEnv("REDIS_DB", "0")),
		},
		ERP: ERPSyncConfig{
			Enabled:    mustParseBool(getEnv("ERP_SYNC_ENABLED", "true")),
			Interval:   mustParseDuration(getEnv("ERP_SYNC_INTERVAL", "5m")),
			SourceMode: getEnv("ERP_SYNC_SOURCE_MODE", "stub"),
			StubFile:   getEnv("ERP_SYNC_STUB_FILE", "config/erp_products_stub.json"),
			Timeout:    mustParseDuration(getEnv("ERP_SYNC_TIMEOUT", "30s")),
		},
		ERPBridge: ERPBridgeConfig{
			BaseURL: getEnv("ERP_BRIDGE_BASE_URL", "http://127.0.0.1:8081"),
			Timeout: mustParseDuration(getEnv("ERP_BRIDGE_TIMEOUT", "15s")),
		},
		ERPRemote: ERPRemoteConfig{
			Mode:                     getEnv("ERP_REMOTE_MODE", "local"),
			BaseURL:                  getEnv("ERP_REMOTE_BASE_URL", ""),
			UpsertPath:               getEnv("ERP_REMOTE_UPSERT_PATH", "/open/webapi/itemapi/itemsku/itemskubatchupload"),
			ItemStyleUpdatePath:      getEnv("ERP_REMOTE_ITEM_STYLE_UPDATE_PATH", "/open/webapi/itemapi/itemskuim/itemupload"),
			ShelveBatchPath:          getEnv("ERP_REMOTE_SHELVE_BATCH_PATH", "/open/webapi/wmsapi/openshelve/skubatchshelve"),
			UnshelveBatchPath:        getEnv("ERP_REMOTE_UNSHELVE_BATCH_PATH", "/open/webapi/wmsapi/openoffshelve/skubatchoffshelve"),
			VirtualQtyPath:           getEnv("ERP_REMOTE_VIRTUAL_QTY_PATH", "/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys"),
			SyncLogsPath:             getEnv("ERP_REMOTE_SYNC_LOGS_PATH", "/v1/erp/sync-logs"),
			GetCompanyUsersPath:      getEnv("ERP_REMOTE_GET_COMPANY_USERS_PATH", "/open/webapi/userapi/company/getcompanyusers"),
			SkuQueryPath:             getEnv("ERP_REMOTE_SKU_QUERY_PATH", "/open/sku/query"),
			OpenWebCharset:           getEnv("ERP_REMOTE_OPENWEB_CHARSET", "utf-8"),
			OpenWebVersion:           getEnv("ERP_REMOTE_OPENWEB_VERSION", "2"),
			Timeout:                  mustParseDuration(getEnv("ERP_REMOTE_TIMEOUT", "15s")),
			RetryMax:                 mustParseInt(getEnv("ERP_REMOTE_RETRY_MAX", "2")),
			RetryBackoff:             mustParseDuration(getEnv("ERP_REMOTE_RETRY_BACKOFF", "600ms")),
			AuthMode:                 getEnv("ERP_REMOTE_AUTH_MODE", "none"),
			AuthHeaderToken:          getEnv("ERP_REMOTE_AUTH_HEADER_TOKEN", ""),
			AppKey:                   getEnv("ERP_REMOTE_APP_KEY", ""),
			AppSecret:                getEnv("ERP_REMOTE_APP_SECRET", ""),
			AccessToken:              getEnv("ERP_REMOTE_ACCESS_TOKEN", ""),
			HeaderAppKey:             getEnv("ERP_REMOTE_HEADER_APP_KEY", "X-App-Key"),
			HeaderAccessToken:        getEnv("ERP_REMOTE_HEADER_ACCESS_TOKEN", "X-Access-Token"),
			HeaderTimestamp:          getEnv("ERP_REMOTE_HEADER_TIMESTAMP", "X-Timestamp"),
			HeaderNonce:              getEnv("ERP_REMOTE_HEADER_NONCE", "X-Nonce"),
			HeaderSignature:          getEnv("ERP_REMOTE_HEADER_SIGNATURE", "X-Signature"),
			SignatureIncludeBodyHash: mustParseBool(getEnv("ERP_REMOTE_SIGNATURE_INCLUDE_BODY_HASH", "true")),
			FallbackToLocalOnError:   mustParseBool(getEnv("ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR", "true")),
		},
		UploadService: UploadServiceConfig{
			Enabled:                 mustParseBool(getEnv("UPLOAD_SERVICE_ENABLED", "true")),
			BaseURL:                 getEnv("UPLOAD_SERVICE_BASE_URL", "http://127.0.0.1:8092"),
			BrowserMultipartBaseURL: getEnv("UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL", ""),
			BrowserDownloadBaseURL:  getEnv("UPLOAD_SERVICE_BROWSER_DOWNLOAD_BASE_URL", ""),
			Timeout:                 mustParseDuration(getEnv("UPLOAD_SERVICE_TIMEOUT", "15s")),
			InternalToken:           firstNonEmptyEnv("UPLOAD_SERVICE_INTERNAL_TOKEN", "UPLOAD_SERVICE_AUTH_TOKEN"),
			StorageProvider:         getEnv("UPLOAD_STORAGE_PROVIDER", "oss"),
		},
		OSSDirect: OSSDirectConfig{
			Enabled:         mustParseBool(getEnv("OSS_DIRECT_ENABLED", "false")),
			Endpoint:        getEnv("OSS_ENDPOINT", ""),
			Bucket:          getEnv("OSS_BUCKET", ""),
			AccessKeyID:     getEnv("OSS_ACCESS_KEY_ID", ""),
			AccessKeySecret: getEnv("OSS_ACCESS_KEY_SECRET", ""),
			PresignExpiry:   mustParseDuration(getEnv("OSS_PRESIGN_EXPIRY", "15m")),
			PublicEndpoint:  getEnv("OSS_PUBLIC_ENDPOINT", ""),
			PartSize:        mustParseInt64(getEnv("OSS_PART_SIZE", "10485760")),
		},
		AssetCleanup: AssetCleanupConfig{
			Enabled: mustParseBool(getEnv("ASSET_CLEANUP_ENABLED", "false")),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Auth:           authSettings,
		FrontendAccess: frontendAccess,
	}
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("MYSQL_DSN is required")
	}
	return cfg, nil
}

func loadAuthSettings(path string) (domain.AuthSettings, error) {
	settings := domain.AuthSettings{}
	if err := unmarshalConfigFile(path, embeddedAuthSettings, &settings); err != nil {
		return domain.AuthSettings{}, fmt.Errorf("load auth settings: %w", err)
	}
	return settings, validateAuthSettings(settings)
}

func loadFrontendAccessSettings(path string) (domain.FrontendAccessSettings, error) {
	settings := domain.FrontendAccessSettings{}
	if err := unmarshalConfigFile(path, embeddedFrontendAccess, &settings); err != nil {
		return domain.FrontendAccessSettings{}, fmt.Errorf("load frontend access settings: %w", err)
	}
	return settings, nil
}

func unmarshalConfigFile(path string, fallback []byte, target interface{}) error {
	contents := fallback
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		contents = raw
	}
	return json.Unmarshal(contents, target)
}

func validateAuthSettings(settings domain.AuthSettings) error {
	if len(settings.Departments) == 0 {
		return fmt.Errorf("departments must not be empty")
	}
	validDepartments := map[domain.Department]struct{}{}
	for _, department := range domain.DefaultDepartments() {
		validDepartments[department] = struct{}{}
	}
	for _, department := range settings.Departments {
		if _, ok := validDepartments[department]; !ok {
			return fmt.Errorf("unknown department %q in auth settings", department)
		}
	}
	for key := range settings.DepartmentAdminKeys {
		if _, ok := validDepartments[domain.Department(key)]; !ok {
			return fmt.Errorf("unknown department %q in department_admin_keys", key)
		}
	}
	for key, teams := range settings.DepartmentTeams {
		if _, ok := validDepartments[domain.Department(key)]; !ok {
			return fmt.Errorf("unknown department %q in department_teams", key)
		}
		seen := map[string]struct{}{}
		for _, team := range teams {
			if team == "" {
				return fmt.Errorf("department %q contains empty team", key)
			}
			if _, ok := seen[team]; ok {
				return fmt.Errorf("department %q contains duplicate team %q", key, team)
			}
			seen[team] = struct{}{}
		}
	}
	for _, entry := range settings.SuperAdmins {
		teams := settings.DepartmentTeams[string(entry.Department)]
		if entry.Team != "" {
			valid := false
			for _, team := range teams {
				if team == entry.Team {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("super admin %q has invalid team %q for department %q", entry.Username, entry.Team, entry.Department)
			}
		}
		for _, role := range entry.Roles {
			if !domain.IsKnownRole(role) {
				return fmt.Errorf("unknown role %q in super_admins", role)
			}
		}
		for _, department := range entry.ManagedDepartments {
			if _, ok := validDepartments[domain.Department(department)]; !ok {
				return fmt.Errorf("unknown managed department %q in super_admins", department)
			}
		}
		seenTeams := map[string]struct{}{}
		for _, team := range teams {
			seenTeams[team] = struct{}{}
		}
		for _, team := range entry.ManagedTeams {
			if _, ok := seenTeams[team]; !ok {
				return fmt.Errorf("unknown managed team %q in super_admins for department %q", team, entry.Department)
			}
		}
		if entry.Status != "" && !entry.Status.Valid() {
			return fmt.Errorf("invalid status %q in super_admins", entry.Status)
		}
		if entry.EmploymentType != "" && !entry.EmploymentType.Valid() {
			return fmt.Errorf("invalid employment_type %q in super_admins", entry.EmploymentType)
		}
	}
	for _, entry := range settings.ConfiguredAssignments {
		if entry.Department == "" {
			return fmt.Errorf("configured user assignment department is required")
		}
		if _, ok := validDepartments[entry.Department]; !ok {
			return fmt.Errorf("unknown department %q in configured_user_assignments", entry.Department)
		}
		teams := settings.DepartmentTeams[string(entry.Department)]
		valid := false
		for _, team := range teams {
			if team == entry.Team {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("configured user assignment has invalid team %q for department %q", entry.Team, entry.Department)
		}
		for _, department := range entry.ManagedDepartments {
			if _, ok := validDepartments[domain.Department(department)]; !ok {
				return fmt.Errorf("unknown managed department %q in configured_user_assignments", department)
			}
		}
		seenTeams := map[string]struct{}{}
		for _, team := range teams {
			seenTeams[team] = struct{}{}
		}
		for _, team := range entry.ManagedTeams {
			if _, ok := seenTeams[team]; !ok {
				return fmt.Errorf("unknown managed team %q in configured_user_assignments for department %q", team, entry.Department)
			}
		}
		for _, role := range entry.Roles {
			if !domain.IsKnownRole(role) {
				return fmt.Errorf("unknown role %q in configured_user_assignments", role)
			}
		}
		if entry.Status != "" && !entry.Status.Valid() {
			return fmt.Errorf("invalid status %q in configured_user_assignments", entry.Status)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return v
		}
	}
	return ""
}

func mustParseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func mustParseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}

func mustParseBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func mustParseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
