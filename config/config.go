package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	ERPImageProxy  ERPImageProxyConfig
	UploadService  UploadServiceConfig
	OSSDirect      OSSDirectConfig
	ExternalAssets ExternalAssetsConfig
	AssetWorkbench AssetWorkbenchConfig
	AssetCleanup   AssetCleanupConfig
	AI             AIConfig
	Experience     ExperienceConfig
	BusinessTrend  BusinessTrendConfig
	CostGovernance CostGovernanceConfig
	WeCom          WeComConfig
	WebPush        WebPushConfig
	Log            LogConfig
	Auth           domain.AuthSettings
	FrontendAccess domain.FrontendAccessSettings
}

type WeComConfig struct {
	AiBotEnabled       bool
	AiBotBotID         string
	AiBotSecret        string
	AiBotDefaultChatID string
	AiBotWSURL         string
	AiBotQueueSize     int
}

type WebPushConfig struct {
	Enabled                    bool
	VAPIDPublicKey             string
	VAPIDPrivateKey            string
	VAPIDSubject               string
	WorkerInterval             time.Duration
	WorkerLimit                int
	LeaseTTL                   time.Duration
	RetryBaseDelay             time.Duration
	MaxAttempts                int
	SKUSyncFailureScanInterval time.Duration
	SKUSyncFailureScanLimit    int
}

type AIConfig struct {
	Enabled         bool
	Provider        string
	BaseURL         string
	APIKey          string
	Model           string
	Timeout         time.Duration
	MaxTokens       int
	RateLimitWindow time.Duration
	RateLimitMax    int
}

type ExperienceConfig struct {
	UIEnabled                    bool
	CaptureEnabled               bool
	AIFeedbackEnabled            bool
	BehaviorCaptureEnabled       bool
	MicroQuestionEnabled         bool
	ReviewMaterializationEnabled bool
	BehaviorSampleRate           float64
	EnabledSurfaces              []string
	WorkerEnabled                bool
	WorkerInterval               time.Duration
	WorkerBatchSize              int
	WorkerMaxAttempts            int
	OutboxLeaseTTL               time.Duration
	RuntimeConfigFile            string
	RetentionDays                int
}

type BusinessTrendConfig struct {
	ChinaHotURL         string
	ApifyToken          string
	ApifyBaseURL        string
	ApifyDouyinHotActor string
	ApifyDouyinActor    string
	ApifyRedNoteActor   string
	Apify1688Actor      string
	ApifyTaobaoActor    string
	Timeout             time.Duration
	MaxExternalKeywords int
	MaxExternalItems    int
}

type CostGovernanceConfig struct {
	LegacyAliasFallbackEnabled bool
}

type OSSDirectConfig struct {
	Enabled         bool
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PresignExpiry   time.Duration
	HTTPTimeout     time.Duration
	PublicEndpoint  string
	PartSize        int64
}

type AssetCleanupConfig struct {
	Enabled bool
}

type ExternalAssetsConfig struct {
	Enabled             bool
	BFFBaseURL          string
	BFFBrowserBaseURL   string
	AListBaseURL        string
	AListToken          string
	AListMounts         string
	AListTimeout        time.Duration
	SyncInterval        time.Duration
	LinkRefreshInterval time.Duration
	FullSyncEnabled     bool
	FullSyncInterval    time.Duration
	FullSyncMounts      string
	FullSyncPageSize    int
	FullSyncMaxDepth    int
	FullSyncMaxFiles    int
	FullSyncMaxDirs     int
	OSSOriginalPrefix   string
	OSSPreviewPrefix    string
	OSSRequiredPrefixes string
	LocalPathMappings   string
	PrepareInterval     time.Duration
	PrepareLimit        int
	PrepareConcurrency  int
}

type AssetWorkbenchConfig struct {
	CookieDomain                string
	Timezone                    string
	OSSPrefix                   string
	UploadSessionTTL            time.Duration
	PreviewWorkerEnabled        bool
	PreviewWorkerInterval       time.Duration
	PreviewWorkerLimit          int
	PreviewWorkerLeaseTTL       time.Duration
	PreviewWorkerMaxAttempts    int
	PreviewWorkerRetryBaseDelay time.Duration
	UploadExpiryWorkerEnabled   bool
	UploadExpiryWorkerInterval  time.Duration
	UploadExpiryWorkerLimit     int
	BatchJobWorkerEnabled       bool
	BatchJobWorkerInterval      time.Duration
	BatchJobWorkerLimit         int
	BatchJobWorkerLeaseTTL      time.Duration
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
	CombineSKUQueryPath      string
	OrderActionQueryPath     string
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

type ERPImageProxyConfig struct {
	PublicBaseURL string
	SigningSecret string
	TokenTTL      time.Duration
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
			// No default DSN: shipping a built-in root:password fallback is a
			// credential risk, so deployments must set MYSQL_DSN explicitly.
			DSN:             getEnv("MYSQL_DSN", ""),
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
			Interval:   mustParseDuration(getEnv("ERP_SYNC_INTERVAL", "1h")),
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
			CombineSKUQueryPath:      getEnv("ERP_REMOTE_COMBINE_SKU_QUERY_PATH", "/open/combine/sku/query"),
			OrderActionQueryPath:     getEnv("ERP_REMOTE_ORDER_ACTION_QUERY_PATH", "/open/order/action/query"),
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
		ERPImageProxy: ERPImageProxyConfig{
			PublicBaseURL: getEnv("ERP_IMAGE_PROXY_PUBLIC_BASE_URL", "https://yongbo.cloud"),
			SigningSecret: firstNonEmptyEnv(
				"ERP_IMAGE_PROXY_SIGNING_SECRET",
				"PRODUCT_MANAGEMENT_IMAGE_PROXY_SIGNING_SECRET",
				"UPLOAD_SERVICE_INTERNAL_TOKEN",
				"UPLOAD_SERVICE_AUTH_TOKEN",
				"OSS_ACCESS_KEY_SECRET",
				"ERP_REMOTE_APP_SECRET",
			),
			TokenTTL: mustParseDuration(getEnv("ERP_IMAGE_PROXY_TOKEN_TTL", "8760h")),
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
			HTTPTimeout:     mustParseDuration(getEnv("OSS_HTTP_TIMEOUT", "5m")),
			PublicEndpoint:  getEnv("OSS_PUBLIC_ENDPOINT", ""),
			PartSize:        mustParseInt64(getEnv("OSS_PART_SIZE", "10485760")),
		},
		ExternalAssets: ExternalAssetsConfig{
			Enabled:             mustParseBool(getEnv("EXTERNAL_ASSETS_ENABLED", "false")),
			BFFBaseURL:          getEnv("EXTERNAL_ASSETS_BFF_BASE_URL", ""),
			BFFBrowserBaseURL:   getEnv("EXTERNAL_ASSETS_BFF_BROWSER_BASE_URL", ""),
			AListBaseURL:        getEnv("EXTERNAL_ASSETS_ALIST_BASE_URL", ""),
			AListToken:          getEnv("EXTERNAL_ASSETS_ALIST_TOKEN", ""),
			AListMounts:         getEnv("EXTERNAL_ASSETS_ALIST_MOUNTS", "/quark:netdisk,/p1:netdisk,/p2:netdisk,/p3:nas_local"),
			AListTimeout:        mustParseDuration(getEnv("EXTERNAL_ASSETS_ALIST_TIMEOUT", "30s")),
			SyncInterval:        mustParseDuration(getEnv("EXTERNAL_ASSETS_SYNC_INTERVAL", "1h")),
			LinkRefreshInterval: mustParseDuration(getEnv("EXTERNAL_ASSETS_LINK_REFRESH_INTERVAL", "1h")),
			FullSyncEnabled:     mustParseBool(getEnv("EXTERNAL_ASSETS_FULL_SYNC_ENABLED", "false")),
			FullSyncInterval:    mustParseDuration(getEnv("EXTERNAL_ASSETS_FULL_SYNC_INTERVAL", "")),
			FullSyncMounts:      getEnv("EXTERNAL_ASSETS_FULL_SYNC_MOUNTS", ""),
			FullSyncPageSize:    mustParseInt(getEnv("EXTERNAL_ASSETS_FULL_SYNC_PAGE_SIZE", "100")),
			FullSyncMaxDepth:    mustParseInt(getEnv("EXTERNAL_ASSETS_FULL_SYNC_MAX_DEPTH", "16")),
			FullSyncMaxFiles:    mustParseInt(getEnv("EXTERNAL_ASSETS_FULL_SYNC_MAX_FILES_PER_MOUNT", "20000")),
			FullSyncMaxDirs:     mustParseInt(getEnv("EXTERNAL_ASSETS_FULL_SYNC_MAX_DIRS_PER_MOUNT", "5000")),
			OSSOriginalPrefix:   getEnv("EXTERNAL_ASSETS_OSS_ORIGINAL_PREFIX", "external-assets/alist/original"),
			OSSPreviewPrefix:    getEnv("EXTERNAL_ASSETS_OSS_PREVIEW_PREFIX", "external-assets/alist/preview"),
			OSSRequiredPrefixes: getEnv("EXTERNAL_ASSETS_OSS_REQUIRED_PREFIXES", "/p3/仓库素材区/徐凯"),
			LocalPathMappings:   getEnv("EXTERNAL_ASSETS_LOCAL_PATH_MAPPINGS", "/p3=/volume1/image_lib"),
			PrepareInterval:     mustParseDuration(getEnv("EXTERNAL_ASSETS_PREPARE_INTERVAL", "30s")),
			PrepareLimit:        mustParseInt(getEnv("EXTERNAL_ASSETS_PREPARE_LIMIT", "50")),
			PrepareConcurrency:  mustParseInt(getEnv("EXTERNAL_ASSETS_PREPARE_CONCURRENCY", "4")),
		},
		AssetWorkbench: AssetWorkbenchConfig{
			CookieDomain:                getEnv("ASSET_COOKIE_DOMAIN", ""),
			Timezone:                    getEnv("ASSET_WORKBENCH_TIMEZONE", "Asia/Shanghai"),
			OSSPrefix:                   getEnv("ASSET_WORKBENCH_OSS_PREFIX", "asset-workbench"),
			UploadSessionTTL:            mustParseDuration(getEnv("ASSET_WORKBENCH_UPLOAD_SESSION_TTL", "24h")),
			PreviewWorkerEnabled:        mustParseBool(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_ENABLED", "false")),
			PreviewWorkerInterval:       mustParseDuration(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_INTERVAL", "15s")),
			PreviewWorkerLimit:          mustParseInt(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_LIMIT", "8")),
			PreviewWorkerLeaseTTL:       mustParseDuration(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_LEASE_TTL", "5m")),
			PreviewWorkerMaxAttempts:    mustParseInt(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_MAX_ATTEMPTS", "5")),
			PreviewWorkerRetryBaseDelay: mustParseDuration(getEnv("ASSET_WORKBENCH_PREVIEW_WORKER_RETRY_BASE_DELAY", "30s")),
			UploadExpiryWorkerEnabled:   mustParseBool(getEnv("ASSET_WORKBENCH_UPLOAD_EXPIRY_WORKER_ENABLED", "true")),
			UploadExpiryWorkerInterval:  mustParseDuration(getEnv("ASSET_WORKBENCH_UPLOAD_EXPIRY_WORKER_INTERVAL", "10m")),
			UploadExpiryWorkerLimit:     mustParseInt(getEnv("ASSET_WORKBENCH_UPLOAD_EXPIRY_WORKER_LIMIT", "100")),
			BatchJobWorkerEnabled:       mustParseBool(getEnv("ASSET_WORKBENCH_BATCH_JOB_WORKER_ENABLED", "true")),
			BatchJobWorkerInterval:      mustParseDuration(getEnv("ASSET_WORKBENCH_BATCH_JOB_WORKER_INTERVAL", "5s")),
			BatchJobWorkerLimit:         mustParseInt(getEnv("ASSET_WORKBENCH_BATCH_JOB_WORKER_LIMIT", "2")),
			BatchJobWorkerLeaseTTL:      mustParseDuration(getEnv("ASSET_WORKBENCH_BATCH_JOB_WORKER_LEASE_TTL", "10m")),
		},
		AssetCleanup: AssetCleanupConfig{
			Enabled: mustParseBool(getEnv("ASSET_CLEANUP_ENABLED", "false")),
		},
		AI: AIConfig{
			Enabled:         mustParseBool(getEnv("AI_AGENT_ENABLED", "false")),
			Provider:        getEnv("AI_AGENT_PROVIDER", "anthropic_compatible"),
			BaseURL:         getEnv("AI_AGENT_BASE_URL", ""),
			APIKey:          getEnv("AI_AGENT_API_KEY", ""),
			Model:           getEnv("AI_AGENT_MODEL", ""),
			Timeout:         mustParseDuration(getEnv("AI_AGENT_TIMEOUT", "30s")),
			MaxTokens:       mustParseInt(getEnv("AI_AGENT_MAX_TOKENS", "900")),
			RateLimitWindow: mustParseDuration(getEnv("AI_AGENT_RATE_LIMIT_WINDOW", "5h")),
			RateLimitMax:    mustParseInt(getEnv("AI_AGENT_RATE_LIMIT_MAX_CALLS", "800")),
		},
		Experience: ExperienceConfig{
			UIEnabled:                    mustParseBool(getEnv("EXPERIENCE_UI_ENABLED", "false")),
			CaptureEnabled:               mustParseBool(getEnv("EXPERIENCE_CAPTURE_ENABLED", "false")),
			AIFeedbackEnabled:            mustParseBool(getEnv("EXPERIENCE_AI_FEEDBACK_ENABLED", "false")),
			BehaviorCaptureEnabled:       mustParseBool(getEnv("EXPERIENCE_BEHAVIOR_CAPTURE_ENABLED", "false")),
			MicroQuestionEnabled:         mustParseBool(getEnv("EXPERIENCE_MICRO_QUESTION_ENABLED", "false")),
			ReviewMaterializationEnabled: mustParseBool(getEnv("EXPERIENCE_REVIEW_MATERIALIZATION_ENABLED", "false")),
			BehaviorSampleRate:           mustParseFloat(getEnv("EXPERIENCE_BEHAVIOR_SAMPLE_RATE", "0.2")),
			EnabledSurfaces:              splitCSV(getEnv("EXPERIENCE_ENABLED_SURFACES", "task_detail,asset_center,data_center")),
			WorkerEnabled:                mustParseBool(getEnv("EXPERIENCE_WORKER_ENABLED", "false")),
			WorkerInterval:               mustParseDuration(getEnv("EXPERIENCE_WORKER_INTERVAL", "15s")),
			WorkerBatchSize:              mustParseInt(getEnv("EXPERIENCE_WORKER_BATCH_SIZE", "50")),
			WorkerMaxAttempts:            mustParseInt(getEnv("EXPERIENCE_WORKER_MAX_ATTEMPTS", "5")),
			OutboxLeaseTTL:               mustParseDuration(getEnv("EXPERIENCE_OUTBOX_LEASE_TTL", "5m")),
			RuntimeConfigFile:            getEnv("EXPERIENCE_RUNTIME_CONFIG_FILE", ""),
			RetentionDays:                mustParseInt(getEnv("EXPERIENCE_RETENTION_DAYS", "180")),
		},
		BusinessTrend: BusinessTrendConfig{
			ChinaHotURL:         getEnv("BUSINESS_TREND_CHINA_HOT_URL", ""),
			ApifyToken:          getEnv("APIFY_TOKEN", ""),
			ApifyBaseURL:        getEnv("BUSINESS_TREND_APIFY_BASE_URL", "https://api.apify.com"),
			ApifyDouyinHotActor: getEnv("BUSINESS_TREND_APIFY_DOUYIN_HOT_ACTOR", "zen-studio/douyin-hot-search-scraper"),
			ApifyDouyinActor:    getEnv("BUSINESS_TREND_APIFY_DOUYIN_SEARCH_ACTOR", "zen-studio/douyin-search-scraper"),
			ApifyRedNoteActor:   getEnv("BUSINESS_TREND_APIFY_REDNOTE_SEARCH_ACTOR", "zen-studio/rednote-search-scraper"),
			Apify1688Actor:      getEnv("BUSINESS_TREND_APIFY_1688_ACTOR", "automation-lab/1688-scraper"),
			ApifyTaobaoActor:    getEnv("BUSINESS_TREND_APIFY_TAOBAO_ACTOR", "zen-studio/taobao-detail-scraper"),
			Timeout:             mustParseDuration(getEnv("BUSINESS_TREND_EXTERNAL_TIMEOUT", "20s")),
			MaxExternalKeywords: mustParseInt(getEnv("BUSINESS_TREND_MAX_EXTERNAL_KEYWORDS", "8")),
			MaxExternalItems:    mustParseInt(getEnv("BUSINESS_TREND_MAX_EXTERNAL_ITEMS", "24")),
		},
		CostGovernance: CostGovernanceConfig{
			LegacyAliasFallbackEnabled: mustParseBool(getEnv("COST_RULE_LEGACY_ALIAS_FALLBACK_ENABLED", "true")),
		},
		WeCom: WeComConfig{
			AiBotEnabled:       mustParseBool(getEnv("WECOM_AIBOT_ENABLED", "false")),
			AiBotBotID:         getEnv("WECOM_AIBOT_BOT_ID", ""),
			AiBotSecret:        getEnv("WECOM_AIBOT_SECRET", ""),
			AiBotDefaultChatID: getEnv("WECOM_AIBOT_DEFAULT_CHAT_ID", ""),
			AiBotWSURL:         getEnv("WECOM_AIBOT_WS_URL", "wss://openws.work.weixin.qq.com"),
			AiBotQueueSize:     mustParseInt(getEnv("WECOM_AIBOT_QUEUE_SIZE", "200")),
		},
		WebPush: WebPushConfig{
			Enabled:                    mustParseBool(getEnv("WEB_PUSH_ENABLED", "false")),
			VAPIDPublicKey:             getEnv("WEB_PUSH_VAPID_PUBLIC_KEY", ""),
			VAPIDPrivateKey:            getEnv("WEB_PUSH_VAPID_PRIVATE_KEY", ""),
			VAPIDSubject:               getEnv("WEB_PUSH_SUBJECT", "mailto:ops@yongbo.cloud"),
			WorkerInterval:             mustParseDuration(getEnv("WEB_PUSH_WORKER_INTERVAL", "10s")),
			WorkerLimit:                mustParseInt(getEnv("WEB_PUSH_WORKER_LIMIT", "20")),
			LeaseTTL:                   mustParseDuration(getEnv("WEB_PUSH_LEASE_TTL", "2m")),
			RetryBaseDelay:             mustParseDuration(getEnv("WEB_PUSH_RETRY_BASE_DELAY", "30s")),
			MaxAttempts:                mustParseInt(getEnv("WEB_PUSH_MAX_ATTEMPTS", "5")),
			SKUSyncFailureScanInterval: mustParseDuration(getEnv("SKU_SYNC_FAILURE_NOTIFICATION_SCAN_INTERVAL", "5m")),
			SKUSyncFailureScanLimit:    mustParseInt(getEnv("SKU_SYNC_FAILURE_NOTIFICATION_SCAN_LIMIT", "50")),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Auth:           authSettings,
		FrontendAccess: frontendAccess,
	}
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("MYSQL_DSN is required (no built-in default; set the environment variable explicitly)")
	}
	if cfg.WebPush.Enabled {
		missing := make([]string, 0, 3)
		if strings.TrimSpace(cfg.WebPush.VAPIDPublicKey) == "" {
			missing = append(missing, "WEB_PUSH_VAPID_PUBLIC_KEY")
		}
		if strings.TrimSpace(cfg.WebPush.VAPIDPrivateKey) == "" {
			missing = append(missing, "WEB_PUSH_VAPID_PRIVATE_KEY")
		}
		if strings.TrimSpace(cfg.WebPush.VAPIDSubject) == "" {
			missing = append(missing, "WEB_PUSH_SUBJECT")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("WEB_PUSH_ENABLED=true requires %s", strings.Join(missing, ", "))
		}
	}
	return cfg, nil
}

// Known credential placeholders from auth_identity.example.json. They must
// never appear in a production identity config loaded from disk.
const (
	exampleSuperAdminPassword        = "ChangeMeAdmin123"
	bootstrapSuperAdminPassword      = "520520Abc"
	allowBootstrapCredentialsEnvName = "AUTH_ALLOW_INSECURE_BOOTSTRAP_CREDENTIALS"
	exampleDepartmentAdminKey        = "CHANGE_ME_ADMIN_KEY"
)

func loadAuthSettings(path string) (domain.AuthSettings, error) {
	settings := domain.AuthSettings{}
	raw, readErr := os.ReadFile(path)
	switch {
	case readErr == nil && len(raw) > 0:
		if err := json.Unmarshal(raw, &settings); err != nil {
			return domain.AuthSettings{}, fmt.Errorf("load auth settings: %w", err)
		}
		if err := rejectExampleCredentials(settings); err != nil {
			return domain.AuthSettings{}, fmt.Errorf("auth settings %q: %w", path, err)
		}
	case mustParseBool(getEnv("AUTH_ALLOW_EMBEDDED_SETTINGS", "false")):
		// Dev/test escape hatch: the embedded example contains placeholder
		// credentials and must never silently seed a production deployment.
		if err := json.Unmarshal(embeddedAuthSettings, &settings); err != nil {
			return domain.AuthSettings{}, fmt.Errorf("load embedded auth settings: %w", err)
		}
		if err := rejectExampleCredentials(settings); err != nil {
			return domain.AuthSettings{}, fmt.Errorf("embedded auth settings: %w", err)
		}
	default:
		return domain.AuthSettings{}, fmt.Errorf(
			"auth settings file %q is missing or empty; set AUTH_SETTINGS_FILE to a real identity config (the embedded example fallback is disabled unless AUTH_ALLOW_EMBEDDED_SETTINGS=true)",
			path,
		)
	}
	return settings, validateAuthSettings(settings)
}

func rejectExampleCredentials(settings domain.AuthSettings) error {
	allowBootstrap := mustParseBool(getEnv(allowBootstrapCredentialsEnvName, "false"))
	for _, entry := range settings.SuperAdmins {
		switch entry.Password {
		case exampleSuperAdminPassword, bootstrapSuperAdminPassword:
			if allowBootstrap {
				continue
			}
			return fmt.Errorf("super admin %q still uses the example default password; change it before starting the server", entry.Username)
		}
	}
	for department, keys := range settings.DepartmentAdminKeys {
		for _, key := range keys {
			if key == exampleDepartmentAdminKey {
				if allowBootstrap {
					continue
				}
				return fmt.Errorf("department %q still uses the example admin key placeholder; change it before starting the server", department)
			}
		}
	}
	return nil
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
	// Existing auth settings files may still reference retired departments;
	// keep accepting them for config load while seeding stays baseline-only.
	for _, department := range domain.CompatibilityDepartments() {
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

func mustParseFloat(s string) float64 {
	n, _ := strconv.ParseFloat(s, 64)
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

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
