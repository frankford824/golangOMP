package assetworkbench

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	assetcenter "workflow/service/asset_center"
)

type Config struct {
	Timezone                 string
	OSSPrefix                string
	UploadSessionTTL         time.Duration
	PreviewWorkerLeaseTTL    time.Duration
	PreviewWorkerMaxAttempts int
	BatchJobWorkerLeaseTTL   time.Duration
}

type objectStore interface {
	Enabled() bool
	CreateUploadPlan(context.Context, string, int64, string) (*baseservice.OSSDirectUploadPlan, error)
	CompleteMultipartUpload(context.Context, string, string, []baseservice.OSSCompletePart) error
	AbortMultipartUpload(context.Context, string, string) error
	StatObject(context.Context, string) (*baseservice.OSSObjectInfo, bool, error)
	DeleteObject(context.Context, string) error
	CopyObject(context.Context, string, string) error
	OpenObject(context.Context, string) (io.ReadCloser, error)
	UploadObject(context.Context, string, string, []byte) error
	PresignDownloadURLWithFilename(string, string) *baseservice.OSSDirectDownloadInfo
	PresignPreviewURL(string) *baseservice.OSSDirectDownloadInfo
}

type Option func(*Service)

const (
	assetWorkbenchProfileAutoRepriceBatchSize      = 200
	assetWorkbenchClientMaterialBatchDownloadLimit = 100
	assetWorkbenchClientMaterialBatchUpdateLimit   = 500
	assetWorkbenchClientMaterialBatchAsyncLimit    = 20000
)

type Service struct {
	cfg             Config
	repo            repo.AssetWorkbenchRepo
	userRepo        repo.UserRepo
	sessionRevoker  UserSessionRevoker
	notifications   ProfileCompletionNotifier
	tx              repo.TxRunner
	identity        WorkbenchIdentityRegistrar
	oss             objectStore
	renderer        baseservice.AssetPreviewRenderer
	systemAssets    SystemAssetSearcher
	systemDownloads SystemAssetDownloader
	systemPreviews  SystemAssetPreviewer
	nowFn           func() time.Time
	loc             *time.Location
	archiveCacheMu  sync.Mutex
	archiveCache    map[string]*archiveTempCacheEntry
}

type WorkbenchIdentityRegistrar interface {
	RegisterAssetWorkbenchUser(ctx context.Context, p baseservice.RegisterAssetWorkbenchUserParams) (*domain.AuthResult, *domain.AppError)
}

type UserSessionRevoker interface {
	RevokeActiveByUserID(ctx context.Context, tx repo.Tx, userID int64, at time.Time) (int64, error)
}

type ProfileCompletionNotifier interface {
	CreateDedupedNotification(ctx context.Context, userID int64, ntype domain.NotificationType, payload json.RawMessage, dedupeScope, dedupeKey string) (*domain.Notification, bool, error)
}

type SystemAssetSearcher interface {
	Search(ctx context.Context, query domain.AssetSearchQuery) (*assetcenter.SearchResult, *domain.AppError)
}

type SystemMaterialBrowser interface {
	BrowseMaterials(ctx context.Context, query assetcenter.MaterialBrowseQuery) (*assetcenter.MaterialBrowseResult, *domain.AppError)
}

type SystemAssetDetailer interface {
	GetDetail(ctx context.Context, assetID int64) (*assetcenter.AssetDetail, *domain.AppError)
}

type ExternalAssetDetailer interface {
	GetExternalDetail(ctx context.Context, externalID int64) (*assetcenter.AssetDetail, *domain.AppError)
}

type SystemAssetDownloader interface {
	DownloadLatest(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	BuildBatchDownloadManifest(ctx context.Context, assetIDs []int64, opts ...assetcenter.BatchDownloadOption) (*assetcenter.BatchDownloadManifest, *domain.AppError)
}

type SystemAssetPreviewer interface {
	GetAssetPreviewInfoByID(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError)
}

type ExternalAssetDownloader interface {
	DownloadExternal(ctx context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError)
	PreviewExternal(ctx context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError)
}

func NewService(cfg Config, opts ...Option) *Service {
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	if strings.TrimSpace(cfg.OSSPrefix) == "" {
		cfg.OSSPrefix = "asset-workbench"
	}
	if cfg.UploadSessionTTL <= 0 {
		cfg.UploadSessionTTL = 24 * time.Hour
	}
	if cfg.PreviewWorkerLeaseTTL <= 0 {
		cfg.PreviewWorkerLeaseTTL = 5 * time.Minute
	}
	if cfg.PreviewWorkerMaxAttempts <= 0 {
		cfg.PreviewWorkerMaxAttempts = 5
	}
	if cfg.BatchJobWorkerLeaseTTL <= 0 {
		cfg.BatchJobWorkerLeaseTTL = 10 * time.Minute
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	svc := &Service{
		cfg:   cfg,
		nowFn: time.Now,
		loc:   loc,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func WithRepository(workbenchRepo repo.AssetWorkbenchRepo, tx repo.TxRunner) Option {
	return func(s *Service) {
		s.repo = workbenchRepo
		s.tx = tx
	}
}

func WithUserRepository(userRepo repo.UserRepo) Option {
	return func(s *Service) {
		s.userRepo = userRepo
	}
}

func WithUserSessionRepository(sessionRepo UserSessionRevoker) Option {
	return func(s *Service) {
		s.sessionRevoker = sessionRepo
	}
}

func WithNotificationCreator(notifier ProfileCompletionNotifier) Option {
	return func(s *Service) {
		s.notifications = notifier
	}
}

func WithIdentityRegistrar(identity WorkbenchIdentityRegistrar) Option {
	return func(s *Service) {
		s.identity = identity
	}
}

func WithOSSDirect(oss *baseservice.OSSDirectService) Option {
	return func(s *Service) {
		s.oss = oss
	}
}

func WithPreviewRenderer(renderer baseservice.AssetPreviewRenderer) Option {
	return func(s *Service) {
		s.renderer = renderer
	}
}

func WithSystemAssetSearcher(searcher SystemAssetSearcher) Option {
	return func(s *Service) {
		s.systemAssets = searcher
		if downloader, ok := searcher.(SystemAssetDownloader); ok {
			s.systemDownloads = downloader
		}
		if previewer, ok := searcher.(SystemAssetPreviewer); ok {
			s.systemPreviews = previewer
		}
	}
}

func WithSystemAssetDownloader(downloader SystemAssetDownloader) Option {
	return func(s *Service) {
		s.systemDownloads = downloader
		if previewer, ok := downloader.(SystemAssetPreviewer); ok {
			s.systemPreviews = previewer
		}
	}
}

func WithSystemAssetPreviewer(previewer SystemAssetPreviewer) Option {
	return func(s *Service) {
		s.systemPreviews = previewer
	}
}

type BootstrapResponse struct {
	App                    string                        `json:"app"`
	Version                string                        `json:"version"`
	User                   domain.RequestActor           `json:"user"`
	Profile                *domain.AssetWorkbenchProfile `json:"profile,omitempty"`
	Timezone               string                        `json:"timezone"`
	OSSPrefix              string                        `json:"oss_prefix"`
	UploadSessionTTL       int64                         `json:"upload_session_ttl_seconds"`
	IsAdmin                bool                          `json:"is_admin"`
	Access                 *AssetWorkbenchAccessState    `json:"access,omitempty"`
	RoleLabels             []string                      `json:"role_labels"`
	Capabilities           []string                      `json:"capabilities"`
	SettlementItemTypes    []string                      `json:"settlement_item_types"`
	DeferredBusinessItems  []DeferredBusinessItem        `json:"deferred_business_items"`
	ArchitectureGuardrails []string                      `json:"architecture_guardrails"`
}

type AssetWorkbenchAccessState struct {
	MembershipStatus string        `json:"membership_status"`
	IsEnabled        bool          `json:"is_enabled"`
	IsAdminShell     bool          `json:"is_admin_shell"`
	AssetRoles       []domain.Role `json:"asset_roles"`
	RoleLabels       []string      `json:"role_labels"`
	Capabilities     []string      `json:"capabilities"`
	DeniedReason     string        `json:"denied_reason"`
}

type EntryResponse struct {
	State     string                     `json:"state"`
	Message   string                     `json:"message"`
	Access    *AssetWorkbenchAccessState `json:"access,omitempty"`
	Bootstrap *BootstrapResponse         `json:"bootstrap,omitempty"`
}

type DeferredBusinessItem struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type RegisterParams struct {
	Account       string `json:"account"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Phone         string `json:"phone"`
	Mobile        string `json:"mobile"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	WorkerType    string `json:"worker_type"`
	Province      string `json:"province"`
	City          string `json:"city"`
	IDCard        string `json:"id_card"`
	Gender        string `json:"gender"`
	AlipayAccount string `json:"alipay_account"`
}

type RegisterResponse struct {
	Auth    *domain.AuthResult            `json:"auth"`
	Profile *domain.AssetWorkbenchProfile `json:"profile"`
}

type UpsertProfileParams struct {
	WorkerType    string     `json:"worker_type"`
	JobGrade      string     `json:"job_grade"`
	RealName      string     `json:"real_name"`
	Phone         string     `json:"phone"`
	Province      string     `json:"province"`
	City          string     `json:"city"`
	IDCard        string     `json:"id_card"`
	Gender        string     `json:"gender"`
	AlipayAccount string     `json:"alipay_account"`
	OnboardedAt   *time.Time `json:"onboarded_at"`
	GradeHidden   bool       `json:"grade_hidden"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason"`
}

type UpdateMemberIdentityParams struct {
	Identity string `json:"identity"`
	Reason   string `json:"reason"`
}

type AccessRequestParams struct {
	IdentityType string `json:"identity_type"`
	Reason       string `json:"reason"`
}

type AccessOpenParams struct {
	UserID       int64         `json:"user_id"`
	Roles        []domain.Role `json:"roles"`
	IdentityType string        `json:"identity_type"`
	Reason       string        `json:"reason"`
}

type AccessDisableParams struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason"`
}

type UpdateMemberRolesParams struct {
	Roles  []domain.Role `json:"roles"`
	Reason string        `json:"reason"`
}

type AccountMergePreviewParams struct {
	SourceUserID    int64 `json:"source_user_id"`
	CanonicalUserID int64 `json:"canonical_user_id"`
}

type AccountMergeParams struct {
	SourceUserID    int64             `json:"source_user_id"`
	CanonicalUserID int64             `json:"canonical_user_id"`
	ProfileChoices  map[string]string `json:"profile_choices"`
	Reason          string            `json:"reason"`
}

type AccountMergePreview struct {
	SourceUserID    int64                    `json:"source_user_id"`
	CanonicalUserID int64                    `json:"canonical_user_id"`
	Conflicts       map[string]MergeConflict `json:"conflicts"`
	Counts          map[string]int64         `json:"counts"`
	AffectedMonths  []string                 `json:"affected_months"`
	SettlementNote  string                   `json:"settlement_note"`
}

type MergeConflict struct {
	Field          string `json:"field"`
	SourceValue    string `json:"source_value"`
	CanonicalValue string `json:"canonical_value"`
}

type CreateDifficultyClassParams struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

type UpdateDifficultyClassParams struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

type CreatePriceMatrixParams struct {
	WorkerType      string     `json:"worker_type"`
	JobGrade        string     `json:"job_grade"`
	DifficultyClass string     `json:"difficulty_class"`
	UnitPrice       float64    `json:"unit_price"`
	EffectiveFrom   time.Time  `json:"effective_from"`
	EffectiveTo     *time.Time `json:"effective_to"`
	Remark          string     `json:"remark"`
}

type CreateDeductionRuleParams struct {
	WorkerType      string     `json:"worker_type"`
	JobGrade        string     `json:"job_grade"`
	DifficultyClass string     `json:"difficulty_class"`
	DeductionAmount float64    `json:"deduction_amount"`
	EffectiveFrom   time.Time  `json:"effective_from"`
	EffectiveTo     *time.Time `json:"effective_to"`
	Remark          string     `json:"remark"`
}

type CreateWelfareRuleParams struct {
	RuleName      string          `json:"rule_name"`
	WorkerType    string          `json:"worker_type"`
	JobGrade      string          `json:"job_grade"`
	RuleType      string          `json:"rule_type"`
	Amount        float64         `json:"amount"`
	Config        json.RawMessage `json:"config_json"`
	EffectiveFrom time.Time       `json:"effective_from"`
	EffectiveTo   *time.Time      `json:"effective_to"`
	Remark        string          `json:"remark"`
}

type CreatePromoCouponParams struct {
	CouponCode      string          `json:"coupon_code"`
	CouponName      string          `json:"coupon_name"`
	Mode            string          `json:"mode"`
	Amount          *float64        `json:"amount"`
	Percent         *float64        `json:"percent"`
	Priority        int             `json:"priority"`
	WorkerType      string          `json:"worker_type"`
	JobGrade        string          `json:"job_grade"`
	DifficultyClass string          `json:"difficulty_class"`
	EligibleUserIDs json.RawMessage `json:"eligible_user_ids_json"`
	EligibleCodes   json.RawMessage `json:"eligible_codes_json"`
	EffectiveFrom   time.Time       `json:"effective_from"`
	EffectiveTo     *time.Time      `json:"effective_to"`
	Remark          string          `json:"remark"`
}

type SetCostRuleEnabledParams struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

type UpsertGroupParams struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type GroupMembersParams struct {
	UserIDs []int64 `json:"user_ids"`
}

type CreateUploadDirectoryParams struct {
	Name             string   `json:"name"`
	OSSPrefix        string   `json:"oss_prefix"`
	Description      string   `json:"description"`
	DifficultyClass  string   `json:"difficulty_class"`
	AllowedFileTypes []string `json:"allowed_file_types"`
	Enabled          *bool    `json:"enabled"`
	SortOrder        int      `json:"sort_order"`
}

type UpdateUploadDirectoryParams struct {
	Name             *string  `json:"name"`
	OSSPrefix        *string  `json:"oss_prefix"`
	Description      *string  `json:"description"`
	DifficultyClass  *string  `json:"difficulty_class"`
	AllowedFileTypes []string `json:"allowed_file_types"`
	Enabled          *bool    `json:"enabled"`
	SortOrder        *int     `json:"sort_order"`
}

type CreateClientMaterialParams struct {
	AssetID     int64  `json:"asset_id"`
	SourceType  string `json:"source_type"`
	SourceRef   string `json:"source_ref"`
	ResourceID  string `json:"resource_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

type UpdateClientMaterialParams struct {
	AssetID     *int64  `json:"asset_id"`
	SourceType  *string `json:"source_type"`
	SourceRef   *string `json:"source_ref"`
	ResourceID  *string `json:"resource_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

type ClientMaterialBatchDownloadParams struct {
	MaterialIDs []int64 `json:"material_ids"`
	NamingMode  string  `json:"naming_mode,omitempty"`
}

type BatchUpdateClientMaterialsParams struct {
	Action         string                            `json:"action"`
	Items          []CreateClientMaterialParams      `json:"items"`
	Folders        []ClientMaterialBatchFolderParams `json:"folders"`
	Query          string                            `json:"query"`
	Source         string                            `json:"source"`
	FormatCategory string                            `json:"format_category"`
	BusinessLane   string                            `json:"business_lane"`
	SelectionScope string                            `json:"selection_scope"`
}

type ClientMaterialBatchFolderParams struct {
	Path            string `json:"path"`
	Source          string `json:"source"`
	FormatCategory  string `json:"format_category"`
	BusinessLane    string `json:"business_lane"`
	IncludeChildren bool   `json:"include_children"`
}

type ClientMaterialBatchUpdateResult struct {
	Requested     int                                    `json:"requested"`
	Created       int                                    `json:"created"`
	Updated       int                                    `json:"updated"`
	Enabled       int                                    `json:"enabled"`
	Disabled      int                                    `json:"disabled"`
	Removed       int                                    `json:"removed"`
	Skipped       int                                    `json:"skipped"`
	Failed        int                                    `json:"failed"`
	AsyncRequired bool                                   `json:"async_required,omitempty"`
	JobID         string                                 `json:"job_id,omitempty"`
	Job           *domain.AssetWorkbenchBatchJob         `json:"job,omitempty"`
	Message       string                                 `json:"message,omitempty"`
	Items         []*domain.AssetWorkbenchClientMaterial `json:"items,omitempty"`
	Failures      []ClientMaterialBatchUpdateFailure     `json:"failures,omitempty"`
}

type ClientMaterialBatchUpdateFailure struct {
	Index      int    `json:"index"`
	AssetID    int64  `json:"asset_id,omitempty"`
	SourceType string `json:"source_type,omitempty"`
	SourceRef  string `json:"source_ref,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Reason     string `json:"reason"`
}

type BatchJobListResult struct {
	Items []*domain.AssetWorkbenchBatchJob `json:"items"`
	Total int64                            `json:"total"`
	Page  int                              `json:"page"`
	Size  int                              `json:"size"`
}

type ClientMaterialBatchDownloadManifest struct {
	Items        []ClientMaterialBatchDownloadItem    `json:"items"`
	Failures     []ClientMaterialBatchDownloadFailure `json:"failures,omitempty"`
	SuccessCount int                                  `json:"success_count"`
	FailureCount int                                  `json:"failure_count"`
	TotalSize    int64                                `json:"total_size"`
	ExpiresAt    *time.Time                           `json:"expires_at,omitempty"`
}

type ClientMaterialBatchDownloadItem struct {
	MaterialID  int64      `json:"material_id"`
	AssetID     int64      `json:"asset_id"`
	SourceType  string     `json:"source_type"`
	SourceRef   string     `json:"source_ref"`
	Filename    string     `json:"filename"`
	FileSize    int64      `json:"file_size"`
	MimeType    string     `json:"mime_type,omitempty"`
	DownloadURL string     `json:"download_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type ClientMaterialBatchDownloadFailure struct {
	MaterialID int64  `json:"material_id"`
	AssetID    int64  `json:"asset_id,omitempty"`
	SourceType string `json:"source_type,omitempty"`
	SourceRef  string `json:"source_ref,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Reason     string `json:"reason"`
}

type ClientMaterialSearchParams struct {
	Query    string `json:"q"`
	SKU      string `json:"sku"`
	Creator  string `json:"creator"`
	Admin    bool   `json:"admin"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type ClientMaterialSearchResult struct {
	Items []*domain.AssetWorkbenchClientMaterial `json:"items"`
	Total int64                                  `json:"total"`
	Page  int                                    `json:"page"`
	Size  int                                    `json:"size"`
}

type MaterialGroupSearchParams struct {
	Query          string `json:"q"`
	Source         string `json:"source"`
	FormatCategory string `json:"format_category"`
	BusinessLane   string `json:"business_lane"`
	Page           int    `json:"page"`
	PageSize       int    `json:"page_size"`
}

type MaterialGroupRow struct {
	GroupKey     string                     `json:"group_key"`
	GroupCode    string                     `json:"group_code"`
	GroupType    string                     `json:"group_type"`
	Title        string                     `json:"title"`
	SourceType   string                     `json:"source_type"`
	FileTotal    int64                      `json:"file_total"`
	PreviewFiles []*assetcenter.AssetDetail `json:"preview_files,omitempty"`
}

type MaterialGroupSearchResult struct {
	Items []*MaterialGroupRow `json:"items"`
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
}

type MaterialGroupFilesResult struct {
	GroupKey string                     `json:"group_key"`
	Items    []*assetcenter.AssetDetail `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	Size     int                        `json:"size"`
}

type CreateUploadSessionParams struct {
	OriginalFilename      string `json:"original_filename"`
	FileSize              int64  `json:"file_size"`
	MimeType              string `json:"mime_type"`
	FileHash              string `json:"file_hash"`
	UploadDirectoryID     int64  `json:"upload_directory_id"`
	UploadBatchID         string `json:"upload_batch_id"`
	RelativePath          string `json:"relative_path"`
	IsFolderUpload        bool   `json:"is_folder_upload"`
	ExpectedBusinessMonth string `json:"expected_business_month"`
}

type CompleteUploadSessionParams struct {
	Parts []baseservice.OSSCompletePart `json:"parts"`
}

type UploadSessionResponse struct {
	Session *domain.AssetWorkbenchUploadSession `json:"session"`
	Plan    interface{}                         `json:"plan,omitempty"`
}

type CreateSubmissionParams struct {
	Notes                 string                       `json:"notes"`
	ExpectedBusinessMonth string                       `json:"expected_business_month"`
	MonthRolloverAck      bool                         `json:"month_rollover_ack"`
	BusinessMonthOverride string                       `json:"business_month_override"`
	Items                 []CreateSubmissionItemParams `json:"items"`
}

type CreateSubmissionItemParams struct {
	OrderNo          string   `json:"order_no"`
	TemplateID       int64    `json:"template_id"`
	DifficultyClass  string   `json:"difficulty_class"`
	Finalized        bool     `json:"finalized"`
	PageCount        int      `json:"page_count"`
	ItemCount        int      `json:"item_count"`
	UploadSessionIDs []string `json:"upload_session_ids"`
}

type UpdateSubmissionItemQCParams struct {
	QCStatus string `json:"qc_status"`
	Reason   string `json:"reason"`
}

type SubmissionItemQCImportResult struct {
	Updated  []*domain.AssetWorkbenchSubmissionItem `json:"updated"`
	Failures []SubmissionItemQCImportFailure        `json:"failures"`
}

type SubmissionItemQCImportFailure struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type UpdateSubmissionItemParams struct {
	OrderNo         *string `json:"order_no"`
	DifficultyClass *string `json:"difficulty_class"`
	Finalized       *bool   `json:"finalized"`
	PageCount       *int    `json:"page_count"`
	Reason          string  `json:"reason"`
}

type VoidSubmissionParams struct {
	Reason string `json:"reason"`
}

type VoidSubmissionItemParams struct {
	Reason string `json:"reason"`
}

type RepriceSubmissionItemParams struct {
	Reason string `json:"reason"`
}

type BatchMoveFilesParams struct {
	FileIDs           []int64 `json:"file_ids"`
	UploadDirectoryID int64   `json:"upload_directory_id"`
	Reason            string  `json:"reason"`
}

type BatchDeleteFilesParams struct {
	FileIDs []int64 `json:"file_ids"`
	Reason  string  `json:"reason"`
}

type BatchFileMutationResult struct {
	Files    []*domain.AssetWorkbenchSubmissionFile `json:"files,omitempty"`
	Deleted  []int64                                `json:"deleted,omitempty"`
	Failures []BatchFileMutationFailure             `json:"failures,omitempty"`
}

type BatchFileMutationFailure struct {
	FileID int64  `json:"file_id"`
	Reason string `json:"reason"`
}

type SubmissionDetail struct {
	Submission *domain.AssetWorkbenchSubmission `json:"submission"`
	Items      []SubmissionItemDetail           `json:"items"`
}

type SubmissionItemDetail struct {
	Item  *domain.AssetWorkbenchSubmissionItem   `json:"item"`
	Files []*domain.AssetWorkbenchSubmissionFile `json:"files"`
}

type FilePreviewMeta struct {
	FileID           int64      `json:"file_id"`
	Status           string     `json:"status"`
	Preparing        bool       `json:"preparing"`
	PreviewURL       string     `json:"preview_url,omitempty"`
	DownloadURL      string     `json:"download_url,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	MimeType         string     `json:"mime_type,omitempty"`
	Filename         string     `json:"filename,omitempty"`
	PreviewAvailable bool       `json:"preview_available"`
	Error            string     `json:"error,omitempty"`
}

type SystemAssetPreviewMeta struct {
	AssetID          int64      `json:"asset_id"`
	SourceType       string     `json:"source_type,omitempty"`
	SourceRef        string     `json:"source_ref,omitempty"`
	Status           string     `json:"status"`
	Preparing        bool       `json:"preparing"`
	PreviewURL       string     `json:"preview_url,omitempty"`
	DownloadURL      string     `json:"download_url,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	MimeType         string     `json:"mime_type,omitempty"`
	Filename         string     `json:"filename,omitempty"`
	PreviewAvailable bool       `json:"preview_available"`
}

type FileDownloadMeta struct {
	FileID      int64     `json:"file_id"`
	Filename    string    `json:"filename"`
	MimeType    string    `json:"mime_type"`
	FileSize    int64     `json:"file_size"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type FileBatchDownloadManifest struct {
	Items    []FileDownloadMeta      `json:"items"`
	Failures []FileBatchDownloadFail `json:"failures,omitempty"`
}

type FileBatchDownloadFail struct {
	FileID int64  `json:"file_id"`
	Reason string `json:"reason"`
}

type BatchDownloadFilesParams struct {
	FileIDs []int64 `json:"file_ids"`
}

type SystemSearchResult struct {
	Items []*assetcenter.AssetDetail `json:"items"`
	Total int64                      `json:"total"`
	Page  int                        `json:"page"`
	Size  int                        `json:"size"`
}

type OverviewSearchParams struct {
	Query       string
	Scope       string
	Creator     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Page        int
	PageSize    int
}

type OverviewSearchResult struct {
	Items []*domain.AssetWorkbenchOverviewRow `json:"items"`
	Total int64                               `json:"total"`
	Page  int                                 `json:"page"`
	Size  int                                 `json:"size"`
}

type SystemAssetBatchDownloadParams struct {
	AssetIDs   []int64 `json:"asset_ids"`
	NamingMode string  `json:"naming_mode,omitempty"`
}

type ImportErrorRecordsParams struct {
	BusinessMonth    string                   `json:"business_month"`
	OriginalFilename string                   `json:"original_filename"`
	Records          []ImportErrorRecordInput `json:"records"`
}

type ImportErrorRecordInput struct {
	PayeeUserID      *int64          `json:"payee_user_id"`
	PayeeName        string          `json:"payee_name"`
	OrderNo          string          `json:"order_no"`
	DifficultyClass  string          `json:"difficulty_class"`
	OccurredDate     string          `json:"occurred_date"`
	ErrorCount       int             `json:"error_count"`
	IssueDescription string          `json:"issue_description"`
	SourceType       string          `json:"source_type"`
	HandlingMethod   string          `json:"handling_method"`
	ReporterName     string          `json:"reporter_name"`
	Remark           string          `json:"remark"`
	RawPayload       json.RawMessage `json:"raw_payload_json,omitempty"`
}

type errorRecordImportMatch struct {
	Status           string
	SubmissionItemID *int64
	CandidateItemIDs []int64
	PayeeUserID      *int64
	DifficultyClass  string
	OccurredDate     *time.Time
	Reason           string
	CandidateUserIDs []int64
}

type SettlementPreview struct {
	BusinessMonth string                 `json:"business_month"`
	Rows          []SettlementPreviewRow `json:"rows"`
	Totals        SettlementPreviewRow   `json:"totals"`
	PayrollRows   []SettlementPayrollRow `json:"payroll_rows"`
}

type SettlementPreviewRow struct {
	PayeeUserID      int64   `json:"payee_user_id"`
	PayeeName        string  `json:"payee_name,omitempty"`
	WorkerType       string  `json:"worker_type,omitempty"`
	ItemCount        int     `json:"item_count"`
	PageCount        int     `json:"page_count"`
	GrossAmount      float64 `json:"gross_amount"`
	ErrorCount       int     `json:"error_count"`
	DeductionAmount  float64 `json:"deduction_amount"`
	WelfareAmount    float64 `json:"welfare_amount"`
	SupplementAmount float64 `json:"supplement_amount"`
	NetAmount        float64 `json:"net_amount"`
}

type SettlementPayrollRow struct {
	PayeeUserID      int64   `json:"payee_user_id"`
	PayeeName        string  `json:"payee_name,omitempty"`
	WorkerType       string  `json:"worker_type,omitempty"`
	BusinessMonth    string  `json:"business_month"`
	RowType          string  `json:"row_type"`
	ItemCount        int     `json:"item_count"`
	PageCount        int     `json:"page_count"`
	GrossAmount      float64 `json:"gross_amount"`
	ErrorCount       int     `json:"error_count"`
	DeductionAmount  float64 `json:"deduction_amount"`
	WelfareAmount    float64 `json:"welfare_amount"`
	SupplementAmount float64 `json:"supplement_amount"`
	AdjustmentAmount float64 `json:"adjustment_amount"`
	NetAmount        float64 `json:"net_amount"`
}

type SettlementReport struct {
	BusinessMonth      string                `json:"business_month"`
	DifficultyClasses  []string              `json:"difficulty_classes"`
	Rows               []SettlementReportRow `json:"rows"`
	Totals             SettlementReportRow   `json:"totals"`
	GeneratedAt        time.Time             `json:"generated_at"`
	OrderCountPolicy   string                `json:"order_count_policy"`
	SettlementDataMode string                `json:"settlement_data_mode"`
}

type SettlementReportRow struct {
	PayeeUserID       int64                              `json:"payee_user_id"`
	BusinessMonth     string                             `json:"business_month"`
	RowType           string                             `json:"row_type"`
	CreatorName       string                             `json:"creator_name"`
	JobGrade          string                             `json:"job_grade"`
	CreatedDate       string                             `json:"created_date"`
	CreatedDateEnd    string                             `json:"created_date_end"`
	OrderCount        int                                `json:"order_count"`
	ItemCount         int                                `json:"item_count"`
	PageCount         int                                `json:"page_count"`
	GrossAmount       float64                            `json:"gross_amount"`
	ErrorCount        int                                `json:"error_count"`
	DeductionAmount   float64                            `json:"deduction_amount"`
	WelfareAmount     float64                            `json:"welfare_amount"`
	SupplementAmount  float64                            `json:"supplement_amount"`
	NetAmount         float64                            `json:"net_amount"`
	ErrorRate         float64                            `json:"error_rate"`
	PageCountShare    float64                            `json:"page_count_share"`
	ErrorCountShare   float64                            `json:"error_count_share"`
	MonthAmountShare  float64                            `json:"month_amount_share"`
	DifficultyMetrics []SettlementReportDifficultyMetric `json:"difficulty_metrics"`
}

type SettlementReportDifficultyMetric struct {
	DifficultyClass     string  `json:"difficulty_class"`
	OrderCount          int     `json:"order_count"`
	ItemCount           int     `json:"item_count"`
	PageCount           int     `json:"page_count"`
	GrossAmount         float64 `json:"gross_amount"`
	ErrorCount          int     `json:"error_count"`
	DeductionAmount     float64 `json:"deduction_amount"`
	ErrorRate           float64 `json:"error_rate"`
	PageCountShare      float64 `json:"page_count_share"`
	ErrorCountShare     float64 `json:"error_count_share"`
	MonthPageCountShare float64 `json:"month_page_count_share"`
}

type MySettlementResponse struct {
	CurrentMonth         string                                       `json:"current_month"`
	EstimatedNetAmount   float64                                      `json:"estimated_net_amount"`
	Months               []MySettlementMonthRow                       `json:"months"`
	SupplementPermission *domain.AssetWorkbenchSupplementPermission   `json:"supplement_permission,omitempty"`
	Supplements          []*domain.AssetWorkbenchSettlementSupplement `json:"supplements"`
}

type MySettlementMonthRow struct {
	BusinessMonth    string  `json:"business_month"`
	ItemCount        int     `json:"item_count"`
	PageCount        int     `json:"page_count"`
	GrossAmount      float64 `json:"gross_amount"`
	DeductionAmount  float64 `json:"deduction_amount"`
	WelfareAmount    float64 `json:"welfare_amount"`
	SupplementAmount float64 `json:"supplement_amount"`
	AdjustmentAmount float64 `json:"adjustment_amount"`
	NetAmount        float64 `json:"net_amount"`
	Confirmed        bool    `json:"confirmed"`
}

type welfareSettlementLine struct {
	PayeeUserID   int64
	RuleID        int64
	RuleName      string
	BusinessMonth string
	Amount        float64
	Snapshot      json.RawMessage
}

type promoApplication struct {
	Coupon       *domain.AssetWorkbenchPromoCoupon
	UnitPrice    float64
	Snapshot     json.RawMessage
	AppliedLabel string
}

type CreateSettlementSupplementParams struct {
	PayeeUserID      int64    `json:"payee_user_id"`
	BusinessMonth    string   `json:"business_month"`
	OrderNo          string   `json:"order_no"`
	SupplementDate   string   `json:"supplement_date,omitempty"`
	DifficultyClass  string   `json:"difficulty_class"`
	Finalized        bool     `json:"finalized"`
	PageCount        int      `json:"page_count"`
	GrossAmount      float64  `json:"gross_amount"`
	Status           string   `json:"status"`
	UploadSessionIDs []string `json:"upload_session_ids,omitempty"`
}

type BatchDeleteSettlementSupplementsParams struct {
	SupplementIDs []int64 `json:"supplement_ids"`
	Reason        string  `json:"reason"`
}

type BatchDeleteSettlementSupplementsResult struct {
	DeletedIDs  []int64                                      `json:"deleted_ids"`
	Supplements []*domain.AssetWorkbenchSettlementSupplement `json:"supplements"`
}

type SettlementSupplementImportResult struct {
	Created  []*domain.AssetWorkbenchSettlementSupplement `json:"created"`
	Failures []SettlementSupplementImportFailure          `json:"failures"`
}

type SettlementSupplementImportFailure struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type UpsertSupplementPermissionParams struct {
	PayeeUserID   int64  `json:"payee_user_id"`
	BusinessMonth string `json:"business_month"`
	Enabled       bool   `json:"enabled"`
	Reason        string `json:"reason"`
}

type CreateSettlementAdjustmentParams struct {
	BatchID        int64           `json:"batch_id"`
	PayeeUserID    int64           `json:"payee_user_id"`
	AdjustmentType string          `json:"adjustment_type"`
	Direction      string          `json:"direction"`
	Amount         float64         `json:"amount"`
	Reason         string          `json:"reason"`
	Payload        json.RawMessage `json:"payload_json"`
}

type SettlementBatchDetail struct {
	Batch       *domain.AssetWorkbenchSettlementBatch  `json:"batch"`
	Items       []*domain.AssetWorkbenchSettlementItem `json:"items"`
	PayrollRows []SettlementPayrollRow                 `json:"payroll_rows"`
}

type UpsertSavedViewParams struct {
	ViewType  string          `json:"view_type"`
	ViewName  string          `json:"view_name"`
	Config    json.RawMessage `json:"config_json"`
	IsDefault bool            `json:"is_default"`
}

func (s *Service) Register(ctx context.Context, params RegisterParams) (*RegisterResponse, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.identity == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench registration is not configured.", nil)
	}
	account := strings.TrimSpace(firstNonEmpty(params.Account, params.Username))
	name := strings.TrimSpace(firstNonEmpty(params.Name, params.DisplayName))
	phone := strings.TrimSpace(firstNonEmpty(params.Phone, params.Mobile))
	if account == "" || name == "" || phone == "" || strings.TrimSpace(params.Password) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "account, name, phone and password are required.", nil)
	}
	workerType := normalizeWorkerType(params.WorkerType)
	if workerType == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择人员类型。", nil)
	}
	onboardedAt := s.nowFn().UTC()
	profileParams := UpsertProfileParams{
		WorkerType:    workerType,
		JobGrade:      "",
		RealName:      name,
		Phone:         phone,
		Province:      strings.TrimSpace(params.Province),
		City:          strings.TrimSpace(params.City),
		IDCard:        strings.TrimSpace(params.IDCard),
		Gender:        strings.TrimSpace(params.Gender),
		AlipayAccount: strings.TrimSpace(params.AlipayAccount),
		OnboardedAt:   &onboardedAt,
		Status:        domain.AssetWorkbenchProfileStatusPending,
		Reason:        "asset workbench self-registration",
	}
	profile, appErr := s.normalizeProfile(0, 0, profileParams)
	if appErr != nil {
		return nil, appErr
	}
	if appErr = validateRequiredClientProfile(profile); appErr != nil {
		return nil, appErr
	}
	auth, appErr := s.identity.RegisterAssetWorkbenchUser(ctx, baseservice.RegisterAssetWorkbenchUserParams{
		Username:    account,
		DisplayName: name,
		Mobile:      phone,
		Email:       strings.TrimSpace(params.Email),
		Password:    params.Password,
	})
	if appErr != nil {
		return nil, appErr
	}
	if auth == nil || auth.User == nil || auth.User.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench registration did not create a user.", nil)
	}
	actor := domain.RequestActor{
		ID:       auth.User.ID,
		Username: auth.User.Username,
		Roles:    auth.User.Roles,
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
	profile.UserID = auth.User.ID
	profile.CreatedBy = &actor.ID
	profile.UpdatedBy = &actor.ID
	profile, appErr = s.upsertProfile(ctx, actor, profile, profileParams.Reason, true)
	if appErr != nil {
		return nil, appErr
	}
	if s.tx != nil {
		if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
			_, err := s.repo.OpenMembership(ctx, tx, repo.AssetWorkbenchAccessOpenParams{
				UserID:       auth.User.ID,
				Status:       domain.AppMembershipStatusActive,
				IdentityType: domain.AppMembershipIdentityExternal,
				Source:       domain.AppMembershipSourceAssetRegistered,
				OpenedBy:     auth.User.ID,
			})
			return err
		}); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to open asset workbench access for registered user.", err.Error())
		}
	}
	return &RegisterResponse{Auth: auth, Profile: profile}, nil
}

func (s *Service) Entry(ctx context.Context, actor domain.RequestActor) (*EntryResponse, *domain.AppError) {
	if actor.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil)
	}
	access, appErr := s.ResolveAssetWorkbenchAccess(ctx, actor)
	if appErr != nil {
		return nil, appErr
	}
	if access.IsEnabled {
		bootstrap, appErr := s.buildBootstrap(ctx, actor, access)
		if appErr != nil {
			return nil, appErr
		}
		return &EntryResponse{State: "ready", Message: "工作台已开通", Access: access, Bootstrap: bootstrap}, nil
	}
	state := access.MembershipStatus
	if state == "" {
		state = "not_member"
	}
	message := access.DeniedReason
	if message == "" {
		switch state {
		case domain.AppMembershipStatusPending:
			message = "资产工作台开通申请正在处理。"
		case domain.AppMembershipStatusDisabled:
			message = "资产工作台访问已停用。"
		case domain.AppMembershipStatusMerged:
			message = "该账号已合并，请使用主账号登录。"
		default:
			message = "该账号尚未开通资产工作台。"
		}
	}
	return &EntryResponse{State: state, Message: message, Access: access}, nil
}

func (s *Service) Bootstrap(ctx context.Context, actor domain.RequestActor) (*BootstrapResponse, *domain.AppError) {
	access, appErr := s.ResolveAssetWorkbenchAccess(ctx, actor)
	if appErr != nil {
		return nil, appErr
	}
	if !access.IsEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, firstNonEmpty(access.DeniedReason, "Asset workbench access is not active."), map[string]string{"membership_status": access.MembershipStatus})
	}
	return s.buildBootstrap(ctx, actor, access)
}

func (s *Service) buildBootstrap(ctx context.Context, actor domain.RequestActor, access *AssetWorkbenchAccessState) (*BootstrapResponse, *domain.AppError) {
	var profile *domain.AssetWorkbenchProfile
	if s.repo != nil && actor.ID > 0 {
		item, err := s.repo.GetProfileByUserID(ctx, actor.ID)
		if err == nil {
			refreshProfileCompletion(item)
			profile = item
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench profile.", err.Error())
		}
	}
	return &BootstrapResponse{
		App:                 "asset_workbench",
		Version:             "v1",
		User:                actor,
		Profile:             profile,
		Timezone:            s.cfg.Timezone,
		OSSPrefix:           s.cfg.OSSPrefix,
		UploadSessionTTL:    int64(s.cfg.UploadSessionTTL.Seconds()),
		IsAdmin:             access != nil && access.IsAdminShell,
		Access:              access,
		RoleLabels:          roleLabelsForActor(actor),
		Capabilities:        assetWorkbenchCapabilities(actor),
		SettlementItemTypes: domain.DefaultAssetWorkbenchSettlementItemTypes(),
		DeferredBusinessItems: []DeferredBusinessItem{
			{
				Key:    "complex_welfare_rules",
				Status: "deferred",
				Note:   "v1 reserves rule maintenance and manual lines; automatic attendance and no-error bonuses are added later.",
			},
			{
				Key:    "coupon_combiner",
				Status: "deferred",
				Note:   "v1 uses one fixed coupon winner; stacked coupon calculation is added later.",
			},
		},
		ArchitectureGuardrails: []string{
			"submission_items are the settlement minimum unit",
			"deductions are frozen at settlement time, not at submission time",
			"welfare is generated by payee_user_id plus business_month",
			"payroll_rows always emit normal piecework and supplement piecework rows per payee/month",
			"business months use Asia/Shanghai while persisted timestamps stay UTC",
		},
	}, nil
}

func (s *Service) ResolveAssetWorkbenchAccess(ctx context.Context, actor domain.RequestActor) (*AssetWorkbenchAccessState, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if actor.ID <= 0 {
		return &AssetWorkbenchAccessState{MembershipStatus: "not_member", DeniedReason: "Authentication required."}, nil
	}
	membership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, actor.ID)
	if errors.Is(err, sql.ErrNoRows) && s.tx != nil {
		source := ""
		reason := ""
		identityType := domain.AppMembershipIdentityStaff
		switch {
		case actorHasAny(actor, domain.RoleSuperAdmin, domain.RoleHRAdmin):
			source = domain.AppMembershipSourceGlobalAdminAuto
			reason = "全局管理角色自动开通资产工作台"
		case actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement):
			source = domain.AppMembershipSourceMainOpsOpened
			reason = "已有资产工作台角色，自动补齐开通状态"
		}
		if source != "" {
			var opened *domain.AppMembership
			if txErr := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
				openedBy := actor.ID
				var upsertErr error
				opened, upsertErr = s.repo.UpsertMembership(ctx, tx, &domain.AppMembership{
					AppCode:      domain.AssetWorkbenchAppCode,
					UserID:       actor.ID,
					Status:       domain.AppMembershipStatusActive,
					IdentityType: identityType,
					Source:       source,
					OpenedBy:     &openedBy,
				})
				if upsertErr != nil {
					return upsertErr
				}
				return s.appendIdentityEvent(ctx, tx, actor.ID, actor.ID, domain.AppIdentityActionAccessOpened, nil, opened, reason)
			}); txErr != nil {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to auto-open asset workbench access.", txErr.Error())
			}
			membership, err = s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, actor.ID)
		}
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to resolve asset workbench access.", err.Error())
	}
	status := "not_member"
	if membership != nil {
		status = membership.Status
	}
	access := &AssetWorkbenchAccessState{
		MembershipStatus: status,
		AssetRoles:       assetRolesFromActor(actor),
		RoleLabels:       roleLabelsForActor(actor),
		Capabilities:     assetWorkbenchCapabilities(actor),
		IsAdminShell:     isAssetWorkbenchAdmin(actor),
	}
	if status == domain.AppMembershipStatusActive {
		access.IsEnabled = true
		return access, nil
	}
	switch status {
	case domain.AppMembershipStatusPending:
		access.DeniedReason = "资产工作台开通申请正在处理。"
	case domain.AppMembershipStatusDisabled:
		access.DeniedReason = "资产工作台访问已停用。"
	case domain.AppMembershipStatusMerged:
		access.DeniedReason = "该账号已合并，请使用主账号登录。"
	default:
		access.DeniedReason = "该账号尚未开通资产工作台。"
	}
	return access, nil
}

func (s *Service) UpsertMyProfile(ctx context.Context, actor domain.RequestActor, params UpsertProfileParams) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if actor.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil)
	}
	existing, err := s.repo.GetProfileByUserID(ctx, actor.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench profile.", err.Error())
	}
	params = preserveSelfManagedProfileFields(params, existing, s.nowFn().UTC())
	profile, appErr := s.normalizeProfile(actor.ID, actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr = validateRequiredClientProfile(profile); appErr != nil {
		return nil, appErr
	}
	return s.upsertProfile(ctx, actor, profile, params.Reason, false)
}

func (s *Service) HRUpsertProfile(ctx context.Context, actor domain.RequestActor, userID int64, params UpsertProfileParams) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only HR or settlement roles can update workbench profiles.", nil)
	}
	if userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required.", nil)
	}
	existing, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench profile.", err.Error())
	}
	params = preserveExistingProfilePII(params, existing)
	profile, appErr := s.normalizeProfile(userID, actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	return s.upsertProfile(ctx, actor, profile, params.Reason, true)
}

func preserveExistingProfilePII(params UpsertProfileParams, existing *domain.AssetWorkbenchProfile) UpsertProfileParams {
	if existing == nil {
		return params
	}
	if strings.TrimSpace(params.Phone) == "" && existing.Phone != nil {
		params.Phone = *existing.Phone
	}
	if strings.TrimSpace(params.IDCard) == "" && existing.IDCard != nil {
		params.IDCard = *existing.IDCard
	}
	if strings.TrimSpace(params.AlipayAccount) == "" {
		params.AlipayAccount = existing.AlipayAccount
	}
	if strings.TrimSpace(params.Gender) == "" {
		params.Gender = existing.Gender
	}
	if params.OnboardedAt == nil {
		params.OnboardedAt = existing.OnboardedAt
	}
	return params
}

func preserveSelfManagedProfileFields(params UpsertProfileParams, existing *domain.AssetWorkbenchProfile, now time.Time) UpsertProfileParams {
	params = preserveExistingProfilePII(params, existing)
	if existing == nil {
		if strings.TrimSpace(params.WorkerType) == "" {
			params.WorkerType = domain.AssetWorkbenchWorkerTypeParttime
		}
		params.JobGrade = ""
		if params.OnboardedAt == nil {
			params.OnboardedAt = &now
		}
		if strings.TrimSpace(params.Status) == "" {
			params.Status = domain.AssetWorkbenchProfileStatusPending
		}
		params.GradeHidden = false
		return params
	}
	if strings.TrimSpace(params.WorkerType) == "" {
		params.WorkerType = existing.WorkerType
	}
	params.JobGrade = existing.JobGrade
	params.OnboardedAt = existing.OnboardedAt
	params.GradeHidden = existing.GradeHidden
	params.Status = existing.Status
	return params
}

func (s *Service) ListProfiles(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchProfileFilter) ([]*domain.AssetWorkbenchProfile, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only HR or settlement roles can list workbench profiles.", nil)
	}
	items, total, err := s.repo.ListProfiles(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench profiles.", err.Error())
	}
	return maskProfileListPII(items), total, nil
}

func (s *Service) GetProfile(ctx context.Context, actor domain.RequestActor, userID int64) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only HR or settlement roles can view full workbench profiles.", nil)
	}
	if userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required.", nil)
	}
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, mapRepoReadError(err, "Asset workbench profile not found.", "Failed to load asset workbench profile.")
	}
	refreshProfileCompletion(profile)
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventProfilePIIViewed, domain.AssetWorkbenchEntityProfile, &profile.ID, nil, nil, "查看人员完整资料")
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to record profile access.", err.Error())
	}
	return profile, nil
}

func (s *Service) ListMembers(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can list workbench members.", nil)
	}
	items, total, err := s.repo.ListMembers(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench members.", err.Error())
	}
	for _, item := range items {
		decorateMemberForActor(item, actor)
	}
	return items, total, nil
}

func (s *Service) SearchPeople(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can search workbench people.", nil)
	}
	if strings.TrimSpace(filter.Scope) == "all_users" && !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can search all users.", nil)
	}
	items, total, err := s.repo.SearchPeople(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to search asset workbench people.", err.Error())
	}
	for _, item := range items {
		decorateMemberForActor(item, actor)
	}
	return items, total, nil
}

func (s *Service) UpdateMemberIdentity(ctx context.Context, actor domain.RequestActor, userID int64, params UpdateMemberIdentityParams) (*domain.AssetWorkbenchMember, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeUploadEndpointDeprecated, "This endpoint is deprecated. Use /asset-workbench/members/{user_id}/roles.", nil)
}

func (s *Service) RequestAccess(ctx context.Context, actor domain.RequestActor, params AccessRequestParams) (*domain.AppMembership, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if actor.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil)
	}
	identityType := normalizeMembershipIdentityType(params.IdentityType)
	var membership *domain.AppMembership
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		membership, err = s.repo.RequestMembership(ctx, tx, domain.AssetWorkbenchAppCode, actor.ID, identityType)
		if err != nil {
			return err
		}
		return s.appendIdentityEvent(ctx, tx, actor.ID, actor.ID, domain.AppIdentityActionAccessRequested, nil, membership, strings.TrimSpace(params.Reason))
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to request asset workbench access.", err.Error())
	}
	return membership, nil
}

func (s *Service) OpenAccess(ctx context.Context, actor domain.RequestActor, params AccessOpenParams) (*domain.AppMembership, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.userRepo == nil || s.tx == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench user role repository is not configured.", nil)
	}
	if params.UserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required.", nil)
	}
	roles := normalizeAssetWorkbenchRoleSet(params.Roles, true)
	grantsManagement := containsManagementAssetRole(roles)
	if grantsManagement && !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can grant asset workbench management roles.", nil)
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can open submitter access.", nil)
	}
	currentMembership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, params.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load membership.", err.Error())
	}
	if currentMembership != nil && currentMembership.Status == domain.AppMembershipStatusDisabled && !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can restore disabled workbench access.", nil)
	}
	currentRoles, err := s.userRepo.ListRoles(ctx, params.UserID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load user roles.", err.Error())
	}
	nextRoles := mergeAssetWorkbenchRoles(currentRoles, roles)
	var membership *domain.AppMembership
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.ReplaceRoles(ctx, tx, params.UserID, nextRoles); err != nil {
			return err
		}
		var err error
		membership, err = s.repo.OpenMembership(ctx, tx, repo.AssetWorkbenchAccessOpenParams{
			UserID:       params.UserID,
			Status:       domain.AppMembershipStatusActive,
			IdentityType: normalizeMembershipIdentityType(params.IdentityType),
			Source:       domain.AppMembershipSourceMainOpsOpened,
			OpenedBy:     actor.ID,
		})
		if err != nil {
			return err
		}
		return s.appendIdentityEvent(ctx, tx, actor.ID, params.UserID, domain.AppIdentityActionAccessOpened, map[string]interface{}{
			"membership": currentMembership,
			"roles":      currentRoles,
		}, map[string]interface{}{
			"membership": membership,
			"roles":      nextRoles,
		}, strings.TrimSpace(params.Reason))
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to open asset workbench access.", err.Error())
	}
	return membership, nil
}

func (s *Service) DisableAccess(ctx context.Context, actor domain.RequestActor, params AccessDisableParams) (*domain.AppMembership, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.userRepo == nil || s.tx == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench user role repository is not configured.", nil)
	}
	if !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can disable workbench access.", nil)
	}
	if params.UserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "A disable reason is required.", nil)
	}
	currentRoles, err := s.userRepo.ListRoles(ctx, params.UserID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load user roles.", err.Error())
	}
	nextRoles := removeAssetWorkbenchRoles(currentRoles)
	var membership *domain.AppMembership
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		membership, err = s.repo.DisableMembership(ctx, tx, domain.AssetWorkbenchAppCode, params.UserID, actor.ID, reason, assetRolesFromRoles(currentRoles))
		if err != nil {
			return err
		}
		if err := s.userRepo.ReplaceRoles(ctx, tx, params.UserID, nextRoles); err != nil {
			return err
		}
		return s.appendIdentityEvent(ctx, tx, actor.ID, params.UserID, domain.AppIdentityActionAccessDisabled, map[string]interface{}{
			"roles": currentRoles,
		}, map[string]interface{}{
			"membership": membership,
			"roles":      nextRoles,
		}, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to disable asset workbench access.", err.Error())
	}
	return membership, nil
}

func (s *Service) UpdateMemberRoles(ctx context.Context, actor domain.RequestActor, userID int64, params UpdateMemberRolesParams) (*domain.AssetWorkbenchMember, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.userRepo == nil || s.tx == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench user role repository is not configured.", nil)
	}
	if !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can change workbench roles.", nil)
	}
	if userID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "user_id is required.", nil)
	}
	membership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, userID)
	if err != nil {
		return nil, mapRepoReadError(err, "Membership not found.", "Failed to load membership.")
	}
	if membership.Status != domain.AppMembershipStatusActive {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Only active workbench members can change roles.", map[string]string{"status": membership.Status})
	}
	currentRoles, err := s.userRepo.ListRoles(ctx, userID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load user roles.", err.Error())
	}
	nextRoles := mergeAssetWorkbenchRoles(removeAssetWorkbenchRoles(currentRoles), normalizeAssetWorkbenchRoleSet(params.Roles, true))
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.userRepo.ReplaceRoles(ctx, tx, userID, nextRoles); err != nil {
			return err
		}
		return s.appendIdentityEvent(ctx, tx, actor.ID, userID, domain.AppIdentityActionRolesUpdated, map[string]interface{}{
			"roles": currentRoles,
		}, map[string]interface{}{
			"roles": nextRoles,
		}, strings.TrimSpace(params.Reason))
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update asset workbench member roles.", err.Error())
	}
	member, appErr := s.loadMemberByID(ctx, actor, userID)
	if appErr != nil {
		return nil, appErr
	}
	return member, nil
}

func (s *Service) PreviewAccountMerge(ctx context.Context, actor domain.RequestActor, params AccountMergePreviewParams) (*AccountMergePreview, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can preview account merge.", nil)
	}
	if appErr := s.validateMergeUsers(ctx, params.SourceUserID, params.CanonicalUserID); appErr != nil {
		return nil, appErr
	}
	conflicts, appErr := s.profileMergeConflicts(ctx, params.SourceUserID, params.CanonicalUserID)
	if appErr != nil {
		return nil, appErr
	}
	counts, err := s.repo.CountAccountMergeImpact(ctx, params.SourceUserID, params.CanonicalUserID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to count account merge impact.", err.Error())
	}
	return &AccountMergePreview{
		SourceUserID:    params.SourceUserID,
		CanonicalUserID: params.CanonicalUserID,
		Conflicts:       conflicts,
		Counts:          mergeRewriteCountsMap(counts, 0),
		SettlementNote:  "已确认结算的 payee 可迁到主账号用于工作台归属；paid_to_user_id 与 payout_snapshot_json 不变，财务导出仍按真实发放对象读取。",
	}, nil
}

func (s *Service) MergeAccounts(ctx context.Context, actor domain.RequestActor, params AccountMergeParams) (*AccountMergePreview, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if s.userRepo == nil || s.tx == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench merge dependencies are not configured.", nil)
	}
	if !actorHasAny(actor, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can merge workbench accounts.", nil)
	}
	if appErr := s.validateMergeUsers(ctx, params.SourceUserID, params.CanonicalUserID); appErr != nil {
		return nil, appErr
	}
	conflicts, appErr := s.profileMergeConflicts(ctx, params.SourceUserID, params.CanonicalUserID)
	if appErr != nil {
		return nil, appErr
	}
	for field := range conflicts {
		choice := strings.TrimSpace(params.ProfileChoices[field])
		if choice != "source" && choice != "canonical" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Profile conflict choice is required.", map[string]string{"field": field})
		}
	}
	sourceUser, err := s.userRepo.GetByID(ctx, params.SourceUserID)
	if err != nil || sourceUser == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "Source user not found.", nil)
	}
	canonicalUser, err := s.userRepo.GetByID(ctx, params.CanonicalUserID)
	if err != nil || canonicalUser == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "Canonical user not found.", nil)
	}
	sourceRoles, err := s.userRepo.ListRoles(ctx, params.SourceUserID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load source roles.", err.Error())
	}
	canonicalRoles, err := s.userRepo.ListRoles(ctx, params.CanonicalUserID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load canonical roles.", err.Error())
	}
	sourceMembership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, params.SourceUserID)
	if err != nil {
		return nil, mapRepoReadError(err, "Source membership not found.", "Failed to load source membership.")
	}
	sourceAssetRoles := []domain.Role{}
	if sourceMembership.Status != domain.AppMembershipStatusDisabled {
		sourceAssetRoles = assetRolesFromRoles(sourceRoles)
	}
	counts := repo.AssetWorkbenchMergeRewriteCounts{}
	var revokedSessions int64
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.repo.LockMembership(ctx, tx, domain.AssetWorkbenchAppCode, params.SourceUserID); err != nil {
			return err
		}
		canonicalMembership, err := s.repo.LockMembership(ctx, tx, domain.AssetWorkbenchAppCode, params.CanonicalUserID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if canonicalMembership != nil && canonicalMembership.Status == domain.AppMembershipStatusMerged {
			return domain.NewAppError(domain.ErrCodeConflict, "Canonical user is already merged.", nil)
		}
		if err := s.repo.MarkMembershipMerged(ctx, tx, domain.AssetWorkbenchAppCode, params.SourceUserID); err != nil {
			return err
		}
		sourceUser.Status = domain.UserStatusDisabled
		sourceUser.UpdatedAt = s.nowFn().UTC()
		if err := s.userRepo.Update(ctx, tx, sourceUser); err != nil {
			return err
		}
		if s.sessionRevoker != nil {
			var err error
			revokedSessions, err = s.sessionRevoker.RevokeActiveByUserID(ctx, tx, params.SourceUserID, s.nowFn().UTC())
			if err != nil {
				return err
			}
		}
		openedBy := actor.ID
		if _, err := s.repo.UpsertMembership(ctx, tx, &domain.AppMembership{
			AppCode:      domain.AssetWorkbenchAppCode,
			UserID:       params.CanonicalUserID,
			Status:       domain.AppMembershipStatusActive,
			IdentityType: domain.AppMembershipIdentityStaff,
			Source:       domain.AppMembershipSourceMerged,
			OpenedBy:     &openedBy,
		}); err != nil {
			return err
		}
		nextCanonicalRoles := mergeAssetWorkbenchRoles(canonicalRoles, sourceAssetRoles)
		if err := s.userRepo.ReplaceRoles(ctx, tx, params.CanonicalUserID, nextCanonicalRoles); err != nil {
			return err
		}
		if err := s.userRepo.ReplaceRoles(ctx, tx, params.SourceUserID, removeAssetWorkbenchRoles(sourceRoles)); err != nil {
			return err
		}
		if err := s.repo.MergeProfiles(ctx, tx, params.SourceUserID, params.CanonicalUserID, params.ProfileChoices, actor.ID); err != nil {
			return err
		}
		var rewriteErr error
		counts, rewriteErr = s.repo.RewriteAccountOwnership(ctx, tx, params.SourceUserID, params.CanonicalUserID)
		if rewriteErr != nil {
			return rewriteErr
		}
		if _, err := s.repo.CreateAccountLink(ctx, tx, &domain.AssetWorkbenchAccountLink{
			SourceUserID:    params.SourceUserID,
			CanonicalUserID: params.CanonicalUserID,
			Status:          "merged",
			CreatedBy:       actor.ID,
		}); err != nil {
			return err
		}
		if err := s.appendIdentityEvent(ctx, tx, actor.ID, params.CanonicalUserID, domain.AppIdentityActionAccountMerged, map[string]interface{}{
			"source_user_id":    params.SourceUserID,
			"canonical_user_id": params.CanonicalUserID,
			"source_roles":      sourceRoles,
			"canonical_roles":   canonicalRoles,
		}, map[string]interface{}{
			"source_user_id":          params.SourceUserID,
			"canonical_user_id":       params.CanonicalUserID,
			"canonical_roles":         nextCanonicalRoles,
			"rewrite_counts":          counts,
			"revoked_source_sessions": revokedSessions,
			"paid_to_preserved":       true,
		}, strings.TrimSpace(params.Reason)); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventAccountMerged, domain.AssetWorkbenchEntityMember, &params.CanonicalUserID, nil, map[string]interface{}{
			"source_user_id":    params.SourceUserID,
			"canonical_user_id": params.CanonicalUserID,
			"rewrite_counts":    counts,
			"paid_to_preserved": true,
		}, strings.TrimSpace(params.Reason))
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to merge asset workbench accounts.", err.Error())
	}
	return &AccountMergePreview{
		SourceUserID:    params.SourceUserID,
		CanonicalUserID: params.CanonicalUserID,
		Conflicts:       conflicts,
		Counts:          mergeRewriteCountsMap(counts, revokedSessions),
		SettlementNote:  "工作台归属已迁移到主账号；历史真实发放对象仍由 paid_to_user_id 与 payout_snapshot_json 固定。",
	}, nil
}

func mergeRewriteCountsMap(counts repo.AssetWorkbenchMergeRewriteCounts, revokedSessions int64) map[string]int64 {
	return map[string]int64{
		"submissions":              counts.Submissions,
		"submission_items":         counts.SubmissionItems,
		"upload_sessions":          counts.UploadSessions,
		"submission_files":         counts.SubmissionFiles,
		"error_records":            counts.ErrorRecords,
		"settlement_supplements":   counts.SettlementSupplements,
		"settlement_items":         counts.SettlementItems,
		"settlement_items_deduped": counts.SettlementItemsDeduped,
		"group_members":            counts.GroupMembers,
		"saved_views":              counts.SavedViews,
		"grade_periods":            counts.GradePeriods,
		"supplement_permissions":   counts.SupplementPermissions,
		"revoked_source_sessions":  revokedSessions,
	}
}

func (s *Service) validateMergeUsers(ctx context.Context, sourceUserID, canonicalUserID int64) *domain.AppError {
	if sourceUserID <= 0 || canonicalUserID <= 0 || sourceUserID == canonicalUserID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "source_user_id and canonical_user_id must be different positive values.", nil)
	}
	sourceMembership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, sourceUserID)
	if err != nil {
		return mapRepoReadError(err, "Source membership not found.", "Failed to load source membership.")
	}
	if sourceMembership.Status == domain.AppMembershipStatusMerged {
		return domain.NewAppError(domain.ErrCodeConflict, "Source user is already merged.", nil)
	}
	if link, err := s.repo.GetAccountLinkBySource(ctx, sourceUserID); err == nil && link != nil {
		return domain.NewAppError(domain.ErrCodeConflict, "Source user already has an account link.", nil)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check source account link.", err.Error())
	}
	canonicalMembership, err := s.repo.GetMembership(ctx, domain.AssetWorkbenchAppCode, canonicalUserID)
	if err == nil && canonicalMembership != nil && canonicalMembership.Status == domain.AppMembershipStatusMerged {
		return domain.NewAppError(domain.ErrCodeConflict, "Canonical user is already merged.", nil)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to load canonical membership.", err.Error())
	}
	if link, err := s.repo.GetAccountLinkBySource(ctx, canonicalUserID); err == nil && link != nil {
		return domain.NewAppError(domain.ErrCodeConflict, "Canonical user cannot be a merged source account.", nil)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check canonical account link.", err.Error())
	}
	return nil
}

func (s *Service) profileMergeConflicts(ctx context.Context, sourceUserID, canonicalUserID int64) (map[string]MergeConflict, *domain.AppError) {
	source, err := s.repo.GetProfileByUserID(ctx, sourceUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]MergeConflict{}, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load source profile.", err.Error())
	}
	canonical, err := s.repo.GetProfileByUserID(ctx, canonicalUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]MergeConflict{}, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load canonical profile.", err.Error())
	}
	conflicts := map[string]MergeConflict{}
	add := func(field, sourceValue, canonicalValue string) {
		sourceValue = strings.TrimSpace(sourceValue)
		canonicalValue = strings.TrimSpace(canonicalValue)
		if sourceValue != "" && canonicalValue != "" && sourceValue != canonicalValue {
			conflicts[field] = MergeConflict{Field: field, SourceValue: sourceValue, CanonicalValue: canonicalValue}
		}
	}
	ptrValue := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	add("real_name", source.RealName, canonical.RealName)
	add("phone", ptrValue(source.Phone), ptrValue(canonical.Phone))
	add("id_card", ptrValue(source.IDCard), ptrValue(canonical.IDCard))
	add("alipay_account", source.AlipayAccount, canonical.AlipayAccount)
	return conflicts, nil
}

func maskProfileListPII(items []*domain.AssetWorkbenchProfile) []*domain.AssetWorkbenchProfile {
	if len(items) == 0 {
		return []*domain.AssetWorkbenchProfile{}
	}
	out := make([]*domain.AssetWorkbenchProfile, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		refreshProfileCompletion(&copyItem)
		if copyItem.Phone != nil {
			masked := maskSensitiveValue(*copyItem.Phone, 3, 4)
			copyItem.Phone = &masked
		}
		if copyItem.IDCard != nil {
			masked := maskSensitiveValue(*copyItem.IDCard, 0, 4)
			copyItem.IDCard = &masked
		}
		copyItem.AlipayAccount = maskSensitiveValue(copyItem.AlipayAccount, 2, 4)
		out = append(out, &copyItem)
	}
	return out
}

func maskSensitiveValue(value string, prefix, suffix int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= prefix+suffix {
		if len(runes) <= suffix {
			return strings.Repeat("*", len(runes))
		}
		return string(runes[:prefix]) + strings.Repeat("*", len(runes)-prefix)
	}
	return string(runes[:prefix]) + strings.Repeat("*", len(runes)-prefix-suffix) + string(runes[len(runes)-suffix:])
}

func (s *Service) ListDifficultyClasses(ctx context.Context, actor domain.RequestActor, admin bool) ([]*domain.AssetWorkbenchDifficultyClass, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if admin {
		if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench admins can list difficulty classes for administration.", nil)
		}
		items, err := s.repo.ListDifficultyClasses(ctx, repo.AssetWorkbenchDifficultyClassFilter{})
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench difficulty classes.", err.Error())
		}
		return items, nil
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleHRAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can list difficulty classes.", nil)
	}
	enabled := true
	items, err := s.repo.ListDifficultyClasses(ctx, repo.AssetWorkbenchDifficultyClassFilter{Enabled: &enabled})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench difficulty classes.", err.Error())
	}
	return items, nil
}

func (s *Service) CreateDifficultyClass(ctx context.Context, actor domain.RequestActor, params CreateDifficultyClassParams) (*domain.AssetWorkbenchDifficultyClass, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only pricing admins can create difficulty classes.", nil)
	}
	code, appErr := normalizeWorkbenchDifficultyCode(params.Code, false)
	if appErr != nil {
		return nil, appErr
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = code
	}
	item := &domain.AssetWorkbenchDifficultyClass{
		Code:        code,
		Name:        name,
		Description: strings.TrimSpace(params.Description),
		Enabled:     boolValueDefault(params.Enabled, true),
		SortOrder:   params.SortOrder,
		CreatedBy:   &actor.ID,
		UpdatedBy:   &actor.ID,
	}
	var created *domain.AssetWorkbenchDifficultyClass
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateDifficultyClass(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventDifficultyUpserted, domain.AssetWorkbenchEntityDifficultyClass, &created.ID, nil, created, "create difficulty class")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench difficulty class.", err.Error())
	}
	return created, nil
}

func (s *Service) UpdateDifficultyClass(ctx context.Context, actor domain.RequestActor, code string, params UpdateDifficultyClassParams) (*domain.AssetWorkbenchDifficultyClass, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only pricing admins can update difficulty classes.", nil)
	}
	normalizedCode, appErr := normalizeWorkbenchDifficultyCode(code, false)
	if appErr != nil {
		return nil, appErr
	}
	existing, err := s.repo.GetDifficultyClass(ctx, normalizedCode)
	if err != nil {
		return nil, mapRepoReadError(err, "Difficulty class not found.", "Failed to load difficulty class.")
	}
	item := *existing
	if params.Name != nil {
		item.Name = strings.TrimSpace(*params.Name)
	}
	if item.Name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required.", nil)
	}
	if params.Description != nil {
		item.Description = strings.TrimSpace(*params.Description)
	}
	if params.Enabled != nil {
		item.Enabled = *params.Enabled
	}
	if params.SortOrder != nil {
		item.SortOrder = *params.SortOrder
	}
	item.UpdatedBy = &actor.ID
	var updated *domain.AssetWorkbenchDifficultyClass
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateDifficultyClass(ctx, tx, &item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventDifficultyUpserted, domain.AssetWorkbenchEntityDifficultyClass, &updated.ID, existing, updated, "update difficulty class")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, mapRepoReadError(err, "Difficulty class not found.", "Failed to update difficulty class.")
	}
	return updated, nil
}

func (s *Service) ListPriceMatrix(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchPriceMatrixFilter) ([]*domain.AssetWorkbenchPriceMatrix, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template or settlement roles can read price matrix.", nil)
	}
	items, total, err := s.repo.ListPriceMatrix(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench price matrix.", err.Error())
	}
	return items, total, nil
}

func (s *Service) CreatePriceMatrix(ctx context.Context, actor domain.RequestActor, params CreatePriceMatrixParams) (*domain.AssetWorkbenchPriceMatrix, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can create price matrix rules.", nil)
	}
	item, appErr := normalizePriceMatrix(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, item.DifficultyClass, false); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchPriceMatrix
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		existing, err := s.repo.LockPriceMatrixDimension(ctx, tx, item.WorkerType, item.JobGrade, item.DifficultyClass)
		if err != nil {
			return err
		}
		if overlapsPricePeriod(existing, item.EffectiveFrom, item.EffectiveTo) {
			return domain.NewAppError(domain.ErrCodeConflict, "Price effective range overlaps an existing rule.", nil)
		}
		created, err = s.repo.CreatePriceMatrix(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPriceCreated, domain.AssetWorkbenchEntityPriceMatrix, &created.ID, nil, created, params.Remark)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench price matrix rule.", err.Error())
	}
	return created, nil
}

func (s *Service) SetPriceMatrixEnabled(ctx context.Context, actor domain.RequestActor, ruleID int64, params SetCostRuleEnabledParams) (*domain.AssetWorkbenchPriceMatrix, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update price matrix rules.", nil)
	}
	var updated *domain.AssetWorkbenchPriceMatrix
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetPriceMatrixForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		if params.Enabled {
			existing, err := s.repo.LockPriceMatrixDimension(ctx, tx, before.WorkerType, before.JobGrade, before.DifficultyClass)
			if err != nil {
				return err
			}
			for _, item := range existing {
				if item.ID == before.ID {
					item.Enabled = false
				}
			}
			if overlapsPricePeriod(existing, before.EffectiveFrom, before.EffectiveTo) {
				return domain.NewAppError(domain.ErrCodeConflict, "Price effective range overlaps an existing rule.", nil)
			}
		}
		updated, err = s.repo.SetPriceMatrixEnabled(ctx, tx, ruleID, params.Enabled)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPriceUpdated, domain.AssetWorkbenchEntityPriceMatrix, &updated.ID, before, updated, params.Reason)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Price matrix rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update asset workbench price matrix rule.", err.Error())
	}
	return updated, nil
}

func (s *Service) SupersedePriceMatrix(ctx context.Context, actor domain.RequestActor, ruleID int64, params CreatePriceMatrixParams) (*domain.AssetWorkbenchPriceMatrix, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update price matrix rules.", nil)
	}
	next, appErr := normalizePriceMatrix(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, next.DifficultyClass, false); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchPriceMatrix
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetPriceMatrixForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		if !before.Enabled {
			return domain.NewAppError(domain.ErrCodeConflict, "Only enabled price rules can publish a new version.", nil)
		}
		if before.WorkerType != next.WorkerType || before.JobGrade != next.JobGrade || before.DifficultyClass != next.DifficultyClass {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "A new price version must keep the same worker_type, job_grade and difficulty_class.", map[string]string{
				"worker_type":      next.WorkerType,
				"job_grade":        next.JobGrade,
				"difficulty_class": next.DifficultyClass,
			})
		}
		if !next.EffectiveFrom.After(before.EffectiveFrom) {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "New price version effective_from must be after the current rule effective_from.", nil)
		}
		closedEffectiveTo := next.EffectiveFrom.AddDate(0, 0, -1)
		if before.EffectiveTo != nil && before.EffectiveTo.Before(closedEffectiveTo) {
			return domain.NewAppError(domain.ErrCodeConflict, "Selected price rule already ends before the new effective_from. Create a new price rule instead.", nil)
		}
		existing, err := s.repo.LockPriceMatrixDimension(ctx, tx, next.WorkerType, next.JobGrade, next.DifficultyClass)
		if err != nil {
			return err
		}
		for _, item := range existing {
			if item.ID == before.ID {
				item.EffectiveTo = &closedEffectiveTo
			}
		}
		if overlapsPricePeriod(existing, next.EffectiveFrom, next.EffectiveTo) {
			return domain.NewAppError(domain.ErrCodeConflict, "Price effective range overlaps an existing rule.", nil)
		}
		next.RevisionNo = nextPriceRevision(existing)
		closed, err := s.repo.SetPriceMatrixEffectiveTo(ctx, tx, before.ID, &closedEffectiveTo)
		if err != nil {
			return err
		}
		created, err = s.repo.CreatePriceMatrix(ctx, tx, next)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPriceSuperseded, domain.AssetWorkbenchEntityPriceMatrix, &created.ID, before, map[string]interface{}{
			"closed_rule": closed,
			"new_rule":    created,
		}, params.Remark)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Price matrix rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to supersede asset workbench price matrix rule.", err.Error())
	}
	return created, nil
}

func (s *Service) ListDeductionRules(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchDeductionRuleFilter) ([]*domain.AssetWorkbenchDeductionRule, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template or settlement roles can read deduction rules.", nil)
	}
	items, total, err := s.repo.ListDeductionRules(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench deduction rules.", err.Error())
	}
	return items, total, nil
}

func (s *Service) CreateDeductionRule(ctx context.Context, actor domain.RequestActor, params CreateDeductionRuleParams) (*domain.AssetWorkbenchDeductionRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can create deduction rules.", nil)
	}
	item, appErr := normalizeDeductionRule(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, item.DifficultyClass, true); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchDeductionRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		existing, err := s.repo.LockDeductionRuleDimension(ctx, tx, item.WorkerType, item.JobGrade, item.DifficultyClass)
		if err != nil {
			return err
		}
		if overlapsDeductionPeriod(existing, item.EffectiveFrom, item.EffectiveTo) {
			return domain.NewAppError(domain.ErrCodeConflict, "Deduction effective range overlaps an existing rule.", nil)
		}
		created, err = s.repo.CreateDeductionRule(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventDeductionCreated, domain.AssetWorkbenchEntityDeductionRule, &created.ID, nil, created, params.Remark)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench deduction rule.", err.Error())
	}
	return created, nil
}

func (s *Service) SetDeductionRuleEnabled(ctx context.Context, actor domain.RequestActor, ruleID int64, params SetCostRuleEnabledParams) (*domain.AssetWorkbenchDeductionRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update deduction rules.", nil)
	}
	var updated *domain.AssetWorkbenchDeductionRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetDeductionRuleForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		if params.Enabled {
			existing, err := s.repo.LockDeductionRuleDimension(ctx, tx, before.WorkerType, before.JobGrade, before.DifficultyClass)
			if err != nil {
				return err
			}
			for _, item := range existing {
				if item.ID == before.ID {
					item.Enabled = false
				}
			}
			if overlapsDeductionPeriod(existing, before.EffectiveFrom, before.EffectiveTo) {
				return domain.NewAppError(domain.ErrCodeConflict, "Deduction effective range overlaps an existing rule.", nil)
			}
		}
		updated, err = s.repo.SetDeductionRuleEnabled(ctx, tx, ruleID, params.Enabled)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventDeductionUpdated, domain.AssetWorkbenchEntityDeductionRule, &updated.ID, before, updated, params.Reason)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Deduction rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update asset workbench deduction rule.", err.Error())
	}
	return updated, nil
}

func (s *Service) SupersedeDeductionRule(ctx context.Context, actor domain.RequestActor, ruleID int64, params CreateDeductionRuleParams) (*domain.AssetWorkbenchDeductionRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update deduction rules.", nil)
	}
	next, appErr := normalizeDeductionRule(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, next.DifficultyClass, true); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchDeductionRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetDeductionRuleForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		existing, err := s.repo.LockDeductionRuleDimension(ctx, tx, next.WorkerType, next.JobGrade, next.DifficultyClass)
		if err != nil {
			return err
		}
		for _, item := range existing {
			if item.ID == before.ID {
				item.Enabled = false
			}
		}
		if overlapsDeductionPeriod(existing, next.EffectiveFrom, next.EffectiveTo) {
			return domain.NewAppError(domain.ErrCodeConflict, "Deduction effective range overlaps an existing rule.", nil)
		}
		next.RevisionNo = nextDeductionRevision(existing)
		disabled, err := s.repo.SetDeductionRuleEnabled(ctx, tx, before.ID, false)
		if err != nil {
			return err
		}
		created, err = s.repo.CreateDeductionRule(ctx, tx, next)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventDeductionSuperseded, domain.AssetWorkbenchEntityDeductionRule, &created.ID, disabled, created, params.Remark)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Deduction rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to supersede asset workbench deduction rule.", err.Error())
	}
	return created, nil
}

func (s *Service) ListWelfareRules(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchWelfareRuleFilter) ([]*domain.AssetWorkbenchWelfareRule, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template or settlement roles can read welfare rules.", nil)
	}
	items, total, err := s.repo.ListWelfareRules(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench welfare rules.", err.Error())
	}
	return items, total, nil
}

func (s *Service) CreateWelfareRule(ctx context.Context, actor domain.RequestActor, params CreateWelfareRuleParams) (*domain.AssetWorkbenchWelfareRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can create welfare rules.", nil)
	}
	item, appErr := normalizeWelfareRule(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchWelfareRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateWelfareRule(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventWelfareCreated, domain.AssetWorkbenchEntityWelfareRule, &created.ID, nil, created, params.Remark)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench welfare rule.", err.Error())
	}
	return created, nil
}

func (s *Service) SetWelfareRuleEnabled(ctx context.Context, actor domain.RequestActor, ruleID int64, params SetCostRuleEnabledParams) (*domain.AssetWorkbenchWelfareRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update welfare rules.", nil)
	}
	var updated *domain.AssetWorkbenchWelfareRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetWelfareRuleForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		updated, err = s.repo.SetWelfareRuleEnabled(ctx, tx, ruleID, params.Enabled)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventWelfareUpdated, domain.AssetWorkbenchEntityWelfareRule, &updated.ID, before, updated, params.Reason)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Welfare rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update asset workbench welfare rule.", err.Error())
	}
	return updated, nil
}

func (s *Service) SupersedeWelfareRule(ctx context.Context, actor domain.RequestActor, ruleID int64, params CreateWelfareRuleParams) (*domain.AssetWorkbenchWelfareRule, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update welfare rules.", nil)
	}
	next, appErr := normalizeWelfareRule(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchWelfareRule
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetWelfareRuleForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		disabled, err := s.repo.SetWelfareRuleEnabled(ctx, tx, before.ID, false)
		if err != nil {
			return err
		}
		created, err = s.repo.CreateWelfareRule(ctx, tx, next)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventWelfareSuperseded, domain.AssetWorkbenchEntityWelfareRule, &created.ID, disabled, created, params.Remark)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Welfare rule not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to supersede asset workbench welfare rule.", err.Error())
	}
	return created, nil
}

func (s *Service) ListPromoCoupons(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchPromoCouponFilter) ([]*domain.AssetWorkbenchPromoCoupon, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template or settlement roles can read promo coupons.", nil)
	}
	items, total, err := s.repo.ListPromoCoupons(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench promo coupons.", err.Error())
	}
	return items, total, nil
}

func (s *Service) CreatePromoCoupon(ctx context.Context, actor domain.RequestActor, params CreatePromoCouponParams) (*domain.AssetWorkbenchPromoCoupon, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can create promo coupons.", nil)
	}
	item, appErr := normalizePromoCoupon(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, item.DifficultyClass, true); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchPromoCoupon
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreatePromoCoupon(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPromoCreated, domain.AssetWorkbenchEntityPromoCoupon, &created.ID, nil, created, params.Remark)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench promo coupon.", err.Error())
	}
	return created, nil
}

func (s *Service) SetPromoCouponEnabled(ctx context.Context, actor domain.RequestActor, ruleID int64, params SetCostRuleEnabledParams) (*domain.AssetWorkbenchPromoCoupon, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update promo coupons.", nil)
	}
	var updated *domain.AssetWorkbenchPromoCoupon
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetPromoCouponForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		updated, err = s.repo.SetPromoCouponEnabled(ctx, tx, ruleID, params.Enabled)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPromoUpdated, domain.AssetWorkbenchEntityPromoCoupon, &updated.ID, before, updated, params.Reason)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Promo coupon not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update asset workbench promo coupon.", err.Error())
	}
	return updated, nil
}

func (s *Service) SupersedePromoCoupon(ctx context.Context, actor domain.RequestActor, ruleID int64, params CreatePromoCouponParams) (*domain.AssetWorkbenchPromoCoupon, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only template admins can update promo coupons.", nil)
	}
	next, appErr := normalizePromoCoupon(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, next.DifficultyClass, true); appErr != nil {
		return nil, appErr
	}
	var created *domain.AssetWorkbenchPromoCoupon
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetPromoCouponForUpdate(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		disabled, err := s.repo.SetPromoCouponEnabled(ctx, tx, before.ID, false)
		if err != nil {
			return err
		}
		created, err = s.repo.CreatePromoCoupon(ctx, tx, next)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventPromoSuperseded, domain.AssetWorkbenchEntityPromoCoupon, &created.ID, disabled, created, params.Remark)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Promo coupon not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to supersede asset workbench promo coupon.", err.Error())
	}
	return created, nil
}

func (s *Service) ListGroups(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchGroupFilter) ([]*domain.AssetWorkbenchGroup, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can list groups.", nil)
	}
	items, total, err := s.repo.ListGroups(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench groups.", err.Error())
	}
	return items, total, nil
}

func (s *Service) CreateGroup(ctx context.Context, actor domain.RequestActor, params UpsertGroupParams) (*domain.AssetWorkbenchGroup, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can create groups.", nil)
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required.", nil)
	}
	item := &domain.AssetWorkbenchGroup{
		Name:        name,
		Description: strings.TrimSpace(params.Description),
		Enabled:     true,
		CreatedBy:   actor.ID,
	}
	var created *domain.AssetWorkbenchGroup
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateGroup(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventGroupUpserted, domain.AssetWorkbenchEntityGroup, &created.ID, nil, created, "create group")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench group.", err.Error())
	}
	return created, nil
}

func (s *Service) UpdateGroup(ctx context.Context, actor domain.RequestActor, groupID int64, params UpsertGroupParams) (*domain.AssetWorkbenchGroup, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can update groups.", nil)
	}
	name := strings.TrimSpace(params.Name)
	if groupID <= 0 || name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "group_id and name are required.", nil)
	}
	item := &domain.AssetWorkbenchGroup{
		ID:          groupID,
		Name:        name,
		Description: strings.TrimSpace(params.Description),
		Enabled:     params.Enabled,
	}
	var updated *domain.AssetWorkbenchGroup
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateGroup(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventGroupUpserted, domain.AssetWorkbenchEntityGroup, &updated.ID, nil, updated, "update group")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, mapRepoReadError(err, "Group not found.", "Failed to update asset workbench group.")
	}
	return updated, nil
}

func (s *Service) DeleteGroup(ctx context.Context, actor domain.RequestActor, groupID int64) (*domain.AssetWorkbenchGroup, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can disable groups.", nil)
	}
	var updated *domain.AssetWorkbenchGroup
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.SetGroupEnabled(ctx, tx, groupID, false)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventGroupUpserted, domain.AssetWorkbenchEntityGroup, &updated.ID, nil, updated, "disable group")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, mapRepoReadError(err, "Group not found.", "Failed to disable asset workbench group.")
	}
	return updated, nil
}

func (s *Service) AddGroupMembers(ctx context.Context, actor domain.RequestActor, groupID int64, params GroupMembersParams) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can update group members.", nil)
	}
	userIDs := positiveUniqueInt64s(params.UserIDs)
	if groupID <= 0 || len(userIDs) == 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "group_id and user_ids are required.", nil)
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.AddGroupMembers(ctx, tx, groupID, userIDs); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventGroupUpserted, domain.AssetWorkbenchEntityGroup, &groupID, nil, map[string]interface{}{
			"added_user_ids": userIDs,
		}, "add group members")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to add asset workbench group members.", err.Error())
	}
	return nil
}

func (s *Service) RemoveGroupMembers(ctx context.Context, actor domain.RequestActor, groupID int64, params GroupMembersParams) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can update group members.", nil)
	}
	userIDs := positiveUniqueInt64s(params.UserIDs)
	if groupID <= 0 || len(userIDs) == 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "group_id and user_ids are required.", nil)
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.RemoveGroupMembers(ctx, tx, groupID, userIDs); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventGroupUpserted, domain.AssetWorkbenchEntityGroup, &groupID, nil, map[string]interface{}{
			"removed_user_ids": userIDs,
		}, "remove group members")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to remove asset workbench group members.", err.Error())
	}
	return nil
}

func (s *Service) ListGroupMembers(ctx context.Context, actor domain.RequestActor, groupID int64) ([]*domain.AssetWorkbenchGroupMember, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can list group members.", nil)
	}
	if groupID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "group_id is required.", nil)
	}
	items, err := s.repo.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench group members.", err.Error())
	}
	return items, nil
}

func (s *Service) ListUploadDirectories(ctx context.Context, actor domain.RequestActor, admin bool) ([]*domain.AssetWorkbenchUploadDirectory, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if admin {
		if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can list upload directories for administration.", nil)
		}
		items, err := s.repo.ListUploadDirectories(ctx, repo.AssetWorkbenchUploadDirectoryFilter{})
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench upload directories.", err.Error())
		}
		return items, nil
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can list upload directories.", nil)
	}
	enabled := true
	items, err := s.repo.ListUploadDirectories(ctx, repo.AssetWorkbenchUploadDirectoryFilter{Enabled: &enabled})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench upload directories.", err.Error())
	}
	return items, nil
}

func (s *Service) CreateUploadDirectory(ctx context.Context, actor domain.RequestActor, params CreateUploadDirectoryParams) (*domain.AssetWorkbenchUploadDirectory, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can create upload directories.", nil)
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required.", nil)
	}
	prefix, appErr := normalizeUploadDirectoryPrefix(params.OSSPrefix)
	if appErr != nil {
		return nil, appErr
	}
	difficulty, appErr := normalizeUploadDirectoryDifficulty(params.DifficultyClass)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureDifficultyClass(ctx, difficulty, false); appErr != nil {
		return nil, appErr
	}
	allowedFileTypes, appErr := normalizeUploadDirectoryFileTypes(params.AllowedFileTypes)
	if appErr != nil {
		return nil, appErr
	}
	item := &domain.AssetWorkbenchUploadDirectory{
		Name:             name,
		OSSPrefix:        prefix,
		Description:      strings.TrimSpace(params.Description),
		DifficultyClass:  difficulty,
		AllowedFileTypes: allowedFileTypes,
		Enabled:          boolValueDefault(params.Enabled, true),
		SortOrder:        params.SortOrder,
		CreatedBy:        actor.ID,
	}
	var created *domain.AssetWorkbenchUploadDirectory
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateUploadDirectory(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventUploadDirectoryUpserted, domain.AssetWorkbenchEntityUploadDirectory, &created.ID, nil, created, "create upload directory")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench upload directory.", err.Error())
	}
	return created, nil
}

func (s *Service) UpdateUploadDirectory(ctx context.Context, actor domain.RequestActor, directoryID int64, params UpdateUploadDirectoryParams) (*domain.AssetWorkbenchUploadDirectory, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can update upload directories.", nil)
	}
	if directoryID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "directory_id is required.", nil)
	}
	existing, err := s.repo.GetUploadDirectory(ctx, directoryID)
	if err != nil {
		return nil, mapRepoReadError(err, "Upload directory not found.", "Failed to load upload directory.")
	}
	item := *existing
	if params.Name != nil {
		item.Name = strings.TrimSpace(*params.Name)
	}
	if item.Name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required.", nil)
	}
	if params.OSSPrefix != nil {
		prefix, appErr := normalizeUploadDirectoryPrefix(*params.OSSPrefix)
		if appErr != nil {
			return nil, appErr
		}
		item.OSSPrefix = prefix
	}
	if params.Description != nil {
		item.Description = strings.TrimSpace(*params.Description)
	}
	if params.DifficultyClass != nil {
		difficulty, appErr := normalizeUploadDirectoryDifficulty(*params.DifficultyClass)
		if appErr != nil {
			return nil, appErr
		}
		if appErr := s.ensureDifficultyClass(ctx, difficulty, false); appErr != nil {
			return nil, appErr
		}
		item.DifficultyClass = difficulty
	}
	if params.AllowedFileTypes != nil {
		allowedFileTypes, appErr := normalizeUploadDirectoryFileTypes(params.AllowedFileTypes)
		if appErr != nil {
			return nil, appErr
		}
		item.AllowedFileTypes = allowedFileTypes
	}
	if params.Enabled != nil {
		item.Enabled = *params.Enabled
	}
	if params.SortOrder != nil {
		item.SortOrder = *params.SortOrder
	}
	item.UpdatedBy = &actor.ID
	var updated *domain.AssetWorkbenchUploadDirectory
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateUploadDirectory(ctx, tx, &item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventUploadDirectoryUpserted, domain.AssetWorkbenchEntityUploadDirectory, &updated.ID, existing, updated, "update upload directory")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, mapRepoReadError(err, "Upload directory not found.", "Failed to update asset workbench upload directory.")
	}
	return updated, nil
}

func (s *Service) CreateUploadSession(ctx context.Context, actor domain.RequestActor, params CreateUploadSessionParams) (*UploadSessionResponse, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset submitters can create upload sessions.", nil)
	}
	if s.oss == nil || !s.oss.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "OSS direct upload is not enabled.", nil)
	}
	filename := strings.TrimSpace(params.OriginalFilename)
	if filename == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "original_filename is required.", nil)
	}
	if params.FileSize <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_size must be positive.", nil)
	}
	relativePath, appErr := normalizeUploadRelativePath(params.RelativePath, filename)
	if appErr != nil {
		return nil, appErr
	}
	uploadBatchID := normalizeUploadBatchID(params.UploadBatchID)
	expectedBusinessMonth := strings.TrimSpace(params.ExpectedBusinessMonth)
	if expectedBusinessMonth != "" {
		if _, err := time.Parse("2006-01", expectedBusinessMonth); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_business_month must use YYYY-MM format.", map[string]string{
				"expected_business_month": expectedBusinessMonth,
			})
		}
	}
	directory, appErr := s.resolveUploadDirectoryForSession(ctx, params.UploadDirectoryID)
	if appErr != nil {
		return nil, appErr
	}
	if !uploadDirectoryAllowsFile(directory, path.Base(relativePath), params.MimeType) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file type is not allowed for this upload directory.", map[string]interface{}{
			"allowed_file_types": directory.AllowedFileTypes,
		})
	}
	now := s.nowFn().UTC()
	sessionID := uuid.NewString()
	objectKey := s.buildObjectKey(now, sessionID, filename, directory, relativePath)
	plan, err := s.oss.CreateUploadPlan(ctx, objectKey, params.FileSize, params.MimeType)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to create OSS upload plan.", err.Error())
	}
	planJSON, _ := json.Marshal(plan)
	session := &domain.AssetWorkbenchUploadSession{
		SessionID:        sessionID,
		OwnerUserID:      actor.ID,
		Status:           domain.AssetWorkbenchUploadStatusCreated,
		ObjectKey:        objectKey,
		OriginalFilename: filename,
		FileSize:         params.FileSize,
		MimeType:         strings.TrimSpace(params.MimeType),
		FileHash:         strings.TrimSpace(params.FileHash),
		UploadID:         plan.UploadID,
		MultipartPlan:    planJSON,
		ExpiresAt:        now.Add(s.cfg.UploadSessionTTL),
	}
	session.UploadBatchID = uploadBatchID
	session.RelativePath = relativePath
	session.IsFolderUpload = params.IsFolderUpload || strings.Contains(relativePath, "/")
	session.ExpectedBusinessMonth = expectedBusinessMonth
	if directory != nil {
		session.UploadDirectoryID = &directory.ID
		session.UploadDirectoryName = directory.Name
		session.UploadDirectoryPrefix = directory.OSSPrefix
		session.UploadDirectoryDifficultyClass = directory.DifficultyClass
	}
	var created *domain.AssetWorkbenchUploadSession
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateUploadSession(ctx, tx, session)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventUploadSessionCreated, domain.AssetWorkbenchEntityUploadSession, &created.ID, nil, created, "")
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset upload session.", err.Error())
	}
	return &UploadSessionResponse{Session: created, Plan: plan}, nil
}

func (s *Service) MarkUploadSessionUploaded(ctx context.Context, actor domain.RequestActor, sessionID string) (*domain.AssetWorkbenchUploadSession, *domain.AppError) {
	return s.CompleteUploadSession(ctx, actor, sessionID, CompleteUploadSessionParams{})
}

func (s *Service) CompleteUploadSession(ctx context.Context, actor domain.RequestActor, sessionID string, params CompleteUploadSessionParams) (*domain.AssetWorkbenchUploadSession, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	lockedSessionID := strings.TrimSpace(sessionID)
	var session *domain.AssetWorkbenchUploadSession
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		locked, err := s.repo.GetUploadSessionForUpdate(ctx, tx, lockedSessionID)
		if err != nil {
			return err
		}
		if locked == nil {
			return sql.ErrNoRows
		}
		session = locked
		if session.OwnerUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "Upload session is not owned by current user.", nil)
		}
		switch session.Status {
		case domain.AssetWorkbenchUploadStatusUploaded:
			return nil
		case domain.AssetWorkbenchUploadStatusSubmitted:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session is already submitted.", nil)
		case domain.AssetWorkbenchUploadStatusCancelled, domain.AssetWorkbenchUploadStatusExpired:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session is already terminal.", nil)
		}
		if appErr := s.completeAndVerifyUploadObject(ctx, session, params); appErr != nil {
			return appErr
		}
		now := s.nowFn().UTC()
		if err := s.repo.UpdateUploadSessionStatus(ctx, tx, session.SessionID, domain.AssetWorkbenchUploadStatusUploaded, &now, nil, nil); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventUploadSessionUpdated, domain.AssetWorkbenchEntityUploadSession, &session.ID, session, map[string]interface{}{
			"session_id": session.SessionID,
			"status":     domain.AssetWorkbenchUploadStatusUploaded,
		}, "")
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, mapRepoReadError(err, "Upload session not found.", "Failed to load upload session.")
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to mark upload session uploaded.", err.Error())
	}
	updated, err := s.repo.GetUploadSession(ctx, lockedSessionID)
	if err != nil {
		return nil, mapRepoReadError(err, "Upload session not found.", "Failed to reload upload session.")
	}
	return updated, nil
}

func (s *Service) completeAndVerifyUploadObject(ctx context.Context, session *domain.AssetWorkbenchUploadSession, params CompleteUploadSessionParams) *domain.AppError {
	if s.oss == nil || !s.oss.Enabled() || session == nil {
		return nil
	}
	var completeErr error
	if strings.TrimSpace(session.UploadID) != "" {
		if len(params.Parts) == 0 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "multipart upload completion requires parts.", nil)
		}
		completeErr = s.oss.CompleteMultipartUpload(ctx, session.ObjectKey, session.UploadID, params.Parts)
	}
	info, exists, statErr := s.oss.StatObject(ctx, session.ObjectKey)
	if statErr != nil {
		details := statErr.Error()
		if completeErr != nil {
			details = completeErr.Error() + "; verify: " + details
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to verify completed OSS upload.", details)
	}
	if !exists || info == nil {
		if completeErr != nil {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to complete OSS multipart upload.", completeErr.Error())
		}
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "Completed OSS upload is missing.", nil)
	}
	if info.ContentLength != session.FileSize {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "Completed OSS upload size does not match upload session.", map[string]interface{}{
			"expected_size": session.FileSize,
			"actual_size":   info.ContentLength,
		})
	}
	return nil
}

func (s *Service) CancelUploadSession(ctx context.Context, actor domain.RequestActor, sessionID string) (*domain.AssetWorkbenchUploadSession, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	lockedSessionID := strings.TrimSpace(sessionID)
	var session *domain.AssetWorkbenchUploadSession
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		locked, err := s.repo.GetUploadSessionForUpdate(ctx, tx, lockedSessionID)
		if err != nil {
			return err
		}
		if locked == nil {
			return sql.ErrNoRows
		}
		session = locked
		if session.OwnerUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "Upload session is not owned by current user.", nil)
		}
		switch session.Status {
		case domain.AssetWorkbenchUploadStatusCancelled:
			return nil
		case domain.AssetWorkbenchUploadStatusUploaded:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session is already uploaded and cannot be cancelled.", nil)
		case domain.AssetWorkbenchUploadStatusSubmitted:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session is already submitted.", nil)
		case domain.AssetWorkbenchUploadStatusExpired:
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session is already expired.", nil)
		}
		if s.oss != nil && s.oss.Enabled() {
			if strings.TrimSpace(session.UploadID) != "" {
				if err := s.oss.AbortMultipartUpload(ctx, session.ObjectKey, session.UploadID); err != nil {
					return domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to abort OSS multipart upload.", err.Error())
				}
			} else if err := s.oss.DeleteObject(ctx, session.ObjectKey); err != nil {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to delete OSS single-part upload.", err.Error())
			}
		}
		now := s.nowFn().UTC()
		if err := s.repo.UpdateUploadSessionStatus(ctx, tx, session.SessionID, domain.AssetWorkbenchUploadStatusCancelled, nil, &now, nil); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventUploadSessionUpdated, domain.AssetWorkbenchEntityUploadSession, &session.ID, session, map[string]interface{}{
			"session_id": session.SessionID,
			"status":     domain.AssetWorkbenchUploadStatusCancelled,
		}, "cancelled by user")
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, mapRepoReadError(err, "Upload session not found.", "Failed to load upload session.")
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to cancel upload session.", err.Error())
	}
	updated, err := s.repo.GetUploadSession(ctx, lockedSessionID)
	if err != nil {
		return nil, mapRepoReadError(err, "Upload session not found.", "Failed to reload upload session.")
	}
	return updated, nil
}

func (s *Service) ExpireUploadSessions(ctx context.Context, limit int) (int, *domain.AppError) {
	if s.repo == nil || s.tx == nil {
		return 0, nil
	}
	now := s.nowFn().UTC()
	sessions, err := s.repo.ListExpiredUploadSessions(ctx, now, limit)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list expired upload sessions.", err.Error())
	}
	expired := 0
	for _, candidate := range sessions {
		if candidate == nil {
			continue
		}
		markedExpired := false
		if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
			session, err := s.repo.GetUploadSessionForUpdate(ctx, tx, candidate.SessionID)
			if err != nil {
				return err
			}
			if session == nil {
				return sql.ErrNoRows
			}
			if (session.Status != domain.AssetWorkbenchUploadStatusCreated && session.Status != domain.AssetWorkbenchUploadStatusUploading) || session.ExpiresAt.After(now) {
				return nil
			}
			if s.oss != nil && s.oss.Enabled() {
				if strings.TrimSpace(session.UploadID) != "" {
					if err := s.oss.AbortMultipartUpload(ctx, session.ObjectKey, session.UploadID); err != nil {
						return err
					}
				} else if err := s.oss.DeleteObject(ctx, session.ObjectKey); err != nil {
					return err
				}
			}
			if err := s.repo.UpdateUploadSessionStatus(ctx, tx, session.SessionID, domain.AssetWorkbenchUploadStatusExpired, nil, &now, nil); err != nil {
				return err
			}
			if err := s.appendEvent(ctx, tx, domain.RequestActor{}, domain.AssetWorkbenchEventUploadSessionUpdated, domain.AssetWorkbenchEntityUploadSession, &session.ID, session, map[string]interface{}{
				"session_id": session.SessionID,
				"status":     domain.AssetWorkbenchUploadStatusExpired,
			}, "upload session expired"); err != nil {
				return err
			}
			markedExpired = true
			return nil
		}); err != nil {
			continue
		}
		if markedExpired {
			expired++
		}
	}
	return expired, nil
}

func (s *Service) CreateSubmission(ctx context.Context, actor domain.RequestActor, params CreateSubmissionParams) (*SubmissionDetail, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset submitters can create submissions.", nil)
	}
	if len(params.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "items are required.", nil)
	}
	profile, err := s.repo.GetProfileByUserID(ctx, actor.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench profile.", err.Error())
	}
	if profile == nil {
		profile = &domain.AssetWorkbenchProfile{UserID: actor.ID, Status: domain.AssetWorkbenchProfileStatusPending}
	}
	now := s.nowFn().UTC()
	businessMonth, appErr := s.resolveSubmissionBusinessMonth(ctx, actor, params, now)
	if appErr != nil {
		return nil, appErr
	}
	submission := &domain.AssetWorkbenchSubmission{
		SubmissionNo:    "AW" + now.Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", "")),
		SubmitterUserID: actor.ID,
		BusinessMonth:   businessMonth,
		SubmittedAt:     now,
		Status:          domain.AssetWorkbenchSubmissionStatusSubmitted,
		Notes:           strings.TrimSpace(params.Notes),
	}
	detail := &SubmissionDetail{}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		createdSubmission, err := s.repo.CreateSubmission(ctx, tx, submission)
		if err != nil {
			return err
		}
		detail.Submission = createdSubmission
		for _, reqItem := range params.Items {
			uploadSessions, inferredDifficulty, appErr := s.loadSubmissionUploadSessions(ctx, actor, reqItem)
			if appErr != nil {
				return appErr
			}
			if strings.TrimSpace(reqItem.DifficultyClass) == "" && inferredDifficulty != "" {
				reqItem.DifficultyClass = inferredDifficulty
			}
			template, appErr := s.resolveSubmissionTemplate(ctx, actor, profile, reqItem)
			if appErr != nil {
				return appErr
			}
			if template != nil && inferredDifficulty != "" && strings.TrimSpace(template.DifficultyClass) != inferredDifficulty {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "Template difficulty_class does not match the upload directory.", map[string]string{
					"template_difficulty_class":         template.DifficultyClass,
					"upload_directory_difficulty_class": inferredDifficulty,
				})
			}
			effectiveDifficulty := strings.TrimSpace(reqItem.DifficultyClass)
			if template != nil {
				effectiveDifficulty = strings.TrimSpace(template.DifficultyClass)
			}
			if appErr := s.ensureDifficultyClass(ctx, effectiveDifficulty, false); appErr != nil {
				return appErr
			}
			item, appErr := s.buildSubmissionItem(ctx, actor.ID, createdSubmission.ID, now, businessMonth, profile, reqItem, template)
			if appErr != nil {
				return appErr
			}
			createdItem, err := s.repo.CreateSubmissionItem(ctx, tx, item)
			if err != nil {
				return err
			}
			itemDetail := SubmissionItemDetail{Item: createdItem}
			for index, session := range uploadSessions {
				file := submissionFileFromUploadSession(createdSubmission.ID, createdItem.ID, actor.ID, session, index, now)
				createdFile, err := s.repo.CreateSubmissionFile(ctx, tx, file)
				if err != nil {
					return err
				}
				itemDetail.Files = append(itemDetail.Files, createdFile)
				if err := s.repo.UpdateUploadSessionStatus(ctx, tx, session.SessionID, domain.AssetWorkbenchUploadStatusSubmitted, nil, nil, &createdItem.ID); err != nil {
					return err
				}
			}
			detail.Items = append(detail.Items, itemDetail)
		}
		if err := s.repo.RefreshSubmissionTotals(ctx, tx, createdSubmission.ID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSubmissionCreated, domain.AssetWorkbenchEntitySubmission, &createdSubmission.ID, nil, map[string]interface{}{
			"submission": createdSubmission,
			"item_count": len(detail.Items),
		}, params.Notes)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench submission.", err.Error())
	}
	s.notifySubmissionCreated(ctx, actor, profile, detail)
	return detail, nil
}

func submissionFileFromUploadSession(submissionID, itemID, ownerUserID int64, session *domain.AssetWorkbenchUploadSession, sortOrder int, fallbackTime time.Time) *domain.AssetWorkbenchSubmissionFile {
	uploadedAt := fallbackTime
	if session.UploadedAt != nil && !session.UploadedAt.IsZero() {
		uploadedAt = session.UploadedAt.UTC()
	}
	return &domain.AssetWorkbenchSubmissionFile{
		SubmissionID:                   submissionID,
		SubmissionItemID:               itemID,
		UploadSessionID:                &session.ID,
		OwnerUserID:                    ownerUserID,
		UploadDirectoryID:              session.UploadDirectoryID,
		UploadDirectoryName:            session.UploadDirectoryName,
		UploadDirectoryPrefix:          session.UploadDirectoryPrefix,
		UploadDirectoryDifficultyClass: session.UploadDirectoryDifficultyClass,
		UploadBatchID:                  session.UploadBatchID,
		RelativePath:                   session.RelativePath,
		DisplayName:                    path.Base(firstNonEmpty(session.RelativePath, session.OriginalFilename)),
		IsFolderUpload:                 session.IsFolderUpload,
		ObjectKey:                      session.ObjectKey,
		PreviewStatus:                  initialPreviewStatus(session.OriginalFilename, session.MimeType),
		OriginalFilename:               session.OriginalFilename,
		FileExt:                        strings.TrimPrefix(strings.ToLower(filepath.Ext(session.OriginalFilename)), "."),
		FileType:                       inferFileType(session.OriginalFilename, session.MimeType),
		MimeType:                       session.MimeType,
		FileSize:                       session.FileSize,
		FileHash:                       session.FileHash,
		SortOrder:                      sortOrder,
		CreatedAt:                      uploadedAt,
		UpdatedAt:                      uploadedAt,
	}
}

func (s *Service) loadSubmissionUploadSessions(ctx context.Context, actor domain.RequestActor, req CreateSubmissionItemParams) ([]*domain.AssetWorkbenchUploadSession, string, *domain.AppError) {
	sessions := make([]*domain.AssetWorkbenchUploadSession, 0, len(req.UploadSessionIDs))
	difficulty := strings.TrimSpace(req.DifficultyClass)
	for _, rawSessionID := range req.UploadSessionIDs {
		sessionID := strings.TrimSpace(rawSessionID)
		if sessionID == "" {
			return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session_ids cannot contain empty values.", nil)
		}
		session, err := s.repo.GetUploadSession(ctx, sessionID)
		if err != nil {
			return nil, "", mapRepoReadError(err, "Upload session not found.", "Failed to load upload session.")
		}
		if session.OwnerUserID != actor.ID {
			return nil, "", domain.NewAppError(domain.ErrCodePermissionDenied, "Upload session is not owned by current user.", nil)
		}
		if session.Status != domain.AssetWorkbenchUploadStatusUploaded {
			return nil, "", domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Upload session must be uploaded before submission.", map[string]string{"session_id": session.SessionID})
		}
		sessionDifficulty := strings.TrimSpace(session.UploadDirectoryDifficultyClass)
		if sessionDifficulty != "" {
			normalizedDifficulty, appErr := normalizeWorkbenchDifficultyCode(sessionDifficulty, false)
			if appErr != nil {
				return nil, "", appErr
			}
			sessionDifficulty = normalizedDifficulty
			if difficulty == "" {
				difficulty = sessionDifficulty
			} else if difficulty != sessionDifficulty {
				return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "Upload sessions in one item must use the same upload directory difficulty_class.", map[string]string{
					"session_id":       session.SessionID,
					"difficulty_class": sessionDifficulty,
					"item_difficulty":  difficulty,
				})
			}
		}
		sessions = append(sessions, session)
	}
	return sessions, difficulty, nil
}

func (s *Service) ListSubmissions(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchSubmissionFilter) ([]*domain.AssetWorkbenchSubmission, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		filter.SubmitterUserID = &actor.ID
	}
	items, total, err := s.repo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench submissions.", err.Error())
	}
	return items, total, nil
}

func (s *Service) GetSubmissionDetail(ctx context.Context, actor domain.RequestActor, submissionID int64) (*SubmissionDetail, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if submissionID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "submission_id is required.", nil)
	}
	submission, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, mapRepoReadError(err, "Submission not found.", "Failed to load asset workbench submission.")
	}
	if submission.SubmitterUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Submission is not visible to current user.", nil)
	}
	items, err := s.repo.ListSubmissionItems(ctx, submission.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench submission items.", err.Error())
	}
	detail := &SubmissionDetail{
		Submission: submission,
		Items:      make([]SubmissionItemDetail, 0, len(items)),
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		files, err := s.repo.ListSubmissionFiles(ctx, item.ID)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench submission files.", err.Error())
		}
		detail.Items = append(detail.Items, SubmissionItemDetail{Item: item, Files: files})
	}
	return detail, nil
}

func (s *Service) VoidSubmission(ctx context.Context, actor domain.RequestActor, submissionID int64, params VoidSubmissionParams) (*domain.AssetWorkbenchSubmission, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can void submissions.", nil)
	}
	if submissionID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "submission_id is required.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when voiding submission.", nil)
	}
	var updated *domain.AssetWorkbenchSubmission
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		before, err := s.repo.GetSubmissionForUpdate(ctx, tx, submissionID)
		if err != nil {
			return err
		}
		updated, err = s.repo.VoidSubmission(ctx, tx, submissionID, actor.ID, reason, s.nowFn().UTC())
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSubmissionVoided, domain.AssetWorkbenchEntitySubmission, &submissionID, before, updated, reason)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "Submission not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to void submission.", err.Error())
	}
	return updated, nil
}

func (s *Service) UpdateSubmissionItemQC(ctx context.Context, actor domain.RequestActor, itemID int64, params UpdateSubmissionItemQCParams) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can update item QC status.", nil)
	}
	status := strings.TrimSpace(params.QCStatus)
	switch status {
	case domain.AssetWorkbenchSubmissionStatusSubmitted, domain.AssetWorkbenchSubmissionStatusChecked, domain.AssetWorkbenchSubmissionStatusNeedsFix:
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "qc_status must be submitted, checked or needs_fix.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if status == domain.AssetWorkbenchSubmissionStatusNeedsFix && reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when marking item as needs_fix.", nil)
	}
	before, appErr := s.loadMutableSubmissionItem(ctx, itemID)
	if appErr != nil {
		return nil, appErr
	}
	var updated *domain.AssetWorkbenchSubmissionItem
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateSubmissionItemQCStatus(ctx, tx, itemID, status)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemQCUpdated, domain.AssetWorkbenchEntitySubmissionItem, &itemID, before, updated, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update item QC status.", err.Error())
	}
	if s.notifications != nil && before.PayeeUserID > 0 {
		notificationAt := s.nowFn().UTC()
		payload := mustJSON(map[string]interface{}{
			"source":          "asset_workbench",
			"action":          "open_drive",
			"item_id":         before.ID,
			"submission_id":   before.SubmissionID,
			"payee_user_id":   before.PayeeUserID,
			"qc_status":       updated.QCStatus,
			"reason":          reason,
			"reviewed_by":     actor.ID,
			"reviewed_at_utc": notificationAt,
		})
		_, _, _ = s.notifications.CreateDedupedNotification(
			ctx,
			before.PayeeUserID,
			domain.NotificationTypeAssetWorkbenchQCUpdated,
			payload,
			"asset_workbench_qc_updated",
			fmt.Sprintf("asset_workbench_qc_updated:%d:%s:%d", before.ID, updated.QCStatus, notificationAt.UnixNano()),
		)
	}
	return updated, nil
}

func (s *Service) ImportSubmissionItemQCExcel(ctx context.Context, actor domain.RequestActor, businessMonth string, reader io.Reader) (*SubmissionItemQCImportResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can import item QC status.", nil)
	}
	rows, appErr := parseSubmissionItemQCExcel(businessMonth, reader)
	if appErr != nil {
		return nil, appErr
	}
	indexes := newSubmissionItemQCImportIndexes()
	indexOptions := submissionItemQCImportIndexOptionsForRows(rows)
	businessMonth = strings.TrimSpace(businessMonth)
	if businessMonth != "" {
		items, err := s.repo.ListSubmissionItemsByMonth(ctx, businessMonth)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission items for QC import.", err.Error())
		}
		var buildErr *domain.AppError
		indexes, buildErr = s.buildSubmissionItemQCImportIndexes(ctx, items, indexOptions)
		if buildErr != nil {
			return nil, buildErr
		}
	}
	result := &SubmissionItemQCImportResult{
		Updated:  []*domain.AssetWorkbenchSubmissionItem{},
		Failures: []SubmissionItemQCImportFailure{},
	}
	for _, row := range rows {
		itemID, reason := indexes.match(row)
		if itemID <= 0 {
			if reason == "" {
				reason = "item_id, file_id, submission_no+filename, filename, or order_no is required"
			}
			result.Failures = append(result.Failures, SubmissionItemQCImportFailure{Row: row.row, Reason: reason})
			continue
		}
		updated, appErr := s.UpdateSubmissionItemQC(ctx, actor, itemID, UpdateSubmissionItemQCParams{QCStatus: row.qcStatus, Reason: row.reason})
		if appErr != nil {
			result.Failures = append(result.Failures, SubmissionItemQCImportFailure{Row: row.row, Reason: appErr.Message})
			continue
		}
		result.Updated = append(result.Updated, updated)
	}
	return result, nil
}

type submissionItemQCImportIndexes struct {
	orderNoToItemID         map[string]int64
	fileIDToItemID          map[int64]int64
	submissionFileToItemID  map[string]int64
	filenameToItemID        map[string]int64
	ambiguousSubmissionFile map[string]bool
	ambiguousFilename       map[string]bool
}

type submissionItemQCImportIndexOptions struct {
	includeFiles         bool
	includeSubmissionNos bool
}

func submissionItemQCImportIndexOptionsForRows(rows []submissionItemQCExcelRow) submissionItemQCImportIndexOptions {
	var options submissionItemQCImportIndexOptions
	for _, row := range rows {
		if row.fileID > 0 || strings.TrimSpace(row.filename) != "" {
			options.includeFiles = true
		}
		if strings.TrimSpace(row.submissionNo) != "" && strings.TrimSpace(row.filename) != "" {
			options.includeFiles = true
			options.includeSubmissionNos = true
		}
	}
	return options
}

func newSubmissionItemQCImportIndexes() *submissionItemQCImportIndexes {
	return &submissionItemQCImportIndexes{
		orderNoToItemID:         map[string]int64{},
		fileIDToItemID:          map[int64]int64{},
		submissionFileToItemID:  map[string]int64{},
		filenameToItemID:        map[string]int64{},
		ambiguousSubmissionFile: map[string]bool{},
		ambiguousFilename:       map[string]bool{},
	}
}

func (s *Service) buildSubmissionItemQCImportIndexes(ctx context.Context, items []*domain.AssetWorkbenchSubmissionItem, options submissionItemQCImportIndexOptions) (*submissionItemQCImportIndexes, *domain.AppError) {
	indexes := newSubmissionItemQCImportIndexes()
	orderDuplicates := map[string]bool{}
	submissionNoByID := map[int64]string{}
	for _, item := range items {
		if item == nil {
			continue
		}
		orderKey := strings.TrimSpace(item.OrderNo)
		if orderKey != "" {
			if existing, ok := indexes.orderNoToItemID[orderKey]; ok && existing != item.ID {
				orderDuplicates[orderKey] = true
			} else {
				indexes.orderNoToItemID[orderKey] = item.ID
			}
		}
		if options.includeSubmissionNos {
			if _, ok := submissionNoByID[item.SubmissionID]; !ok {
				submission, err := s.repo.GetSubmission(ctx, item.SubmissionID)
				if err != nil {
					return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission records for QC import.", err.Error())
				}
				submissionNoByID[item.SubmissionID] = strings.TrimSpace(submission.SubmissionNo)
			}
		}
		if !options.includeFiles {
			continue
		}
		files, err := s.repo.ListSubmissionFiles(ctx, item.ID)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission files for QC import.", err.Error())
		}
		for _, file := range files {
			if file == nil {
				continue
			}
			if file.ID > 0 {
				indexes.fileIDToItemID[file.ID] = item.ID
			}
			filenameKey := normalizeSubmissionItemQCImportKey(file.OriginalFilename)
			if filenameKey == "" {
				continue
			}
			indexes.addUniqueFilename(filenameKey, item.ID)
			if !options.includeSubmissionNos {
				continue
			}
			if submissionNo := normalizeSubmissionItemQCImportKey(submissionNoByID[item.SubmissionID]); submissionNo != "" {
				indexes.addUniqueSubmissionFile(submissionNo+"|"+filenameKey, item.ID)
			}
		}
	}
	for key := range orderDuplicates {
		delete(indexes.orderNoToItemID, key)
	}
	return indexes, nil
}

func (idx *submissionItemQCImportIndexes) addUniqueSubmissionFile(key string, itemID int64) {
	if key == "" {
		return
	}
	if existing, ok := idx.submissionFileToItemID[key]; ok && existing != itemID {
		idx.ambiguousSubmissionFile[key] = true
		delete(idx.submissionFileToItemID, key)
		return
	}
	if !idx.ambiguousSubmissionFile[key] {
		idx.submissionFileToItemID[key] = itemID
	}
}

func (idx *submissionItemQCImportIndexes) addUniqueFilename(key string, itemID int64) {
	if key == "" {
		return
	}
	if existing, ok := idx.filenameToItemID[key]; ok && existing != itemID {
		idx.ambiguousFilename[key] = true
		delete(idx.filenameToItemID, key)
		return
	}
	if !idx.ambiguousFilename[key] {
		idx.filenameToItemID[key] = itemID
	}
}

func (idx *submissionItemQCImportIndexes) match(row submissionItemQCExcelRow) (int64, string) {
	if row.itemID > 0 {
		return row.itemID, ""
	}
	if row.fileID > 0 {
		if itemID := idx.fileIDToItemID[row.fileID]; itemID > 0 {
			return itemID, ""
		}
		return 0, "file_id is not found in business_month"
	}
	filename := normalizeSubmissionItemQCImportKey(row.filename)
	submissionNo := normalizeSubmissionItemQCImportKey(row.submissionNo)
	if submissionNo != "" && filename != "" {
		key := submissionNo + "|" + filename
		if itemID := idx.submissionFileToItemID[key]; itemID > 0 {
			return itemID, ""
		}
		return 0, "submission_no and filename are not unique or not found in business_month"
	}
	if filename != "" {
		if itemID := idx.filenameToItemID[filename]; itemID > 0 {
			return itemID, ""
		}
		return 0, "filename is not unique or not found in business_month"
	}
	if row.orderNo != "" {
		if itemID := idx.orderNoToItemID[row.orderNo]; itemID > 0 {
			return itemID, ""
		}
		return 0, "order_no is not unique or not found in business_month"
	}
	return 0, ""
}

func normalizeSubmissionItemQCImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Service) VoidSubmissionItem(ctx context.Context, actor domain.RequestActor, itemID int64, params VoidSubmissionItemParams) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can void items.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when voiding item.", nil)
	}
	before, appErr := s.loadMutableSubmissionItem(ctx, itemID)
	if appErr != nil {
		return nil, appErr
	}
	var updated *domain.AssetWorkbenchSubmissionItem
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.VoidSubmissionItem(ctx, tx, itemID, actor.ID, reason, s.nowFn().UTC())
		if err != nil {
			return err
		}
		if err := s.repo.RefreshSubmissionTotals(ctx, tx, before.SubmissionID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemVoided, domain.AssetWorkbenchEntitySubmissionItem, &itemID, before, updated, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to void item.", err.Error())
	}
	return updated, nil
}

func (s *Service) RepriceSubmissionItem(ctx context.Context, actor domain.RequestActor, itemID int64, params RepriceSubmissionItemParams) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can reprice items.", nil)
	}
	before, appErr := s.loadMutableSubmissionItem(ctx, itemID)
	if appErr != nil {
		return nil, appErr
	}
	pricingProfile, appErr := s.pricingProfileForItem(ctx, before)
	if appErr != nil {
		return nil, appErr
	}
	repriced, appErr := s.buildSubmissionItem(ctx, before.PayeeUserID, before.SubmissionID, before.SubmittedAt, before.BusinessMonth, pricingProfile, CreateSubmissionItemParams{
		OrderNo:         before.OrderNo,
		DifficultyClass: before.DifficultyClass,
		Finalized:       before.Finalized,
		PageCount:       before.PageCount,
		ItemCount:       before.ItemCount,
	}, nil)
	if appErr != nil {
		return nil, appErr
	}
	repriced.ID = before.ID
	repriced.QCStatus = before.QCStatus
	repriced.SettlementStatus = before.SettlementStatus
	repriced.CurrentSettlementBatchID = before.CurrentSettlementBatchID
	var updated *domain.AssetWorkbenchSubmissionItem
	reason := strings.TrimSpace(params.Reason)
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateSubmissionItemPricing(ctx, tx, repriced)
		if err != nil {
			return err
		}
		if err := s.repo.RefreshSubmissionTotals(ctx, tx, before.SubmissionID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemRepriced, domain.AssetWorkbenchEntitySubmissionItem, &itemID, before, updated, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to reprice item.", err.Error())
	}
	return updated, nil
}

func (s *Service) UpdateSubmissionItem(ctx context.Context, actor domain.RequestActor, itemID int64, params UpdateSubmissionItemParams) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can edit submission items.", nil)
	}
	before, appErr := s.loadMutableSubmissionItem(ctx, itemID)
	if appErr != nil {
		return nil, appErr
	}
	req := CreateSubmissionItemParams{
		OrderNo:         before.OrderNo,
		DifficultyClass: before.DifficultyClass,
		Finalized:       before.Finalized,
		PageCount:       before.PageCount,
		ItemCount:       before.ItemCount,
	}
	if params.OrderNo != nil {
		req.OrderNo = strings.TrimSpace(*params.OrderNo)
	}
	if params.DifficultyClass != nil {
		req.DifficultyClass = strings.TrimSpace(*params.DifficultyClass)
	}
	if params.Finalized != nil {
		req.Finalized = *params.Finalized
	}
	if params.PageCount != nil {
		req.PageCount = *params.PageCount
	}
	if req.PageCount <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_count must be greater than zero.", nil)
	}
	normalizedDifficulty, appErr := normalizeWorkbenchDifficultyCode(req.DifficultyClass, false)
	if appErr != nil {
		return nil, appErr
	}
	req.DifficultyClass = normalizedDifficulty
	if appErr := s.ensureDifficultyClass(ctx, req.DifficultyClass, false); appErr != nil {
		return nil, appErr
	}
	pricingProfile, appErr := s.pricingProfileForItem(ctx, before)
	if appErr != nil {
		return nil, appErr
	}
	next, appErr := s.buildSubmissionItem(ctx, before.PayeeUserID, before.SubmissionID, before.SubmittedAt, before.BusinessMonth, pricingProfile, req, nil)
	if appErr != nil {
		return nil, appErr
	}
	next.ID = before.ID
	next.TemplateID = before.TemplateID
	next.TemplateNameSnapshot = before.TemplateNameSnapshot
	next.CategorySnapshot = before.CategorySnapshot
	next.QCStatus = before.QCStatus
	next.SettlementStatus = before.SettlementStatus
	next.CurrentSettlementBatchID = before.CurrentSettlementBatchID
	next.VoidedAt = before.VoidedAt
	next.VoidedBy = before.VoidedBy
	next.VoidReason = before.VoidReason
	var updated *domain.AssetWorkbenchSubmissionItem
	reason := strings.TrimSpace(params.Reason)
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateSubmissionItemEditableFields(ctx, tx, next)
		if err != nil {
			return err
		}
		if err := s.repo.RefreshSubmissionTotals(ctx, tx, before.SubmissionID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemUpdated, domain.AssetWorkbenchEntitySubmissionItem, &itemID, before, updated, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to edit submission item.", err.Error())
	}
	return updated, nil
}

func (s *Service) pricingProfileForItem(ctx context.Context, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	profile := &domain.AssetWorkbenchProfile{
		UserID:     item.PayeeUserID,
		WorkerType: strings.TrimSpace(item.WorkerTypeSnapshot),
		JobGrade:   strings.TrimSpace(item.JobGradeSnapshot),
	}
	if profile.WorkerType != "" && profile.JobGrade != "" {
		return profile, nil
	}
	current, err := s.repo.GetProfileByUserID(ctx, item.PayeeUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return profile, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load current asset workbench profile for pricing.", err.Error())
	}
	if profile.WorkerType == "" {
		profile.WorkerType = strings.TrimSpace(current.WorkerType)
	}
	if profile.JobGrade == "" {
		profile.JobGrade = strings.TrimSpace(current.JobGrade)
	}
	return profile, nil
}

func (s *Service) BatchMoveFiles(ctx context.Context, actor domain.RequestActor, params BatchMoveFilesParams) (*BatchFileMutationResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers can move submission files.", nil)
	}
	fileIDs := positiveUniqueInt64s(params.FileIDs)
	if len(fileIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_ids is required.", nil)
	}
	if params.UploadDirectoryID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_directory_id is required.", nil)
	}
	directory, err := s.repo.GetUploadDirectory(ctx, params.UploadDirectoryID)
	if err != nil {
		return nil, mapRepoReadError(err, "Upload directory not found.", "Failed to load upload directory.")
	}
	if directory == nil || !directory.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_directory_id is not enabled.", map[string]interface{}{"upload_directory_id": params.UploadDirectoryID})
	}
	files, err := s.repo.ListSubmissionFilesByIDs(ctx, fileIDs)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission files.", err.Error())
	}
	byID := map[int64]*domain.AssetWorkbenchSubmissionFile{}
	for _, file := range files {
		byID[file.ID] = file
	}
	result := &BatchFileMutationResult{}
	groups := s.completeSubmissionFileMutationGroups(ctx, fileIDs, byID, result)
	for _, group := range groups {
		moved, appErr := s.moveSubmissionFileGroup(ctx, actor, group, directory, strings.TrimSpace(params.Reason))
		if appErr != nil {
			appendBatchFileMutationFailures(result, group, appErr.Message)
			continue
		}
		result.Files = append(result.Files, moved...)
	}
	orderBatchFileMutationFailures(result, fileIDs)
	return result, nil
}

func (s *Service) BatchDeleteFiles(ctx context.Context, actor domain.RequestActor, params BatchDeleteFilesParams) (*BatchFileMutationResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench members can delete submission files.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when deleting files.", nil)
	}
	fileIDs := positiveUniqueInt64s(params.FileIDs)
	if len(fileIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_ids is required.", nil)
	}
	files, err := s.repo.ListSubmissionFilesByIDs(ctx, fileIDs)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission files.", err.Error())
	}
	byID := map[int64]*domain.AssetWorkbenchSubmissionFile{}
	for _, file := range files {
		byID[file.ID] = file
	}
	result := &BatchFileMutationResult{}
	groups := s.completeSubmissionFileMutationGroups(ctx, fileIDs, byID, result)
	for _, group := range groups {
		if appErr := s.deleteSubmissionFileGroup(ctx, actor, group, reason); appErr != nil {
			appendBatchFileMutationFailures(result, group, appErr.Message)
			continue
		}
		for _, file := range group {
			result.Deleted = append(result.Deleted, file.ID)
		}
	}
	orderBatchFileMutationFailures(result, fileIDs)
	return result, nil
}

func (s *Service) completeSubmissionFileMutationGroups(ctx context.Context, fileIDs []int64, byID map[int64]*domain.AssetWorkbenchSubmissionFile, result *BatchFileMutationResult) [][]*domain.AssetWorkbenchSubmissionFile {
	selectedByItem := map[int64][]*domain.AssetWorkbenchSubmissionFile{}
	itemOrder := make([]int64, 0)
	for _, fileID := range fileIDs {
		file := byID[fileID]
		if file == nil {
			result.Failures = append(result.Failures, BatchFileMutationFailure{FileID: fileID, Reason: "file not found"})
			continue
		}
		if _, exists := selectedByItem[file.SubmissionItemID]; !exists {
			itemOrder = append(itemOrder, file.SubmissionItemID)
		}
		selectedByItem[file.SubmissionItemID] = append(selectedByItem[file.SubmissionItemID], file)
	}

	groups := make([][]*domain.AssetWorkbenchSubmissionFile, 0, len(itemOrder))
	for _, itemID := range itemOrder {
		selected := selectedByItem[itemID]
		active, err := s.repo.ListSubmissionFiles(ctx, itemID)
		if err != nil {
			appendBatchFileMutationFailures(result, selected, "Failed to load all files in the priced work.")
			continue
		}
		selectedIDs := make(map[int64]struct{}, len(selected))
		for _, file := range selected {
			selectedIDs[file.ID] = struct{}{}
		}
		complete := len(active) > 0 && len(active) == len(selectedIDs)
		if complete {
			for _, file := range active {
				if file == nil {
					continue
				}
				if _, ok := selectedIDs[file.ID]; !ok {
					complete = false
					break
				}
			}
		}
		if !complete {
			appendBatchFileMutationFailures(result, selected, "All files in one priced work must be selected together.")
			continue
		}
		groups = append(groups, active)
	}
	return groups
}

func appendBatchFileMutationFailures(result *BatchFileMutationResult, files []*domain.AssetWorkbenchSubmissionFile, reason string) {
	for _, file := range files {
		if file == nil {
			continue
		}
		result.Failures = append(result.Failures, BatchFileMutationFailure{FileID: file.ID, Reason: reason})
	}
}

func orderBatchFileMutationFailures(result *BatchFileMutationResult, fileIDs []int64) {
	order := make(map[int64]int, len(fileIDs))
	for index, fileID := range fileIDs {
		if _, exists := order[fileID]; !exists {
			order[fileID] = index
		}
	}
	sort.SliceStable(result.Failures, func(i, j int) bool {
		return order[result.Failures[i].FileID] < order[result.Failures[j].FileID]
	})
}

func (s *Service) moveSubmissionFileGroup(ctx context.Context, actor domain.RequestActor, files []*domain.AssetWorkbenchSubmissionFile, directory *domain.AssetWorkbenchUploadDirectory, reason string) ([]*domain.AssetWorkbenchSubmissionFile, *domain.AppError) {
	if len(files) == 0 || directory == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "files and upload directory are required.", nil)
	}
	item, appErr := s.loadMutableSubmissionItem(ctx, files[0].SubmissionItemID)
	if appErr != nil {
		return nil, appErr
	}
	if s.oss == nil || !s.oss.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "OSS direct move is not enabled.", nil)
	}
	repriced, appErr := s.buildSubmissionItemForDifficulty(ctx, item, directory.DifficultyClass)
	if appErr != nil {
		return nil, appErr
	}

	now := s.nowFn().UTC()
	nextFiles := make([]*domain.AssetWorkbenchSubmissionFile, 0, len(files))
	copiedKeys := make([]string, 0, len(files))
	oldKeys := make([]string, 0, len(files))
	cleanupCopied := func() {
		for _, key := range copiedKeys {
			_ = s.oss.DeleteObject(ctx, key)
		}
	}
	for _, file := range files {
		if file == nil || file.SubmissionItemID != item.ID {
			cleanupCopied()
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "All files must belong to the same priced work.", nil)
		}
		oldKey := strings.TrimSpace(file.ObjectKey)
		if oldKey == "" {
			cleanupCopied()
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "submission file object key is empty.", map[string]interface{}{"file_id": file.ID})
		}
		next := *file
		directoryID := directory.ID
		next.UploadDirectoryID = &directoryID
		next.UploadDirectoryName = directory.Name
		next.UploadDirectoryPrefix = directory.OSSPrefix
		next.UploadDirectoryDifficultyClass = directory.DifficultyClass
		next.ObjectKey = s.buildMovedFileObjectKey(now, file, directory)
		if strings.TrimSpace(next.PreviewKey) == oldKey {
			next.PreviewKey = next.ObjectKey
		}
		if next.ObjectKey != oldKey {
			if err := s.oss.CopyObject(ctx, oldKey, next.ObjectKey); err != nil {
				cleanupCopied()
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to copy file to target upload directory.", err.Error())
			}
			copiedKeys = append(copiedKeys, next.ObjectKey)
			oldKeys = append(oldKeys, oldKey)
		}
		nextFiles = append(nextFiles, &next)
	}

	updatedFiles := make([]*domain.AssetWorkbenchSubmissionFile, 0, len(nextFiles))
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		for index, next := range nextFiles {
			updated, err := s.repo.UpdateSubmissionFileLocation(ctx, tx, next)
			if err != nil {
				return err
			}
			updatedFiles = append(updatedFiles, updated)
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventFileMoved, domain.AssetWorkbenchEntitySubmissionFile, &next.ID, files[index], updated, reason); err != nil {
				return err
			}
		}
		updatedItem, err := s.repo.UpdateSubmissionItemEditableFields(ctx, tx, repriced)
		if err != nil {
			return err
		}
		if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemRepriced, domain.AssetWorkbenchEntitySubmissionItem, &item.ID, item, updatedItem, reason); err != nil {
			return err
		}
		return s.repo.RefreshSubmissionTotals(ctx, tx, item.SubmissionID)
	}); err != nil {
		cleanupCopied()
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to move submission files.", err.Error())
	}
	for _, oldKey := range oldKeys {
		_ = s.oss.DeleteObject(ctx, oldKey)
	}
	return updatedFiles, nil
}

func (s *Service) buildSubmissionItemForDifficulty(ctx context.Context, before *domain.AssetWorkbenchSubmissionItem, difficulty string) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	profile, appErr := s.pricingProfileForItem(ctx, before)
	if appErr != nil {
		return nil, appErr
	}
	templates := make([]*domain.AssetWorkbenchTemplate, 0, 1)
	if before.TemplateID != nil {
		templates = append(templates, &domain.AssetWorkbenchTemplate{
			ID:              *before.TemplateID,
			Name:            before.TemplateNameSnapshot,
			Category:        before.CategorySnapshot,
			DifficultyClass: difficulty,
		})
	}
	next, appErr := s.buildSubmissionItem(ctx, before.PayeeUserID, before.SubmissionID, before.SubmittedAt, before.BusinessMonth, profile, CreateSubmissionItemParams{
		OrderNo:         before.OrderNo,
		DifficultyClass: difficulty,
		Finalized:       before.Finalized,
		PageCount:       before.PageCount,
		ItemCount:       before.ItemCount,
	}, templates...)
	if appErr != nil {
		return nil, appErr
	}
	next.ID = before.ID
	next.TemplateID = before.TemplateID
	next.TemplateNameSnapshot = before.TemplateNameSnapshot
	next.CategorySnapshot = before.CategorySnapshot
	next.QCStatus = before.QCStatus
	next.SettlementStatus = before.SettlementStatus
	next.CurrentSettlementBatchID = before.CurrentSettlementBatchID
	next.VoidedAt = before.VoidedAt
	next.VoidedBy = before.VoidedBy
	next.VoidReason = before.VoidReason
	return next, nil
}

func (s *Service) deleteSubmissionFileGroup(ctx context.Context, actor domain.RequestActor, files []*domain.AssetWorkbenchSubmissionFile, reason string) *domain.AppError {
	if len(files) == 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "files are required.", nil)
	}
	for _, file := range files {
		if file == nil {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required.", nil)
		}
		if file.OwnerUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "Submission file is not owned by current user.", nil)
		}
	}
	item, appErr := s.loadMutableSubmissionItem(ctx, files[0].SubmissionItemID)
	if appErr != nil {
		return appErr
	}
	before := *item
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		now := s.nowFn().UTC()
		for _, file := range files {
			if file.SubmissionItemID != item.ID {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "All files must belong to the same priced work.", nil)
			}
			if err := s.repo.DeleteSubmissionFile(ctx, tx, file.ID, actor.ID, reason, now); err != nil {
				return err
			}
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventFileDeleted, domain.AssetWorkbenchEntitySubmissionFile, &file.ID, file, nil, reason); err != nil {
				return err
			}
		}
		updated, err := s.repo.VoidSubmissionItem(ctx, tx, item.ID, actor.ID, "all files deleted: "+reason, now)
		if err != nil {
			return err
		}
		if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemVoided, domain.AssetWorkbenchEntitySubmissionItem, &item.ID, &before, updated, reason); err != nil {
			return err
		}
		return s.repo.RefreshSubmissionTotals(ctx, tx, item.SubmissionID)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to delete submission file.", err.Error())
	}
	return nil
}

func (s *Service) UpdateSubmissionFileDisplayName(ctx context.Context, actor domain.RequestActor, fileID int64, displayName string) (*domain.AssetWorkbenchSubmissionFile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if fileID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_id is required.", nil)
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers can edit uploaded work names.", nil)
	}
	nextName := strings.TrimSpace(displayName)
	if nextName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "display_name is required.", nil)
	}
	if len([]rune(nextName)) > 180 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "display_name is too long.", map[string]interface{}{"max_length": 180})
	}
	file, err := s.repo.GetSubmissionFile(ctx, fileID)
	if err != nil {
		return nil, mapRepoReadError(err, "Submission file not found.", "Failed to load submission file.")
	}
	var updated *domain.AssetWorkbenchSubmissionFile
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateSubmissionFileDisplayName(ctx, tx, fileID, nextName)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventFileRenamed, domain.AssetWorkbenchEntitySubmissionFile, &fileID, file, updated, "rename uploaded work")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update uploaded work name.", err.Error())
	}
	return updated, nil
}

func (s *Service) GetFilePreview(ctx context.Context, actor domain.RequestActor, fileID int64) (*FilePreviewMeta, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if fileID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_id is required.", nil)
	}
	file, err := s.repo.GetSubmissionFile(ctx, fileID)
	if err != nil {
		return nil, mapRepoReadError(err, "Submission file not found.", "Failed to load submission file.")
	}
	if file.OwnerUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Submission file is not visible to current user.", nil)
	}
	meta := &FilePreviewMeta{
		FileID:   file.ID,
		Status:   file.PreviewStatus,
		MimeType: file.MimeType,
		Filename: firstNonEmpty(file.DisplayName, file.OriginalFilename),
		Preparing: file.PreviewStatus == domain.AssetWorkbenchPreviewStatusPending ||
			file.PreviewStatus == domain.AssetWorkbenchPreviewStatusProcessing,
		Error: file.PreviewError,
	}
	if file.PreviewStatus != domain.AssetWorkbenchPreviewStatusReady {
		return meta, nil
	}
	if s.oss == nil || !s.oss.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "OSS direct preview is not enabled.", nil)
	}
	previewKey := strings.TrimSpace(file.PreviewKey)
	if previewKey == "" {
		previewKey = file.ObjectKey
	}
	signed := s.oss.PresignPreviewURL(previewKey)
	if signed != nil {
		meta.PreviewURL = signed.DownloadURL
		meta.DownloadURL = signed.DownloadURL
		meta.ExpiresAt = &signed.ExpiresAt
		meta.PreviewAvailable = true
	}
	return meta, nil
}

func (s *Service) GetFileDownload(ctx context.Context, actor domain.RequestActor, fileID int64) (*FileDownloadMeta, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if fileID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_id is required.", nil)
	}
	file, err := s.repo.GetSubmissionFile(ctx, fileID)
	if err != nil {
		return nil, mapRepoReadError(err, "Submission file not found.", "Failed to load submission file.")
	}
	if appErr := s.requireFileVisible(actor, file); appErr != nil {
		return nil, appErr
	}
	meta, appErr := s.buildFileDownloadMeta(file, nil)
	if appErr != nil {
		return nil, appErr
	}
	_ = s.recordFileDownloadEvent(ctx, actor, domain.AssetWorkbenchEventFileDownloaded, &file.ID, map[string]interface{}{
		"file_id":       file.ID,
		"submission_id": file.SubmissionID,
		"item_id":       file.SubmissionItemID,
		"filename":      file.OriginalFilename,
	})
	return meta, nil
}

func (s *Service) BuildFileBatchDownloadManifest(ctx context.Context, actor domain.RequestActor, fileIDs []int64) (*FileBatchDownloadManifest, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if len(fileIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_ids is required.", nil)
	}
	const maxBatchFiles = 100
	if len(fileIDs) > maxBatchFiles {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_ids exceed batch download limit.", map[string]interface{}{"limit": maxBatchFiles})
	}
	manifest := &FileBatchDownloadManifest{
		Items:    make([]FileDownloadMeta, 0, len(fileIDs)),
		Failures: make([]FileBatchDownloadFail, 0),
	}
	seen := make(map[int64]struct{}, len(fileIDs))
	usedNames := make(map[string]int, len(fileIDs))
	for _, fileID := range fileIDs {
		if fileID <= 0 {
			manifest.Failures = append(manifest.Failures, FileBatchDownloadFail{FileID: fileID, Reason: "invalid_file_id"})
			continue
		}
		if _, ok := seen[fileID]; ok {
			manifest.Failures = append(manifest.Failures, FileBatchDownloadFail{FileID: fileID, Reason: "duplicate_file_id"})
			continue
		}
		seen[fileID] = struct{}{}
		file, err := s.repo.GetSubmissionFile(ctx, fileID)
		if err != nil || file == nil {
			manifest.Failures = append(manifest.Failures, FileBatchDownloadFail{FileID: fileID, Reason: "file_not_found"})
			continue
		}
		if appErr := s.requireFileVisible(actor, file); appErr != nil {
			manifest.Failures = append(manifest.Failures, FileBatchDownloadFail{FileID: fileID, Reason: "not_visible"})
			continue
		}
		meta, appErr := s.buildFileDownloadMeta(file, usedNames)
		if appErr != nil {
			manifest.Failures = append(manifest.Failures, FileBatchDownloadFail{FileID: fileID, Reason: appErr.Code})
			continue
		}
		manifest.Items = append(manifest.Items, *meta)
	}
	if len(manifest.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "No requested files are available for download.", manifest.Failures)
	}
	_ = s.recordFileDownloadEvent(ctx, actor, domain.AssetWorkbenchEventFileBatchDownloaded, nil, map[string]interface{}{
		"requested_count": len(fileIDs),
		"success_count":   len(manifest.Items),
		"failure_count":   len(manifest.Failures),
	})
	return manifest, nil
}

func (s *Service) ImportErrorRecords(ctx context.Context, actor domain.RequestActor, params ImportErrorRecordsParams) (*domain.AssetWorkbenchErrorImportBatch, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only managers or settlement roles can import error records.", nil)
	}
	businessMonth := strings.TrimSpace(params.BusinessMonth)
	if businessMonth == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month is required.", nil)
	}
	if len(params.Records) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "records are required.", nil)
	}
	var submissionItems []*domain.AssetWorkbenchSubmissionItem
	var difficultyIndex map[string]string
	matches := make([]errorRecordImportMatch, len(params.Records))
	matchedRows := 0
	unmatchedRows := 0
	ambiguousRows := 0
	for idx, input := range params.Records {
		var match errorRecordImportMatch
		var appErr *domain.AppError
		if isQualityErrorImportInput(input) {
			if difficultyIndex == nil {
				difficultyIndex, appErr = s.errorImportDifficultyIndex(ctx)
				if appErr != nil {
					return nil, appErr
				}
			}
			match, appErr = s.matchQualityErrorRecord(ctx, input, difficultyIndex)
			if appErr != nil {
				return nil, appErr
			}
		} else {
			if submissionItems == nil {
				var err error
				submissionItems, err = s.repo.ListSubmissionItemsByMonth(ctx, businessMonth)
				if err != nil {
					return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load submission items for error import matching.", err.Error())
				}
			}
			match = matchImportedErrorRecord(submissionItems, input)
		}
		matches[idx] = match
		switch match.Status {
		case domain.AssetWorkbenchErrorMatchStatusMatched:
			matchedRows++
		case domain.AssetWorkbenchErrorMatchStatusAmbiguous:
			ambiguousRows++
		default:
			unmatchedRows++
		}
	}
	var batch *domain.AssetWorkbenchErrorImportBatch
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		batch, err = s.repo.CreateErrorImportBatch(ctx, tx, &domain.AssetWorkbenchErrorImportBatch{
			ImportNo:         "AWE" + s.nowFn().UTC().Format("20060102150405") + strings.ToUpper(uuid.NewString()[:8]),
			BusinessMonth:    businessMonth,
			UploadedBy:       actor.ID,
			OriginalFilename: strings.TrimSpace(params.OriginalFilename),
			Status:           "imported",
			TotalRows:        len(params.Records),
			MatchedRows:      matchedRows,
			UnmatchedRows:    unmatchedRows,
			AmbiguousRows:    ambiguousRows,
		})
		if err != nil {
			return err
		}
		for idx, input := range params.Records {
			orderNo := strings.TrimSpace(input.OrderNo)
			if input.ErrorCount < 0 {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "error_count must be non-negative.", nil)
			}
			match := matches[idx]
			raw := errorRecordRawPayload(input, match)
			payeeUserID := input.PayeeUserID
			if match.PayeeUserID != nil {
				payeeUserID = match.PayeeUserID
			}
			difficultyClass := strings.TrimSpace(match.DifficultyClass)
			if difficultyClass == "" {
				difficultyClass = strings.TrimSpace(input.DifficultyClass)
			}
			if _, err := s.repo.CreateErrorRecord(ctx, tx, &domain.AssetWorkbenchErrorRecord{
				ImportBatchID:    batch.ID,
				BusinessMonth:    businessMonth,
				PayeeUserID:      payeeUserID,
				OrderNo:          orderNo,
				DifficultyClass:  difficultyClass,
				OccurredDate:     match.OccurredDate,
				ErrorCount:       input.ErrorCount,
				RawPayload:       raw,
				MatchStatus:      match.Status,
				SubmissionItemID: match.SubmissionItemID,
			}); err != nil {
				return err
			}
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventErrorImportCreated, domain.AssetWorkbenchEntityErrorImport, &batch.ID, nil, batch, params.OriginalFilename)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to import error records.", err.Error())
	}
	return batch, nil
}

func (s *Service) ImportErrorRecordsExcel(ctx context.Context, actor domain.RequestActor, businessMonth string, originalFilename string, reader io.Reader) (*domain.AssetWorkbenchErrorImportBatch, *domain.AppError) {
	if reader == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required.", nil)
	}
	records, appErr := parseErrorRecordsExcel(reader)
	if appErr != nil {
		return nil, appErr
	}
	return s.ImportErrorRecords(ctx, actor, ImportErrorRecordsParams{
		BusinessMonth:    businessMonth,
		OriginalFilename: originalFilename,
		Records:          records,
	})
}

func (s *Service) MySettlement(ctx context.Context, actor domain.RequestActor) (*MySettlementResponse, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, assetWorkbenchRolesForService()...) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Authentication required.", nil)
	}
	currentMonth := s.businessMonth(s.nowFn().UTC())
	rowsByMonth := map[string]*MySettlementMonthRow{}
	ensureRow := func(month string) *MySettlementMonthRow {
		row := rowsByMonth[month]
		if row == nil {
			row = &MySettlementMonthRow{BusinessMonth: month}
			rowsByMonth[month] = row
		}
		return row
	}

	confirmedItems, err := s.repo.ListConfirmedSettlementItemsByPayee(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load confirmed settlement items.", err.Error())
	}
	for _, item := range confirmedItems {
		if item == nil {
			continue
		}
		row := ensureRow(item.BusinessMonth)
		row.Confirmed = true
		switch item.ItemType {
		case domain.AssetWorkbenchItemTypeGrossPiecework:
			row.ItemCount++
			row.PageCount += int(item.Quantity)
			row.GrossAmount += item.Amount
		case domain.AssetWorkbenchItemTypeErrorDeduction:
			row.DeductionAmount += item.Amount
		case domain.AssetWorkbenchItemTypeWelfare:
			row.WelfareAmount += item.Amount
		case domain.AssetWorkbenchItemTypeSupplement:
			row.SupplementAmount += item.Amount
		case domain.AssetWorkbenchItemTypeAdjustment, domain.AssetWorkbenchItemTypeReversal:
			if item.Direction == "debit" {
				row.AdjustmentAmount -= item.Amount
			} else {
				row.AdjustmentAmount += item.Amount
			}
		}
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount + row.AdjustmentAmount
	}

	currentItems, err := s.repo.ListSubmissionItemsByMonth(ctx, currentMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load current month submission items.", err.Error())
	}
	selfItems := make([]*domain.AssetWorkbenchSubmissionItem, 0, len(currentItems))
	for _, item := range currentItems {
		if item == nil || item.PayeeUserID != actor.ID {
			continue
		}
		if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		selfItems = append(selfItems, item)
	}
	currentSupplements, _, err := s.repo.ListSettlementSupplements(ctx, repo.AssetWorkbenchSettlementSupplementFilter{
		PayeeUserID:   &actor.ID,
		BusinessMonth: currentMonth,
		PageSize:      500,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load current month supplements.", err.Error())
	}
	if appErr := s.enrichSettlementSupplementFiles(ctx, currentSupplements); appErr != nil {
		return nil, appErr
	}
	permission, err := s.repo.GetSupplementPermission(ctx, actor.ID, currentMonth)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load current supplement permission.", err.Error())
	}
	if errors.Is(err, sql.ErrNoRows) {
		permission = nil
	}
	errorRecords, err := s.repo.ListErrorRecordsByMonth(ctx, currentMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load current month error records.", err.Error())
	}
	preview, appErr := s.buildSettlementPreview(ctx, currentMonth, selfItems, errorRecords, currentSupplements)
	if appErr != nil {
		return nil, appErr
	}
	for _, previewRow := range preview.Rows {
		if previewRow.PayeeUserID != actor.ID {
			continue
		}
		row := ensureRow(currentMonth)
		row.ItemCount += previewRow.ItemCount
		row.PageCount += previewRow.PageCount
		row.GrossAmount += previewRow.GrossAmount
		row.DeductionAmount += previewRow.DeductionAmount
		row.WelfareAmount += previewRow.WelfareAmount
		row.SupplementAmount += previewRow.SupplementAmount
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount + row.AdjustmentAmount
	}

	months := make([]MySettlementMonthRow, 0, len(rowsByMonth))
	for _, row := range rowsByMonth {
		months = append(months, *row)
	}
	sort.Slice(months, func(i, j int) bool {
		return months[i].BusinessMonth > months[j].BusinessMonth
	})
	currentRow := rowsByMonth[currentMonth]
	estimated := 0.0
	if currentRow != nil {
		estimated = currentRow.NetAmount
	}
	return &MySettlementResponse{
		CurrentMonth:         currentMonth,
		EstimatedNetAmount:   estimated,
		Months:               months,
		SupplementPermission: permission,
		Supplements:          currentSupplements,
	}, nil
}

func (s *Service) PreviewSettlement(ctx context.Context, actor domain.RequestActor, businessMonth string) (*SettlementPreview, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can preview settlements.", nil)
	}
	businessMonth = strings.TrimSpace(businessMonth)
	if businessMonth == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month is required.", nil)
	}
	items, appErr := s.loadSettleableItems(ctx, businessMonth)
	if appErr != nil {
		return nil, appErr
	}
	supplements, appErr := s.loadSettleableSupplements(ctx, businessMonth)
	if appErr != nil {
		return nil, appErr
	}
	errorRecords, err := s.repo.ListErrorRecordsByMonth(ctx, businessMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load error records.", err.Error())
	}
	preview, appErr := s.buildSettlementPreview(ctx, businessMonth, items, errorRecords, supplements)
	if appErr != nil {
		return nil, appErr
	}
	return preview, nil
}

func (s *Service) SettlementReport(ctx context.Context, actor domain.RequestActor, businessMonth string) (*SettlementReport, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can view settlement reports.", nil)
	}
	businessMonth = strings.TrimSpace(businessMonth)
	if businessMonth == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month is required.", nil)
	}
	items, appErr := s.loadSettleableItems(ctx, businessMonth)
	if appErr != nil {
		return nil, appErr
	}
	supplements, appErr := s.loadSettleableSupplements(ctx, businessMonth)
	if appErr != nil {
		return nil, appErr
	}
	errorRecords, err := s.repo.ListErrorRecordsByMonth(ctx, businessMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load error records.", err.Error())
	}
	report, appErr := s.buildSettlementReport(ctx, businessMonth, items, errorRecords, supplements)
	if appErr != nil {
		return nil, appErr
	}
	return report, nil
}

func (s *Service) GenerateSettlementBatch(ctx context.Context, actor domain.RequestActor, businessMonth string) (*domain.AssetWorkbenchSettlementBatch, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can generate settlement batches.", nil)
	}
	businessMonth = strings.TrimSpace(businessMonth)
	if businessMonth == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month is required.", nil)
	}
	errorRecords, err := s.repo.ListErrorRecordsByMonth(ctx, businessMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load error records.", err.Error())
	}
	var batch *domain.AssetWorkbenchSettlementBatch
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		items, err := s.repo.LockSettleableItems(ctx, tx, businessMonth)
		if err != nil {
			return err
		}
		supplements, err := s.repo.LockSettleableSupplements(ctx, tx, businessMonth)
		if err != nil {
			return err
		}
		if len(items) == 0 && len(supplements) == 0 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "No settleable items or supplements for this month.", nil)
		}
		preview, appErr := s.buildSettlementPreview(ctx, businessMonth, items, errorRecords, supplements)
		if appErr != nil {
			return appErr
		}
		batch, err = s.repo.CreateSettlementBatch(ctx, tx, &domain.AssetWorkbenchSettlementBatch{
			BatchNo:          "AWB" + s.nowFn().UTC().Format("20060102150405") + strings.ToUpper(uuid.NewString()[:8]),
			BusinessMonth:    businessMonth,
			Status:           domain.AssetWorkbenchBatchStatusGenerated,
			GeneratedBy:      actor.ID,
			ItemCount:        preview.Totals.ItemCount,
			GrossAmount:      preview.Totals.GrossAmount,
			DeductionAmount:  preview.Totals.DeductionAmount,
			WelfareAmount:    preview.Totals.WelfareAmount,
			SupplementAmount: preview.Totals.SupplementAmount,
			NetAmount:        preview.Totals.NetAmount,
		})
		if err != nil {
			return err
		}
		welfareLines, appErr := s.buildWelfareLines(ctx, businessMonth, items)
		if appErr != nil {
			return appErr
		}
		profiles := map[int64]*domain.AssetWorkbenchProfile{}
		deductionCache := map[string]deductionRuleCacheEntry{}
		itemIDs := make([]int64, 0, len(items))
		for _, item := range items {
			itemIDs = append(itemIDs, item.ID)
			submissionItemID := item.ID
			unitPrice := 0.0
			if item.BaseUnitPrice != nil {
				unitPrice = *item.BaseUnitPrice
			}
			if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
				BatchID:          batch.ID,
				ItemType:         domain.AssetWorkbenchItemTypeGrossPiecework,
				SubmissionItemID: &submissionItemID,
				PayeeUserID:      item.PayeeUserID,
				BusinessMonth:    businessMonth,
				Amount:           item.GrossAmount,
				Quantity:         float64(item.PageCount),
				UnitPrice:        &unitPrice,
				Direction:        "credit",
				SourceRefType:    "submission_item",
				SourceRefID:      &submissionItemID,
				Snapshot:         item.PricingSnapshot,
			}); err != nil {
				return err
			}
			errorCount := matchedErrorCount(errorRecords, item)
			if errorCount > 0 {
				deduction, ruleSnapshot, appErr := s.calculateDeduction(ctx, item, errorCount)
				if appErr != nil {
					return appErr
				}
				if deduction > 0 {
					if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
						BatchID:          batch.ID,
						ItemType:         domain.AssetWorkbenchItemTypeErrorDeduction,
						SubmissionItemID: &submissionItemID,
						PayeeUserID:      item.PayeeUserID,
						BusinessMonth:    businessMonth,
						Amount:           deduction,
						Quantity:         float64(errorCount),
						Direction:        "debit",
						SourceRefType:    "error_record",
						SourceRefID:      &submissionItemID,
						Snapshot:         ruleSnapshot,
					}); err != nil {
						return err
					}
				}
			}
		}
		for _, record := range errorRecords {
			if record == nil || record.PayeeUserID == nil || record.MatchStatus != domain.AssetWorkbenchErrorMatchStatusMatched || strings.TrimSpace(record.DifficultyClass) == "" || record.ErrorCount <= 0 {
				continue
			}
			payeeID := *record.PayeeUserID
			profile, appErr := s.settlementReportProfile(ctx, payeeID, profiles)
			if appErr != nil {
				return appErr
			}
			deduction, ruleSnapshot, appErr := s.calculateQualityErrorDeductionCached(ctx, businessMonth, record, profile, deductionCache)
			if appErr != nil {
				return appErr
			}
			if deduction <= 0 {
				continue
			}
			sourceID := record.ID
			if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
				BatchID:       batch.ID,
				ItemType:      domain.AssetWorkbenchItemTypeErrorDeduction,
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				Amount:        deduction,
				Quantity:      float64(record.ErrorCount),
				Direction:     "debit",
				SourceRefType: "error_record",
				SourceRefID:   &sourceID,
				Snapshot:      ruleSnapshot,
			}); err != nil {
				return err
			}
		}
		for _, line := range welfareLines {
			sourceID := line.RuleID
			unitPrice := line.Amount
			if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
				BatchID:       batch.ID,
				ItemType:      domain.AssetWorkbenchItemTypeWelfare,
				PayeeUserID:   line.PayeeUserID,
				BusinessMonth: businessMonth,
				Amount:        line.Amount,
				Quantity:      1,
				UnitPrice:     &unitPrice,
				Direction:     "credit",
				SourceRefType: "welfare_rule",
				SourceRefID:   &sourceID,
				Snapshot:      line.Snapshot,
			}); err != nil {
				return err
			}
		}
		supplementIDs := make([]int64, 0, len(supplements))
		for _, supplement := range supplements {
			supplementIDs = append(supplementIDs, supplement.ID)
			sourceID := supplement.ID
			unitPrice := supplement.GrossAmount
			if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
				BatchID:       batch.ID,
				ItemType:      domain.AssetWorkbenchItemTypeSupplement,
				PayeeUserID:   supplement.PayeeUserID,
				BusinessMonth: businessMonth,
				Amount:        supplement.GrossAmount,
				Quantity:      float64(supplement.PageCount),
				UnitPrice:     &unitPrice,
				Direction:     "credit",
				SourceRefType: "settlement_supplement",
				SourceRefID:   &sourceID,
				Snapshot: mustJSON(map[string]interface{}{
					"supplement_id":    supplement.ID,
					"order_no":         supplement.OrderNo,
					"difficulty_class": supplement.DifficultyClass,
					"page_count":       supplement.PageCount,
					"finalized":        supplement.Finalized,
				}),
			}); err != nil {
				return err
			}
		}
		if err := s.repo.AttachItemsToSettlementBatch(ctx, tx, batch.ID, itemIDs); err != nil {
			return err
		}
		if err := s.repo.AttachSupplementsToSettlementBatch(ctx, tx, batch.ID, supplementIDs); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSettlementGenerated, domain.AssetWorkbenchEntitySettlement, &batch.ID, nil, batch, businessMonth)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to generate settlement batch.", err.Error())
	}
	return batch, nil
}

func (s *Service) ConfirmSettlementBatch(ctx context.Context, actor domain.RequestActor, batchID int64) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can confirm settlement batches.", nil)
	}
	items, err := s.repo.ListSettlementItemsByBatch(ctx, batchID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to load settlement items.", err.Error())
	}
	payoutSnapshots, appErr := s.buildPayoutSnapshots(ctx, items, batchID)
	if appErr != nil {
		return appErr
	}
	confirmedAt := s.nowFn().UTC()
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.repo.LockSettlementBatch(ctx, tx, batchID); err != nil {
			return err
		}
		if err := s.repo.FreezeSettlementPayouts(ctx, tx, batchID, confirmedAt, payoutSnapshots); err != nil {
			return err
		}
		if err := s.repo.ConfirmSettlementBatch(ctx, tx, batchID, actor.ID, confirmedAt); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSettlementConfirmed, domain.AssetWorkbenchEntitySettlement, &batchID, nil, map[string]interface{}{
			"batch_id":         batchID,
			"status":           domain.AssetWorkbenchBatchStatusConfirmed,
			"payout_snapshots": len(payoutSnapshots),
			"confirmed_at_utc": confirmedAt,
		}, "")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to confirm settlement batch.", err.Error())
	}
	if s.notifications != nil {
		for payeeID := range payoutSnapshots {
			payload := mustJSON(map[string]interface{}{
				"source":          "asset_workbench",
				"action":          "open_settlement",
				"batch_id":        batchID,
				"payee_user_id":   payeeID,
				"business_months": settlementItemMonthsForPayee(items, payeeID),
				"status":          domain.AssetWorkbenchBatchStatusConfirmed,
				"confirmed_at":    confirmedAt,
			})
			_, _, _ = s.notifications.CreateDedupedNotification(
				ctx,
				payeeID,
				domain.NotificationTypeAssetWorkbenchSettlementUpdated,
				payload,
				"asset_workbench_settlement_updated",
				fmt.Sprintf("asset_workbench_settlement_confirmed:%d:%d", batchID, payeeID),
			)
		}
	}
	return nil
}

func (s *Service) buildPayoutSnapshots(ctx context.Context, items []*domain.AssetWorkbenchSettlementItem, batchID int64) (map[int64]json.RawMessage, *domain.AppError) {
	payees := map[int64]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		payees[item.PayeeUserID] = struct{}{}
	}
	if len(payees) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Settlement batch has no payable items.", nil)
	}
	snapshots := make(map[int64]json.RawMessage, len(payees))
	for payeeID := range payees {
		profile, err := s.repo.GetProfileByUserID(ctx, payeeID)
		if err != nil {
			return nil, mapRepoReadError(err, "Payee profile not found.", "Failed to load payee payout profile.")
		}
		if profile == nil || strings.TrimSpace(profile.RealName) == "" || strings.TrimSpace(profile.AlipayAccount) == "" || profile.IDCard == nil || strings.TrimSpace(*profile.IDCard) == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Payee payout profile is incomplete; confirm is blocked.", map[string]interface{}{
				"batch_id":      batchID,
				"payee_user_id": payeeID,
			})
		}
		snapshots[payeeID] = mustJSON(map[string]interface{}{
			"source":           "confirm_time",
			"payee_user_id":    payeeID,
			"real_name":        profile.RealName,
			"alipay_account":   profile.AlipayAccount,
			"id_card_masked":   maskSensitiveValue(*profile.IDCard, 0, 4),
			"business_months":  settlementItemMonthsForPayee(items, payeeID),
			"confirmed_at_utc": s.nowFn().UTC(),
			"payout_authority": "paid_to_user_id+payout_snapshot_json",
		})
	}
	return snapshots, nil
}

func settlementItemMonthsForPayee(items []*domain.AssetWorkbenchSettlementItem, payeeID int64) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range items {
		if item == nil || item.PayeeUserID != payeeID {
			continue
		}
		month := strings.TrimSpace(item.BusinessMonth)
		if month == "" {
			continue
		}
		if _, ok := seen[month]; ok {
			continue
		}
		seen[month] = struct{}{}
		out = append(out, month)
	}
	sort.Strings(out)
	return out
}

func (s *Service) CancelSettlementBatch(ctx context.Context, actor domain.RequestActor, batchID int64, reason string) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can cancel settlement batches.", nil)
	}
	if strings.TrimSpace(reason) == "" {
		return domain.NewAppError(domain.ErrCodeReasonRequired, "A cancellation reason is required.", nil)
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		trimmedReason := strings.TrimSpace(reason)
		if err := s.repo.CancelGeneratedSettlementBatch(ctx, tx, batchID, actor.ID, trimmedReason, s.nowFn().UTC()); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSettlementCancelled, domain.AssetWorkbenchEntitySettlement, &batchID, nil, map[string]interface{}{
			"batch_id": batchID,
			"status":   domain.AssetWorkbenchBatchStatusCancelled,
		}, trimmedReason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to cancel settlement batch.", err.Error())
	}
	return nil
}

func (s *Service) CreateSettlementAdjustment(ctx context.Context, actor domain.RequestActor, params CreateSettlementAdjustmentParams) (*domain.AssetWorkbenchSettlementAdjustment, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can create settlement adjustments.", nil)
	}
	if params.BatchID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch_id is required.", nil)
	}
	if params.PayeeUserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id is required.", nil)
	}
	adjustmentType := normalizeSettlementAdjustmentType(params.AdjustmentType)
	direction := normalizeSettlementAdjustmentDirection(params.Direction, adjustmentType)
	amount := params.Amount
	if amount <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "amount must be greater than zero.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "An adjustment reason is required.", nil)
	}
	signedAmount := amount
	if direction == "debit" {
		signedAmount = -amount
	}
	var created *domain.AssetWorkbenchSettlementAdjustment
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		batch, err := s.repo.LockSettlementBatch(ctx, tx, params.BatchID)
		if err != nil {
			return err
		}
		if batch.Status != domain.AssetWorkbenchBatchStatusConfirmed {
			return domain.NewAppError(domain.ErrCodeConflict, "Only confirmed settlement batches can receive adjustments.", map[string]interface{}{
				"batch_id": params.BatchID,
				"status":   batch.Status,
			})
		}
		batchID := batch.ID
		created, err = s.repo.CreateSettlementAdjustment(ctx, tx, &domain.AssetWorkbenchSettlementAdjustment{
			BatchID:        &batchID,
			PayeeUserID:    params.PayeeUserID,
			BusinessMonth:  batch.BusinessMonth,
			AdjustmentType: adjustmentType,
			Amount:         signedAmount,
			Reason:         reason,
			Status:         domain.AssetWorkbenchAdjustmentStatusApplied,
			Payload: mustJSON(map[string]interface{}{
				"input":      params.Payload,
				"direction":  direction,
				"amount":     amount,
				"signed":     signedAmount,
				"batch_id":   batch.ID,
				"batch_no":   batch.BatchNo,
				"created_by": actor.ID,
			}),
			CreatedBy: actor.ID,
		})
		if err != nil {
			return err
		}
		sourceID := created.ID
		unitPrice := amount
		itemType := domain.AssetWorkbenchItemTypeAdjustment
		if adjustmentType == domain.AssetWorkbenchAdjustmentTypeReversal {
			itemType = domain.AssetWorkbenchItemTypeReversal
		}
		if _, err := s.repo.CreateSettlementItem(ctx, tx, &domain.AssetWorkbenchSettlementItem{
			BatchID:       batch.ID,
			ItemType:      itemType,
			PayeeUserID:   params.PayeeUserID,
			BusinessMonth: batch.BusinessMonth,
			Amount:        amount,
			Quantity:      1,
			UnitPrice:     &unitPrice,
			Direction:     direction,
			SourceRefType: "settlement_adjustment",
			SourceRefID:   &sourceID,
			Snapshot: mustJSON(map[string]interface{}{
				"adjustment_id":   created.ID,
				"adjustment_type": adjustmentType,
				"direction":       direction,
				"amount":          amount,
				"signed_amount":   signedAmount,
				"reason":          reason,
			}),
		}); err != nil {
			return err
		}
		if err := s.repo.ApplySettlementBatchAdjustment(ctx, tx, batch.ID, signedAmount); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSettlementAdjusted, domain.AssetWorkbenchEntityAdjustment, &created.ID, nil, created, reason)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create settlement adjustment.", err.Error())
	}
	return created, nil
}

func (s *Service) ListSettlementBatches(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchSettlementBatchFilter) ([]*domain.AssetWorkbenchSettlementBatch, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can list settlement batches.", nil)
	}
	items, total, err := s.repo.ListSettlementBatches(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list settlement batches.", err.Error())
	}
	return items, total, nil
}

func (s *Service) GetSettlementBatchDetail(ctx context.Context, actor domain.RequestActor, batchID int64) (*SettlementBatchDetail, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can view settlement batch details.", nil)
	}
	if batchID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch_id is required.", nil)
	}
	batch, err := s.repo.GetSettlementBatch(ctx, batchID)
	if err != nil {
		return nil, mapRepoReadError(err, "Settlement batch not found.", "Failed to load settlement batch.")
	}
	items, err := s.repo.ListSettlementItemsByBatch(ctx, batchID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list settlement batch items.", err.Error())
	}
	payrollRows, appErr := s.buildSettlementPayrollRowsFromItems(ctx, batch.BusinessMonth, items)
	if appErr != nil {
		return nil, appErr
	}
	return &SettlementBatchDetail{Batch: batch, Items: items, PayrollRows: payrollRows}, nil
}

func (s *Service) ListSupplementPermissions(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchSupplementPermissionFilter) ([]*domain.AssetWorkbenchSupplementPermission, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can list supplement permissions.", nil)
	}
	items, total, err := s.repo.ListSupplementPermissions(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list supplement permissions.", err.Error())
	}
	return items, total, nil
}

func (s *Service) ListSupplementEligibleMonths(ctx context.Context, actor domain.RequestActor, payeeUserID int64) ([]string, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can list supplement eligible months.", nil)
	}
	if payeeUserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id is required.", nil)
	}
	return []string{s.businessMonth(s.nowFn().UTC())}, nil
}

func (s *Service) UpsertSupplementPermission(ctx context.Context, actor domain.RequestActor, params UpsertSupplementPermissionParams) (*domain.AssetWorkbenchSupplementPermission, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can update supplement permissions.", nil)
	}
	item, appErr := normalizeSupplementPermission(actor.ID, params, s.nowFn().UTC())
	if appErr != nil {
		return nil, appErr
	}
	if item.Enabled && item.BusinessMonth != s.businessMonth(s.nowFn().UTC()) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Supplement permission can only be opened for the current natural month.", map[string]interface{}{
			"payee_user_id":          item.PayeeUserID,
			"business_month":         item.BusinessMonth,
			"current_business_month": s.businessMonth(s.nowFn().UTC()),
		})
	}
	var saved *domain.AssetWorkbenchSupplementPermission
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		saved, err = s.repo.UpsertSupplementPermission(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSupplementPermissionChanged, domain.AssetWorkbenchEntitySupplementPermission, &saved.ID, nil, saved, item.Reason)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to update supplement permission.", err.Error())
	}
	if s.notifications != nil && saved.PayeeUserID > 0 {
		payload := mustJSON(map[string]interface{}{
			"source":         "asset_workbench",
			"action":         "open_settlement",
			"permission_id":  saved.ID,
			"payee_user_id":  saved.PayeeUserID,
			"business_month": saved.BusinessMonth,
			"enabled":        saved.Enabled,
			"reason":         saved.Reason,
			"updated_by":     actor.ID,
		})
		_, _, _ = s.notifications.CreateDedupedNotification(
			ctx,
			saved.PayeeUserID,
			domain.NotificationTypeAssetWorkbenchSupplementAccess,
			payload,
			"asset_workbench_supplement_access",
			fmt.Sprintf("asset_workbench_supplement_access:%d:%t:%d", saved.ID, saved.Enabled, s.nowFn().UTC().UnixNano()),
		)
	}
	return saved, nil
}

func (s *Service) ListSettlementSupplements(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, 0, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can list settlement supplements.", nil)
	}
	normalizedFilter, appErr := normalizeSettlementSupplementFilter(filter)
	if appErr != nil {
		return nil, 0, appErr
	}
	items, total, err := s.repo.ListSettlementSupplements(ctx, normalizedFilter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list settlement supplements.", err.Error())
	}
	if appErr := s.enrichSettlementSupplementFiles(ctx, items); appErr != nil {
		return nil, 0, appErr
	}
	return items, total, nil
}

func (s *Service) enrichSettlementSupplementFiles(ctx context.Context, items []*domain.AssetWorkbenchSettlementSupplement) *domain.AppError {
	for _, item := range items {
		if item == nil || item.SubmissionItemID == nil || *item.SubmissionItemID <= 0 {
			continue
		}
		files, err := s.repo.ListSubmissionFiles(ctx, *item.SubmissionItemID)
		if err != nil {
			return domain.NewAppError(domain.ErrCodeInternalError, "Failed to load settlement supplement files.", err.Error())
		}
		item.Files = files
	}
	return nil
}

func (s *Service) CreateSettlementSupplement(ctx context.Context, actor domain.RequestActor, params CreateSettlementSupplementParams) (*domain.AssetWorkbenchSettlementSupplement, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	canManageSupplements := actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin)
	if !canManageSupplements && !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles or the supplement payee can create settlement supplements.", nil)
	}
	if !canManageSupplements {
		if params.PayeeUserID == 0 {
			params.PayeeUserID = actor.ID
		}
		if params.PayeeUserID != actor.ID {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement submitters can only upload their own supplement work.", map[string]interface{}{
				"actor_user_id": actor.ID,
				"payee_user_id": params.PayeeUserID,
			})
		}
		if len(params.UploadSessionIDs) == 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Self-service supplements require uploaded files.", nil)
		}
		params.Status = domain.AssetWorkbenchSupplementStatusApproved
	}
	currentBusinessMonth := s.businessMonth(s.nowFn().UTC())
	if strings.TrimSpace(params.BusinessMonth) == "" {
		params.BusinessMonth = currentBusinessMonth
	}

	var uploadSessions []*domain.AssetWorkbenchUploadSession
	var profile *domain.AssetWorkbenchProfile
	if len(params.UploadSessionIDs) > 0 {
		var appErr *domain.AppError
		uploadSessions, params.DifficultyClass, appErr = s.loadSubmissionUploadSessions(ctx, actor, CreateSubmissionItemParams{
			DifficultyClass:  params.DifficultyClass,
			UploadSessionIDs: params.UploadSessionIDs,
		})
		if appErr != nil {
			return nil, appErr
		}
		if strings.TrimSpace(params.OrderNo) == "" && len(uploadSessions) > 0 {
			params.OrderNo = path.Base(firstNonEmpty(uploadSessions[0].RelativePath, uploadSessions[0].OriginalFilename))
		}
		var err error
		profile, err = s.repo.GetProfileByUserID(ctx, params.PayeeUserID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load supplement payee profile.", err.Error())
			}
			profile = &domain.AssetWorkbenchProfile{UserID: params.PayeeUserID, Status: domain.AssetWorkbenchProfileStatusPending}
		}
	}
	item, appErr := normalizeSettlementSupplement(actor.ID, params)
	if appErr != nil {
		return nil, appErr
	}
	if item.BusinessMonth != currentBusinessMonth {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Settlement supplements must be recorded in the current natural month.", map[string]interface{}{
			"business_month":         item.BusinessMonth,
			"current_business_month": currentBusinessMonth,
		})
	}
	if appErr := s.ensureDifficultyClass(ctx, item.DifficultyClass, false); appErr != nil {
		return nil, appErr
	}
	if appErr := s.ensureSupplementPermissionOpen(ctx, item.PayeeUserID, item.BusinessMonth); appErr != nil {
		return nil, appErr
	}
	duplicateHint, appErr := s.buildSettlementSupplementDuplicateHint(ctx, item)
	if appErr != nil {
		return nil, appErr
	}
	item.DuplicateHint = duplicateHint
	var created *domain.AssetWorkbenchSettlementSupplement
	var createdFiles []*domain.AssetWorkbenchSubmissionFile
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if appErr := s.ensureSupplementPermissionOpenForUpdate(ctx, tx, item.PayeeUserID, item.BusinessMonth); appErr != nil {
			return appErr
		}
		if len(uploadSessions) > 0 {
			now := s.nowFn().UTC()
			submission, err := s.repo.CreateSubmission(ctx, tx, &domain.AssetWorkbenchSubmission{
				SubmissionNo:    "AWS" + now.Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", "")),
				SubmitterUserID: actor.ID,
				BusinessMonth:   item.BusinessMonth,
				SubmittedAt:     now,
				Status:          domain.AssetWorkbenchSubmissionStatusSubmitted,
				Notes:           "self-service settlement supplement upload",
			})
			if err != nil {
				return err
			}
			submissionItem, buildErr := s.buildSubmissionItem(ctx, item.PayeeUserID, submission.ID, now, item.BusinessMonth, profile, CreateSubmissionItemParams{
				OrderNo:          item.OrderNo,
				DifficultyClass:  item.DifficultyClass,
				Finalized:        item.Finalized,
				PageCount:        item.PageCount,
				ItemCount:        1,
				UploadSessionIDs: params.UploadSessionIDs,
			})
			if buildErr != nil {
				return buildErr
			}
			if submissionItem.PricingStatus != domain.AssetWorkbenchPricingStatusPriced {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "Supplement work cannot be submitted until its payee and upload category have an active price.", map[string]interface{}{
					"payee_user_id":    item.PayeeUserID,
					"difficulty_class": item.DifficultyClass,
					"pricing_status":   submissionItem.PricingStatus,
				})
			}
			submissionItem.EntryKind = domain.AssetWorkbenchSubmissionEntryKindSupplement
			createdItem, err := s.repo.CreateSubmissionItem(ctx, tx, submissionItem)
			if err != nil {
				return err
			}
			for index, session := range uploadSessions {
				file, err := s.repo.CreateSubmissionFile(ctx, tx, submissionFileFromUploadSession(submission.ID, createdItem.ID, actor.ID, session, index, now))
				if err != nil {
					return err
				}
				createdFiles = append(createdFiles, file)
				if err := s.repo.UpdateUploadSessionStatus(ctx, tx, session.SessionID, domain.AssetWorkbenchUploadStatusSubmitted, nil, nil, &createdItem.ID); err != nil {
					return err
				}
			}
			if err := s.repo.RefreshSubmissionTotals(ctx, tx, submission.ID); err != nil {
				return err
			}
			item.SubmissionItemID = &createdItem.ID
			item.DifficultyClass = createdItem.DifficultyClass
			item.PageCount = createdItem.PageCount
			item.GrossAmount = createdItem.GrossAmount
			item.Status = domain.AssetWorkbenchSupplementStatusApproved
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSubmissionCreated, domain.AssetWorkbenchEntitySubmission, &submission.ID, nil, map[string]interface{}{
				"submission": submission,
				"item_count": 1,
				"entry_kind": domain.AssetWorkbenchSubmissionEntryKindSupplement,
			}, "self-service supplement upload"); err != nil {
				return err
			}
		}
		var err error
		created, err = s.repo.CreateSettlementSupplement(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSupplementCreated, domain.AssetWorkbenchEntitySupplement, &created.ID, nil, created, params.OrderNo)
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create settlement supplement.", err.Error())
	}
	created.Files = createdFiles
	return created, nil
}

func (s *Service) ImportSettlementSupplementsExcel(ctx context.Context, actor domain.RequestActor, businessMonth string, reader io.Reader) (*SettlementSupplementImportResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles can import settlement supplements.", nil)
	}
	rows, appErr := parseSettlementSupplementExcel(businessMonth, reader)
	if appErr != nil {
		return nil, appErr
	}
	result := &SettlementSupplementImportResult{
		Created:  []*domain.AssetWorkbenchSettlementSupplement{},
		Failures: []SettlementSupplementImportFailure{},
	}
	for _, row := range rows {
		created, appErr := s.CreateSettlementSupplement(ctx, actor, row.params)
		if appErr != nil {
			result.Failures = append(result.Failures, SettlementSupplementImportFailure{Row: row.row, Reason: appErr.Message})
			continue
		}
		result.Created = append(result.Created, created)
	}
	return result, nil
}

func (s *Service) VoidSettlementSupplement(ctx context.Context, actor domain.RequestActor, supplementID int64, reason string) (*domain.AssetWorkbenchSettlementSupplement, *domain.AppError) {
	result, appErr := s.BatchDeleteSettlementSupplements(ctx, actor, BatchDeleteSettlementSupplementsParams{
		SupplementIDs: []int64{supplementID},
		Reason:        reason,
	})
	if appErr != nil {
		return nil, appErr
	}
	if result == nil || len(result.Supplements) != 1 {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to delete settlement supplement.", nil)
	}
	return result.Supplements[0], nil
}

func (s *Service) BatchDeleteSettlementSupplements(ctx context.Context, actor domain.RequestActor, params BatchDeleteSettlementSupplementsParams) (*BatchDeleteSettlementSupplementsResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	canManage := actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin)
	if !canManage && !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only settlement roles or the supplement payee can delete settlement supplements.", nil)
	}
	reason := strings.TrimSpace(params.Reason)
	if reason == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when deleting settlement supplements.", nil)
	}
	supplementIDs := positiveUniqueInt64s(params.SupplementIDs)
	if len(supplementIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplement_ids is required.", nil)
	}
	if len(supplementIDs) > 100 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplement_ids cannot contain more than 100 values.", nil)
	}
	result := &BatchDeleteSettlementSupplementsResult{
		DeletedIDs:  make([]int64, 0, len(supplementIDs)),
		Supplements: make([]*domain.AssetWorkbenchSettlementSupplement, 0, len(supplementIDs)),
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		now := s.nowFn().UTC()
		for _, supplementID := range supplementIDs {
			before, err := s.repo.GetSettlementSupplementForUpdate(ctx, tx, supplementID)
			if err != nil {
				return err
			}
			if !canManage && before.PayeeUserID != actor.ID {
				return domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement payees can only delete their own supplement records.", map[string]interface{}{
					"supplement_id": supplementID,
					"payee_user_id": before.PayeeUserID,
				})
			}
			switch before.Status {
			case domain.AssetWorkbenchSupplementStatusInBatch, domain.AssetWorkbenchSupplementStatusSettled, domain.AssetWorkbenchSupplementStatusVoided:
				return domain.NewAppError(domain.ErrCodeConflict, "Only draft or approved supplements can be deleted.", map[string]interface{}{
					"supplement_id": supplementID,
					"status":        before.Status,
				})
			}
			if before.SubmissionItemID != nil && *before.SubmissionItemID > 0 {
				files, err := s.repo.ListSubmissionFilesForUpdate(ctx, tx, *before.SubmissionItemID)
				if err != nil {
					return err
				}
				for _, file := range files {
					if err := s.repo.DeleteSubmissionFile(ctx, tx, file.ID, actor.ID, reason, now); err != nil {
						return err
					}
					if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventFileDeleted, domain.AssetWorkbenchEntitySubmissionFile, &file.ID, file, nil, reason); err != nil {
						return err
					}
				}
				voidedItem, err := s.repo.VoidSubmissionItem(ctx, tx, *before.SubmissionItemID, actor.ID, "linked supplement voided: "+reason, now)
				if err != nil {
					return err
				}
				if err := s.repo.RefreshSubmissionTotals(ctx, tx, voidedItem.SubmissionID); err != nil {
					return err
				}
				if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemVoided, domain.AssetWorkbenchEntitySubmissionItem, &voidedItem.ID, nil, voidedItem, reason); err != nil {
					return err
				}
			}
			updated, err := s.repo.VoidSettlementSupplement(ctx, tx, supplementID)
			if err != nil {
				return err
			}
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSupplementVoided, domain.AssetWorkbenchEntitySupplement, &updated.ID, before, updated, reason); err != nil {
				return err
			}
			result.DeletedIDs = append(result.DeletedIDs, supplementID)
			result.Supplements = append(result.Supplements, updated)
		}
		return nil
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "One or more settlement supplements were not found.", nil)
		}
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to delete settlement supplement.", err.Error())
	}
	return result, nil
}

func (s *Service) ListEvents(ctx context.Context, actor domain.RequestActor, filter repo.AssetWorkbenchEventFilter) ([]*domain.AssetWorkbenchEvent, int64, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, 0, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) {
		filter.ActorID = &actor.ID
	}
	items, total, err := s.repo.ListEvents(ctx, filter)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench events.", err.Error())
	}
	return items, total, nil
}

func (s *Service) ListSavedViews(ctx context.Context, actor domain.RequestActor, viewType string) ([]*domain.AssetWorkbenchSavedView, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	viewType = strings.TrimSpace(viewType)
	if viewType == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "view_type is required.", nil)
	}
	items, err := s.repo.ListSavedViews(ctx, repo.AssetWorkbenchSavedViewFilter{UserID: actor.ID, ViewType: viewType})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench saved views.", err.Error())
	}
	return items, nil
}

func (s *Service) UpsertSavedView(ctx context.Context, actor domain.RequestActor, params UpsertSavedViewParams) (*domain.AssetWorkbenchSavedView, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	viewType := strings.TrimSpace(params.ViewType)
	viewName := strings.TrimSpace(params.ViewName)
	if viewType == "" || viewName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "view_type and view_name are required.", nil)
	}
	config := normalizeJSON(params.Config)
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	view := &domain.AssetWorkbenchSavedView{
		UserID:    actor.ID,
		ViewType:  viewType,
		ViewName:  viewName,
		Config:    config,
		IsDefault: params.IsDefault,
	}
	var saved *domain.AssetWorkbenchSavedView
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		saved, err = s.repo.UpsertSavedView(ctx, tx, view)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSavedViewUpserted, domain.AssetWorkbenchEntitySavedView, &saved.ID, nil, saved, viewName)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to save asset workbench view.", err.Error())
	}
	return saved, nil
}

func (s *Service) DeleteSavedView(ctx context.Context, actor domain.RequestActor, viewID int64) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if viewID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "view_id is required.", nil)
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.repo.DeleteSavedView(ctx, tx, actor.ID, viewID)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewAppError(domain.ErrCodeNotFound, "Saved view not found.", nil)
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to delete asset workbench saved view.", err.Error())
	}
	return nil
}

func (s *Service) OverviewSearch(ctx context.Context, actor domain.RequestActor, params OverviewSearchParams) (*OverviewSearchResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can search the asset workbench overview.", nil)
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 100 {
		pageSize = 100
	}
	scope := normalizeOverviewSearchScope(params.Scope)
	includeSubmissions, includeItems, includeFiles, includeOperational := overviewSearchIncludes(scope)
	filter := repo.AssetWorkbenchOverviewSearchFilter{
		Keyword:     strings.TrimSpace(params.Query),
		Creator:     strings.TrimSpace(params.Creator),
		OwnerUserID: s.driveOwnerFilter(actor),
		Submissions: includeSubmissions,
		Items:       includeItems,
		Files:       includeFiles,
		CreatedFrom: params.CreatedFrom,
		CreatedTo:   params.CreatedTo,
		Page:        page,
		PageSize:    pageSize,
	}
	var (
		items  = make([]*domain.AssetWorkbenchOverviewRow, 0, pageSize)
		total  int64
		appErr *domain.AppError
		mu     sync.Mutex
	)
	jobs := []func() error{}
	if includeSubmissions || includeItems || includeFiles {
		jobs = append(jobs, func() error {
			rows, rowTotal, err := s.repo.SearchOverviewRows(ctx, filter)
			if err != nil {
				return err
			}
			for _, row := range rows {
				decorateOverviewRow(row)
			}
			mu.Lock()
			items = append(items, rows...)
			total += rowTotal
			mu.Unlock()
			return nil
		})
	}
	if includeOperational && actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) && s.systemAssets != nil {
		jobs = append(jobs, func() error {
			systemResult, err := s.systemAssets.Search(ctx, domain.AssetSearchQuery{
				Keyword:        filter.Keyword,
				CreatedFrom:    params.CreatedFrom,
				CreatedTo:      params.CreatedTo,
				Page:           page,
				Size:           pageSize,
				Source:         domain.AssetResourceSourceAll,
				UsableState:    domain.AssetUsableStateFilterAll,
				FormatCategory: domain.AssetFormatCategoryAll,
				IsArchived:     domain.AssetArchiveFilterFalse,
				TaskStatus:     domain.AssetTaskStatusFilterAll,
			})
			if err != nil {
				mu.Lock()
				appErr = err
				mu.Unlock()
				return nil
			}
			if systemResult != nil {
				mu.Lock()
				total += systemResult.Total
				for _, asset := range systemResult.Items {
					row := overviewRowFromSystemAsset(asset)
					if row == nil || !overviewSystemAssetMatchesCreator(row, filter.Creator) {
						continue
					}
					items = append(items, row)
				}
				mu.Unlock()
			}
			return nil
		})
	} else if includeOperational && actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		jobs = append(jobs, func() error {
			rows, rowTotal, err := s.searchClientMaterialsForOverview(ctx, filter.Keyword, filter.Creator, pageSize)
			if err != nil {
				return err
			}
			mu.Lock()
			items = append(items, rows...)
			total += rowTotal
			mu.Unlock()
			return nil
		})
	}
	if err := runAssetWorkbenchSearchJobs(jobs...); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to search asset workbench overview.", err.Error())
	}
	if appErr != nil {
		return nil, appErr
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			if items[i].Source == items[j].Source {
				return items[i].ID > items[j].ID
			}
			return items[i].Source < items[j].Source
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > pageSize {
		items = items[:pageSize]
	}
	return &OverviewSearchResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (s *Service) SystemSearch(ctx context.Context, actor domain.RequestActor, query string, page int, pageSize int, source string, formatCategory string, businessLane string) (*SystemSearchResult, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can search system assets from workbench.", nil)
	}
	if s.systemAssets == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset searcher is not configured.", nil)
	}
	query = strings.TrimSpace(query)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	sourceFilter := domain.NormalizeAssetResourceSource(source)
	laneFilter := normalizeAssetWorkbenchBusinessLane(businessLane)
	result, appErr := s.systemAssets.Search(ctx, domain.AssetSearchQuery{
		Keyword:        query,
		Page:           page,
		Size:           pageSize,
		Source:         sourceFilter,
		UsableState:    domain.AssetUsableStateFilterAll,
		FormatCategory: domain.AssetFormatCategoryFilter(strings.TrimSpace(formatCategory)),
		BusinessLane:   laneFilter,
		IsArchived:     domain.AssetArchiveFilterFalse,
		TaskStatus:     domain.AssetTaskStatusFilterAll,
	})
	if appErr != nil {
		return nil, appErr
	}
	items := filterVisibleWorkbenchMaterialAssets(result.Items)
	total := result.Total
	if len(items) != len(result.Items) {
		total = int64(len(items))
	}
	return &SystemSearchResult{Items: items, Total: total, Page: result.Page, Size: result.Size}, nil
}

func (s *Service) BrowseMaterials(ctx context.Context, actor domain.RequestActor, path string, page int, pageSize int, source string, formatCategory string, businessLane string) (*assetcenter.MaterialBrowseResult, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can browse material assets from workbench.", nil)
	}
	browser, _ := s.systemAssets.(SystemMaterialBrowser)
	if browser == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Material browser is not configured.", nil)
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	laneFilter := normalizeAssetWorkbenchBusinessLane(businessLane)
	publicPath := normalizeWorkbenchMaterialPath(path)
	if !assetWorkbenchOperationalMaterialPathVisible(publicPath) {
		return &assetcenter.MaterialBrowseResult{
			Path:    publicPath,
			Folders: []assetcenter.MaterialFolder{},
			Files:   []*assetcenter.AssetDetail{},
			Total:   0,
			Page:    page,
			Size:    pageSize,
		}, nil
	}
	if workbenchIsVirtualQuarkRoot(publicPath) {
		return s.browseWorkbenchVirtualQuarkRoot(ctx, browser, page, pageSize, source, formatCategory, laneFilter)
	}
	actualPath := workbenchMaterialActualPath(publicPath)
	result, appErr := browser.BrowseMaterials(ctx, assetcenter.MaterialBrowseQuery{
		Path:           actualPath,
		Source:         domain.NormalizeAssetResourceSource(source),
		FormatCategory: domain.AssetFormatCategoryFilter(strings.TrimSpace(formatCategory)),
		BusinessLane:   laneFilter,
		Page:           page,
		Size:           pageSize,
	})
	if appErr != nil {
		return nil, appErr
	}
	originalFolderCount := len(result.Folders)
	originalFileCount := len(result.Files)
	result.Folders = filterVisibleWorkbenchMaterialFolders(result.Folders)
	result.Folders = rewriteWorkbenchVirtualQuarkFolders(result.Folders)
	if publicPath == "" {
		var hydrateErr *domain.AppError
		result.Folders, hydrateErr = s.hydrateWorkbenchVirtualQuarkRootFolder(ctx, browser, result.Folders, source, formatCategory, laneFilter)
		if hydrateErr != nil {
			return nil, hydrateErr
		}
	}
	result.Files = filterVisibleWorkbenchMaterialAssets(result.Files)
	result.Path = publicPath
	if len(result.Folders) != originalFolderCount || len(result.Files) != originalFileCount {
		result.Total = int64(len(result.Files))
	}
	return result, nil
}

func (s *Service) browseWorkbenchVirtualQuarkRoot(ctx context.Context, browser SystemMaterialBrowser, page int, pageSize int, source string, formatCategory string, businessLane domain.TaskBusinessLane) (*assetcenter.MaterialBrowseResult, *domain.AppError) {
	if businessLane.Valid() {
		return &assetcenter.MaterialBrowseResult{
			Path:    workbenchQuarkMaterialVirtualRoot,
			Folders: []assetcenter.MaterialFolder{},
			Files:   []*assetcenter.AssetDetail{},
			Total:   0,
			Page:    page,
			Size:    pageSize,
		}, nil
	}
	result, appErr := browser.BrowseMaterials(ctx, assetcenter.MaterialBrowseQuery{
		Path:           workbenchQuarkMaterialActualBase,
		Source:         domain.NormalizeAssetResourceSource(source),
		FormatCategory: domain.AssetFormatCategoryFilter(strings.TrimSpace(formatCategory)),
		BusinessLane:   businessLane,
		Page:           page,
		Size:           pageSize,
	})
	if appErr != nil {
		return nil, appErr
	}
	byName := map[string]assetcenter.MaterialFolder{}
	for _, folder := range result.Folders {
		byName[strings.ToLower(strings.TrimSpace(folder.Name))] = folder
	}
	folders := make([]assetcenter.MaterialFolder, 0, len(workbenchQuarkMaterialVirtualFolders))
	var total int64
	for _, virtual := range workbenchQuarkMaterialVirtualFolders {
		folder := byName[strings.ToLower(virtual)]
		total += folder.FileCount
		folders = append(folders, assetcenter.MaterialFolder{
			Path:            workbenchQuarkMaterialVirtualRoot + "/" + virtual,
			Name:            virtual,
			SourceType:      string(domain.AssetResourceSourceExternal),
			FileCount:       folder.FileCount,
			DirectFileCount: folder.DirectFileCount,
		})
	}
	return &assetcenter.MaterialBrowseResult{
		Path:    workbenchQuarkMaterialVirtualRoot,
		Folders: folders,
		Files:   []*assetcenter.AssetDetail{},
		Total:   total,
		Page:    page,
		Size:    pageSize,
	}, nil
}

func (s *Service) hydrateWorkbenchVirtualQuarkRootFolder(ctx context.Context, browser SystemMaterialBrowser, folders []assetcenter.MaterialFolder, source string, formatCategory string, businessLane domain.TaskBusinessLane) ([]assetcenter.MaterialFolder, *domain.AppError) {
	if businessLane.Valid() {
		return folders, nil
	}
	quarkIndex := -1
	for index := range folders {
		if workbenchIsVirtualQuarkRoot(folders[index].Path) {
			quarkIndex = index
			break
		}
	}
	if quarkIndex < 0 {
		return folders, nil
	}
	virtualRoot, appErr := s.browseWorkbenchVirtualQuarkRoot(ctx, browser, 1, 100, source, formatCategory, businessLane)
	if appErr != nil {
		return nil, appErr
	}
	folders[quarkIndex].FileCount = virtualRoot.Total
	folders[quarkIndex].DirectFileCount = 0
	return folders, nil
}

func (s *Service) SearchClientMaterials(ctx context.Context, actor domain.RequestActor, params ClientMaterialSearchParams) (*ClientMaterialSearchResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can search client materials.", nil)
	}
	admin := params.Admin && actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin)
	items, appErr := s.ListClientMaterials(ctx, actor, admin)
	if appErr != nil {
		return nil, appErr
	}
	query := strings.ToLower(strings.TrimSpace(params.Query))
	sku := strings.ToLower(strings.TrimSpace(params.SKU))
	filtered := make([]*domain.AssetWorkbenchClientMaterial, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if query != "" && !clientMaterialMatchesQuery(item, query) {
			continue
		}
		if sku != "" && !clientMaterialMatchesSKU(item, sku) {
			continue
		}
		filtered = append(filtered, item)
	}
	page, pageSize := normalizeServicePage(params.Page, params.PageSize, 50, 100)
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return &ClientMaterialSearchResult{
		Items: filtered[start:end],
		Total: int64(len(filtered)),
		Page:  page,
		Size:  pageSize,
	}, nil
}

func (s *Service) MaterialGroups(ctx context.Context, actor domain.RequestActor, params MaterialGroupSearchParams) (*MaterialGroupSearchResult, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can browse material groups from workbench.", nil)
	}
	if s.systemAssets == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset searcher is not configured.", nil)
	}
	page, pageSize := normalizeServicePage(params.Page, params.PageSize, 50, 100)
	sourceFilter := domain.NormalizeAssetResourceSource(params.Source)
	laneFilter := normalizeAssetWorkbenchBusinessLane(params.BusinessLane)
	result, appErr := s.systemAssets.Search(ctx, domain.AssetSearchQuery{
		Keyword:        strings.TrimSpace(params.Query),
		Page:           1,
		Size:           500,
		Source:         sourceFilter,
		UsableState:    domain.AssetUsableStateFilterAll,
		FormatCategory: domain.AssetFormatCategoryFilter(strings.TrimSpace(params.FormatCategory)),
		BusinessLane:   laneFilter,
		IsArchived:     domain.AssetArchiveFilterFalse,
		TaskStatus:     domain.AssetTaskStatusFilterAll,
	})
	if appErr != nil {
		return nil, appErr
	}
	grouped := groupMaterialAssets(filterVisibleWorkbenchMaterialAssets(result.Items))
	start := (page - 1) * pageSize
	if start > len(grouped) {
		start = len(grouped)
	}
	end := start + pageSize
	if end > len(grouped) {
		end = len(grouped)
	}
	return &MaterialGroupSearchResult{Items: grouped[start:end], Total: int64(len(grouped)), Page: page, Size: pageSize}, nil
}

func (s *Service) MaterialGroupFiles(ctx context.Context, actor domain.RequestActor, groupKey string, page int, pageSize int) (*MaterialGroupFilesResult, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can browse material group files from workbench.", nil)
	}
	if s.systemAssets == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset searcher is not configured.", nil)
	}
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "group_key is required.", nil)
	}
	page, pageSize = normalizeServicePage(page, pageSize, 50, 100)
	result, appErr := s.systemAssets.Search(ctx, domain.AssetSearchQuery{
		Keyword:        materialGroupSearchKeyword(groupKey),
		Page:           1,
		Size:           500,
		Source:         domain.AssetResourceSourceAll,
		UsableState:    domain.AssetUsableStateFilterAll,
		FormatCategory: domain.AssetFormatCategoryAll,
		IsArchived:     domain.AssetArchiveFilterFalse,
		TaskStatus:     domain.AssetTaskStatusFilterAll,
	})
	if appErr != nil {
		return nil, appErr
	}
	matches := make([]*assetcenter.AssetDetail, 0)
	for _, asset := range result.Items {
		if !assetWorkbenchMaterialAssetVisible(asset) {
			continue
		}
		if assetMaterialGroupKey(asset) == groupKey {
			matches = append(matches, asset)
		}
	}
	start := (page - 1) * pageSize
	if start > len(matches) {
		start = len(matches)
	}
	end := start + pageSize
	if end > len(matches) {
		end = len(matches)
	}
	return &MaterialGroupFilesResult{GroupKey: groupKey, Items: matches[start:end], Total: int64(len(matches)), Page: page, Size: pageSize}, nil
}

func (s *Service) SystemAssetDownload(ctx context.Context, actor domain.RequestActor, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can download system assets from workbench.", nil)
	}
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required.", nil)
	}
	if s.systemDownloads == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset downloader is not configured.", nil)
	}
	info, appErr := s.systemDownloads.DownloadLatest(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if info == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset download info is empty.", nil)
	}
	_ = s.recordSystemAssetDownloadEvent(ctx, actor, domain.AssetWorkbenchEventSystemAssetDownloaded, &assetID, map[string]interface{}{
		"asset_id": assetID,
		"filename": info.Filename,
		"mode":     info.DownloadMode,
	})
	return info, nil
}

func (s *Service) SystemAssetPreview(ctx context.Context, actor domain.RequestActor, assetID int64) (*SystemAssetPreviewMeta, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can preview system assets from workbench.", nil)
	}
	if assetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required.", nil)
	}
	return s.systemAssetPreviewMeta(ctx, assetID)
}

func (s *Service) systemAssetPreviewMeta(ctx context.Context, assetID int64) (*SystemAssetPreviewMeta, *domain.AppError) {
	if s.systemPreviews != nil {
		info, appErr := s.systemPreviews.GetAssetPreviewInfoByID(ctx, assetID)
		if appErr == nil {
			return systemAssetPreviewMetaFromDownloadInfo(assetID, string(domain.AssetResourceSourceSystem), strconv.FormatInt(assetID, 10), info), nil
		}
		if appErr.Code != domain.ErrCodeInvalidStateTransition {
			return nil, appErr
		}
	}
	if s.systemDownloads == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset downloader is not configured.", nil)
	}
	info, appErr := s.systemDownloads.DownloadLatest(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if info == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset preview info is empty.", nil)
	}
	return systemAssetPreviewMetaFromDownloadInfo(assetID, string(domain.AssetResourceSourceSystem), strconv.FormatInt(assetID, 10), info), nil
}

func systemAssetPreviewMetaFromDownloadInfo(assetID int64, sourceType, sourceRef string, info *domain.AssetDownloadInfo) *SystemAssetPreviewMeta {
	meta := &SystemAssetPreviewMeta{
		AssetID:          assetID,
		SourceType:       sourceType,
		SourceRef:        sourceRef,
		Status:           domain.AssetWorkbenchPreviewStatusNotApplicable,
		PreviewAvailable: false,
	}
	if info == nil {
		return meta
	}
	meta.Filename = info.Filename
	meta.MimeType = strings.TrimSpace(info.MimeType)
	meta.ExpiresAt = info.ExpiresAt
	if info.DownloadURL != nil {
		meta.DownloadURL = strings.TrimSpace(*info.DownloadURL)
	}
	if meta.DownloadURL != "" && (info.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(meta.MimeType, info.Filename)) {
		meta.Status = domain.AssetWorkbenchPreviewStatusReady
		meta.PreviewURL = meta.DownloadURL
		meta.PreviewAvailable = true
	}
	if strings.Contains(strings.TrimSpace(info.AccessHint), "prepare_required") {
		meta.Status = domain.AssetWorkbenchPreviewStatusPending
		meta.Preparing = true
	}
	return meta
}

func (s *Service) SystemAssetBatchDownloadManifest(ctx context.Context, actor domain.RequestActor, params SystemAssetBatchDownloadParams) (*assetcenter.BatchDownloadManifest, *domain.AppError) {
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can batch download system assets from workbench.", nil)
	}
	if len(params.AssetIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_ids is required.", nil)
	}
	if s.systemDownloads == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset downloader is not configured.", nil)
	}
	manifest, appErr := s.systemDownloads.BuildBatchDownloadManifest(
		ctx,
		params.AssetIDs,
		assetcenter.WithBatchDownloadNamingMode(assetcenter.NormalizeBatchDownloadNamingMode(params.NamingMode)),
	)
	if appErr != nil {
		return nil, appErr
	}
	_ = s.recordSystemAssetDownloadEvent(ctx, actor, domain.AssetWorkbenchEventSystemAssetBatchDownloaded, nil, map[string]interface{}{
		"requested_count": len(params.AssetIDs),
		"success_count":   manifest.SuccessCount,
		"failure_count":   manifest.FailureCount,
		"naming_mode":     assetcenter.NormalizeBatchDownloadNamingMode(params.NamingMode),
	})
	return manifest, nil
}

func (s *Service) ListClientMaterials(ctx context.Context, actor domain.RequestActor, admin bool) ([]*domain.AssetWorkbenchClientMaterial, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if admin {
		if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can list client materials for administration.", nil)
		}
		items, err := s.repo.ListClientMaterials(ctx, repo.AssetWorkbenchClientMaterialFilter{})
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench client materials.", err.Error())
		}
		s.hydrateClientMaterialRows(ctx, items)
		return items, nil
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can list client materials.", nil)
	}
	enabled := true
	items, err := s.repo.ListClientMaterials(ctx, repo.AssetWorkbenchClientMaterialFilter{Enabled: &enabled})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench client materials.", err.Error())
	}
	s.hydrateClientMaterialRows(ctx, items)
	return items, nil
}

func (s *Service) CreateClientMaterial(ctx context.Context, actor domain.RequestActor, params CreateClientMaterialParams) (*domain.AssetWorkbenchClientMaterial, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can publish client materials.", nil)
	}
	source, appErr := resolveClientMaterialSourceInput(params.AssetID, params.SourceType, params.SourceRef, params.ResourceID)
	if appErr != nil {
		return nil, appErr
	}
	snapshot, appErr := s.clientMaterialSourceSnapshot(ctx, source)
	if appErr != nil {
		return nil, appErr
	}
	now := s.nowFn().UTC()
	item := &domain.AssetWorkbenchClientMaterial{
		AssetID:          snapshot.AssetID,
		SourceType:       snapshot.SourceType,
		SourceRef:        snapshot.SourceRef,
		ResourceID:       snapshot.ResourceID,
		SourceLabel:      snapshot.SourceLabel,
		Title:            clientMaterialTitle(params.Title, snapshot.AssetID, &domain.AssetDownloadInfo{Filename: snapshot.Filename}),
		Description:      strings.TrimSpace(params.Description),
		FilenameSnapshot: snapshot.Filename,
		MimeTypeSnapshot: snapshot.MimeType,
		FileSizeSnapshot: snapshot.FileSize,
		ScopeSKUCode:     snapshot.ScopeSKUCode,
		SKUCode:          snapshot.SKUCode,
		PrimarySKUCode:   snapshot.PrimarySKUCode,
		PreviewAvailable: snapshot.PreviewAvailable,
		Enabled:          boolValueDefault(params.Enabled, true),
		SortOrder:        params.SortOrder,
		PublishedBy:      actor.ID,
		PublishedAt:      now,
	}
	var created *domain.AssetWorkbenchClientMaterial
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateClientMaterial(ctx, tx, item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventClientMaterialUpserted, domain.AssetWorkbenchEntityClientMaterial, &created.ID, nil, created, "publish client material")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to publish asset workbench client material.", err.Error())
	}
	s.hydrateClientMaterialRows(ctx, []*domain.AssetWorkbenchClientMaterial{created})
	return created, nil
}

func (s *Service) UpdateClientMaterial(ctx context.Context, actor domain.RequestActor, materialID int64, params UpdateClientMaterialParams) (*domain.AssetWorkbenchClientMaterial, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can update client materials.", nil)
	}
	if materialID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "material_id is required.", nil)
	}
	existing, err := s.repo.GetClientMaterial(ctx, materialID)
	if err != nil {
		return nil, mapRepoReadError(err, "Client material not found.", "Failed to load client material.")
	}
	item := *existing
	normalizeClientMaterialRow(&item)
	if params.AssetID != nil || params.SourceType != nil || params.SourceRef != nil || params.ResourceID != nil {
		sourceType := item.SourceType
		sourceRef := item.SourceRef
		resourceID := item.ResourceID
		assetID := item.AssetID
		if params.AssetID != nil {
			assetID = *params.AssetID
			if params.SourceType == nil && params.SourceRef == nil && params.ResourceID == nil {
				sourceType = string(domain.AssetResourceSourceSystem)
				sourceRef = ""
				resourceID = ""
			}
		}
		if params.SourceType != nil {
			sourceType = *params.SourceType
		}
		if params.SourceRef != nil {
			sourceRef = *params.SourceRef
		}
		if params.ResourceID != nil {
			resourceID = *params.ResourceID
		}
		source, appErr := resolveClientMaterialSourceInput(assetID, sourceType, sourceRef, resourceID)
		if appErr != nil {
			return nil, appErr
		}
		snapshot, appErr := s.clientMaterialSourceSnapshot(ctx, source)
		if appErr != nil {
			return nil, appErr
		}
		item.AssetID = snapshot.AssetID
		item.SourceType = snapshot.SourceType
		item.SourceRef = snapshot.SourceRef
		item.ResourceID = snapshot.ResourceID
		item.SourceLabel = snapshot.SourceLabel
		item.FilenameSnapshot = snapshot.Filename
		item.MimeTypeSnapshot = snapshot.MimeType
		item.FileSizeSnapshot = snapshot.FileSize
		item.ScopeSKUCode = snapshot.ScopeSKUCode
		item.SKUCode = snapshot.SKUCode
		item.PrimarySKUCode = snapshot.PrimarySKUCode
		item.PreviewAvailable = snapshot.PreviewAvailable
		if params.Title == nil && strings.TrimSpace(item.Title) == "" {
			item.Title = clientMaterialTitle("", item.AssetID, &domain.AssetDownloadInfo{Filename: item.FilenameSnapshot})
		}
	}
	if params.Title != nil {
		item.Title = clientMaterialTitle(*params.Title, item.AssetID, &domain.AssetDownloadInfo{Filename: item.FilenameSnapshot})
	}
	if params.Description != nil {
		item.Description = strings.TrimSpace(*params.Description)
	}
	if params.Enabled != nil {
		item.Enabled = *params.Enabled
	}
	if params.SortOrder != nil {
		item.SortOrder = *params.SortOrder
	}
	item.UpdatedBy = &actor.ID
	var updated *domain.AssetWorkbenchClientMaterial
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		updated, err = s.repo.UpdateClientMaterial(ctx, tx, &item)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventClientMaterialUpserted, domain.AssetWorkbenchEntityClientMaterial, &updated.ID, existing, updated, "update client material")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, mapRepoReadError(err, "Client material not found.", "Failed to update asset workbench client material.")
	}
	s.hydrateClientMaterialRows(ctx, []*domain.AssetWorkbenchClientMaterial{updated})
	return updated, nil
}

func (s *Service) DeleteClientMaterial(ctx context.Context, actor domain.RequestActor, materialID int64) *domain.AppError {
	if err := s.requireRepo(); err != nil {
		return err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can delete client materials.", nil)
	}
	if materialID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "material_id is required.", nil)
	}
	existing, err := s.repo.GetClientMaterial(ctx, materialID)
	if err != nil {
		return mapRepoReadError(err, "Client material not found.", "Failed to load client material.")
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.DeleteClientMaterial(ctx, tx, materialID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventClientMaterialDeleted, domain.AssetWorkbenchEntityClientMaterial, &materialID, existing, nil, "delete client material")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		return mapRepoReadError(err, "Client material not found.", "Failed to delete asset workbench client material.")
	}
	return nil
}

func (s *Service) BatchUpdateClientMaterials(ctx context.Context, actor domain.RequestActor, params BatchUpdateClientMaterialsParams) (*ClientMaterialBatchUpdateResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can batch update client materials.", nil)
	}
	action := strings.TrimSpace(strings.ToLower(params.Action))
	switch action {
	case "publish", "enable", "disable", "remove":
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "action must be publish, enable, disable or remove.", nil)
	}
	items, asyncRequired, appErr := s.resolveClientMaterialBatchCandidates(ctx, actor, params, assetWorkbenchClientMaterialBatchUpdateLimit)
	if appErr != nil {
		return nil, appErr
	}
	if asyncRequired {
		job, appErr := s.createClientMaterialBatchJob(ctx, actor, action, params, len(items))
		if appErr != nil {
			return nil, appErr
		}
		return &ClientMaterialBatchUpdateResult{
			Requested:     len(items),
			AsyncRequired: true,
			JobID:         job.JobID,
			Job:           job,
			Message:       fmt.Sprintf("选择范围超过同步处理上限 %d 项，已转入批量任务中心后台处理。", assetWorkbenchClientMaterialBatchUpdateLimit),
		}, nil
	}
	if len(items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "items or folders is required.", nil)
	}
	return s.applyClientMaterialBatchUpdate(ctx, actor, action, items, true)
}

func (s *Service) createClientMaterialBatchJob(ctx context.Context, actor domain.RequestActor, action string, params BatchUpdateClientMaterialsParams, requested int) (*domain.AssetWorkbenchBatchJob, *domain.AppError) {
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch request payload is invalid.", err.Error())
	}
	job := &domain.AssetWorkbenchBatchJob{
		JobID:          "awbjob-" + uuid.NewString(),
		JobType:        domain.AssetWorkbenchAsyncJobTypeClientMaterialBatchUpdate,
		Status:         domain.AssetWorkbenchAsyncJobStatusQueued,
		Action:         strings.TrimSpace(strings.ToLower(action)),
		SelectionScope: strings.TrimSpace(params.SelectionScope),
		RequestedBy:    actor.ID,
		RequestPayload: payload,
		TotalCount:     requested,
	}
	if job.SelectionScope == "" {
		switch {
		case len(params.Folders) > 0:
			job.SelectionScope = "folders"
		case strings.TrimSpace(params.Query) != "":
			job.SelectionScope = "query"
		default:
			job.SelectionScope = "items"
		}
	}
	var created *domain.AssetWorkbenchBatchJob
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		created, err = s.repo.CreateBatchJob(ctx, tx, job)
		if err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventBatchJobCreated, domain.AssetWorkbenchEntityBatchJob, &created.ID, nil, created, "create asset workbench batch job")
	}); err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to create asset workbench batch job.", err.Error())
	}
	return created, nil
}

func (s *Service) applyClientMaterialBatchUpdate(ctx context.Context, actor domain.RequestActor, action string, items []CreateClientMaterialParams, recordEvent bool) (*ClientMaterialBatchUpdateResult, *domain.AppError) {
	existingRows, err := s.repo.ListClientMaterials(ctx, repo.AssetWorkbenchClientMaterialFilter{})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench client materials.", err.Error())
	}
	existingBySource := map[string]*domain.AssetWorkbenchClientMaterial{}
	for _, row := range existingRows {
		normalizeClientMaterialRow(row)
		existingBySource[clientMaterialSourceKey(row.SourceType, row.SourceRef)] = row
	}

	result := &ClientMaterialBatchUpdateResult{
		Requested: len(items),
		Items:     []*domain.AssetWorkbenchClientMaterial{},
		Failures:  []ClientMaterialBatchUpdateFailure{},
	}
	seen := map[string]bool{}
	for index, item := range items {
		source, appErr := resolveClientMaterialSourceInput(item.AssetID, item.SourceType, item.SourceRef, item.ResourceID)
		if appErr != nil {
			result.addClientMaterialBatchFailure(index, item, appErr.Message)
			continue
		}
		key := clientMaterialSourceKey(source.SourceType, source.SourceRef)
		if seen[key] {
			result.Skipped++
			continue
		}
		seen[key] = true
		existing := existingBySource[key]
		switch action {
		case "publish":
			if existing != nil {
				if existing.Enabled {
					result.Skipped++
					continue
				}
				enabled := true
				updated, appErr := s.UpdateClientMaterial(ctx, actor, existing.ID, UpdateClientMaterialParams{Enabled: &enabled})
				if appErr != nil {
					result.addClientMaterialBatchFailure(index, item, appErr.Message)
					continue
				}
				result.Updated++
				result.Enabled++
				result.Items = append(result.Items, updated)
				existingBySource[key] = updated
				continue
			}
			enabled := true
			item.Enabled = &enabled
			created, appErr := s.CreateClientMaterial(ctx, actor, item)
			if appErr != nil {
				result.addClientMaterialBatchFailure(index, item, appErr.Message)
				continue
			}
			result.Created++
			result.Enabled++
			result.Items = append(result.Items, created)
			existingBySource[key] = created
		case "enable", "disable":
			if existing == nil {
				result.Skipped++
				continue
			}
			nextEnabled := action == "enable"
			if existing.Enabled == nextEnabled {
				result.Skipped++
				continue
			}
			updated, appErr := s.UpdateClientMaterial(ctx, actor, existing.ID, UpdateClientMaterialParams{Enabled: &nextEnabled})
			if appErr != nil {
				result.addClientMaterialBatchFailure(index, item, appErr.Message)
				continue
			}
			result.Updated++
			if nextEnabled {
				result.Enabled++
			} else {
				result.Disabled++
			}
			result.Items = append(result.Items, updated)
			existingBySource[key] = updated
		case "remove":
			if existing == nil {
				result.Skipped++
				continue
			}
			if appErr := s.DeleteClientMaterial(ctx, actor, existing.ID); appErr != nil {
				result.addClientMaterialBatchFailure(index, item, appErr.Message)
				continue
			}
			result.Removed++
			delete(existingBySource, key)
		}
	}
	result.Failed = len(result.Failures)
	if result.Failed == 0 {
		result.Failures = nil
	}
	if len(result.Items) == 0 {
		result.Items = nil
	}
	if recordEvent {
		_ = s.tx.RunInTx(ctx, func(tx repo.Tx) error {
			return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventClientMaterialBatchUpdated, domain.AssetWorkbenchEntityClientMaterial, nil, nil, result, "batch update client materials")
		})
	}
	return result, nil
}

func (s *Service) ListBatchJobs(ctx context.Context, actor domain.RequestActor, status string, page int, pageSize int) (*BatchJobListResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can view batch jobs.", nil)
	}
	page, pageSize = normalizeServicePage(page, pageSize, 50, 100)
	items, total, err := s.repo.ListBatchJobs(ctx, repo.AssetWorkbenchBatchJobFilter{
		JobType:  domain.AssetWorkbenchAsyncJobTypeClientMaterialBatchUpdate,
		Status:   strings.TrimSpace(status),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench batch jobs.", err.Error())
	}
	return &BatchJobListResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (s *Service) GetBatchJob(ctx context.Context, actor domain.RequestActor, jobID string) (*domain.AssetWorkbenchBatchJob, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can view batch jobs.", nil)
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_id is required.", nil)
	}
	job, err := s.repo.GetBatchJob(ctx, jobID)
	if err != nil {
		return nil, mapRepoReadError(err, "Batch job not found.", "Failed to load asset workbench batch job.")
	}
	return job, nil
}

func (s *Service) ProcessPendingBatchJobs(ctx context.Context, workerID string, limit int) (int, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return 0, err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "asset-workbench-batch-" + uuid.NewString()
	}
	if limit <= 0 {
		limit = 1
	}
	var jobs []*domain.AssetWorkbenchBatchJob
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		jobs, err = s.repo.ClaimQueuedBatchJobs(ctx, tx, workerID, limit, s.nowFn().UTC().Add(s.cfg.BatchJobWorkerLeaseTTL))
		return err
	}); err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to claim asset workbench batch jobs.", err.Error())
	}
	processed := 0
	for _, job := range jobs {
		if job == nil {
			continue
		}
		if appErr := s.processBatchJob(ctx, job); appErr != nil {
			return processed, appErr
		}
		processed++
	}
	return processed, nil
}

func (s *Service) processBatchJob(ctx context.Context, job *domain.AssetWorkbenchBatchJob) *domain.AppError {
	if job.JobType != domain.AssetWorkbenchAsyncJobTypeClientMaterialBatchUpdate {
		return s.failBatchJob(ctx, job, "unsupported batch job type: "+job.JobType)
	}
	var params BatchUpdateClientMaterialsParams
	if len(job.RequestPayload) == 0 || !json.Valid(job.RequestPayload) {
		return s.failBatchJob(ctx, job, "batch job request payload is invalid")
	}
	if err := json.Unmarshal(job.RequestPayload, &params); err != nil {
		return s.failBatchJob(ctx, job, "decode batch job request payload: "+err.Error())
	}
	action := strings.TrimSpace(strings.ToLower(firstNonEmpty(job.Action, params.Action)))
	actor := domain.RequestActor{
		ID:       job.RequestedBy,
		Roles:    []domain.Role{domain.RoleAssetManager},
		Source:   "asset_workbench_batch_worker",
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
	items, asyncRequired, appErr := s.resolveClientMaterialBatchCandidates(ctx, actor, params, assetWorkbenchClientMaterialBatchAsyncLimit)
	if appErr != nil {
		return s.failBatchJob(ctx, job, appErr.Message)
	}
	if asyncRequired {
		return s.failBatchJob(ctx, job, fmt.Sprintf("选择范围超过异步处理上限 %d 项，请缩小目录或筛选条件。", assetWorkbenchClientMaterialBatchAsyncLimit))
	}
	job.TotalCount = len(items)
	job.ProcessedCount = 0
	job.ResultPayload = mustJSON(map[string]interface{}{"message": "批量任务已开始处理", "total": len(items)})
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.repo.UpdateBatchJobProgress(ctx, tx, job)
	}); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to update batch job progress.", err.Error())
	}
	result, appErr := s.applyClientMaterialBatchUpdate(ctx, actor, action, items, true)
	if appErr != nil {
		return s.failBatchJob(ctx, job, appErr.Message)
	}
	now := s.nowFn().UTC()
	job.Status = domain.AssetWorkbenchAsyncJobStatusSucceeded
	job.ProcessedCount = len(items)
	job.CreatedCount = result.Created
	job.UpdatedCount = result.Updated
	job.EnabledCount = result.Enabled
	job.DisabledCount = result.Disabled
	job.RemovedCount = result.Removed
	job.SkippedCount = result.Skipped
	job.FailedCount = result.Failed
	job.ErrorMessage = ""
	job.FinishedAt = &now
	job.ResultPayload = mustJSON(result)
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.CompleteBatchJob(ctx, tx, job); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventBatchJobCompleted, domain.AssetWorkbenchEntityBatchJob, &job.ID, nil, job, "complete asset workbench batch job")
	}); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to complete asset workbench batch job.", err.Error())
	}
	return nil
}

func (s *Service) failBatchJob(ctx context.Context, job *domain.AssetWorkbenchBatchJob, message string) *domain.AppError {
	if job == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Batch job is nil.", message)
	}
	now := s.nowFn().UTC()
	job.Status = domain.AssetWorkbenchAsyncJobStatusFailed
	job.ErrorMessage = trimString(message, 1000)
	job.FinishedAt = &now
	job.ResultPayload = mustJSON(map[string]interface{}{
		"message": job.ErrorMessage,
		"status":  domain.AssetWorkbenchAsyncJobStatusFailed,
	})
	actor := domain.RequestActor{
		ID:       job.RequestedBy,
		Roles:    []domain.Role{domain.RoleAssetManager},
		Source:   "asset_workbench_batch_worker",
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.CompleteBatchJob(ctx, tx, job); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventBatchJobCompleted, domain.AssetWorkbenchEntityBatchJob, &job.ID, nil, job, "fail asset workbench batch job")
	}); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to mark asset workbench batch job failed.", err.Error())
	}
	return domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench batch job failed.", job.ErrorMessage)
}

func (s *Service) resolveClientMaterialBatchCandidates(ctx context.Context, actor domain.RequestActor, params BatchUpdateClientMaterialsParams, limit int) ([]CreateClientMaterialParams, bool, *domain.AppError) {
	candidates := make([]CreateClientMaterialParams, 0, len(params.Items))
	candidates = append(candidates, params.Items...)
	if len(candidates) > limit {
		return candidates, true, nil
	}
	addAsset := func(asset *assetcenter.AssetDetail) bool {
		if asset == nil || !assetWorkbenchMaterialAssetVisible(asset) {
			return false
		}
		candidates = append(candidates, clientMaterialParamsFromAssetDetail(asset))
		return len(candidates) > limit
	}
	if strings.TrimSpace(params.Query) != "" {
		page := 1
		for {
			search, appErr := s.SystemSearch(ctx, actor, params.Query, page, 100, params.Source, params.FormatCategory, params.BusinessLane)
			if appErr != nil {
				return nil, false, appErr
			}
			for _, asset := range search.Items {
				if addAsset(asset) {
					return candidates, true, nil
				}
			}
			if len(search.Items) == 0 || int64(page*search.Size) >= search.Total {
				break
			}
			page++
		}
	}
	for _, folder := range params.Folders {
		if strings.TrimSpace(folder.FormatCategory) == "" {
			folder.FormatCategory = params.FormatCategory
		}
		if strings.TrimSpace(folder.BusinessLane) == "" {
			folder.BusinessLane = params.BusinessLane
		}
		asyncRequired, appErr := s.appendClientMaterialBatchFolderCandidates(ctx, actor, folder, limit, &candidates)
		if appErr != nil || asyncRequired {
			return candidates, asyncRequired, appErr
		}
	}
	return candidates, false, nil
}

func (s *Service) appendClientMaterialBatchFolderCandidates(ctx context.Context, actor domain.RequestActor, folder ClientMaterialBatchFolderParams, limit int, candidates *[]CreateClientMaterialParams) (bool, *domain.AppError) {
	source := firstNonEmpty(folder.Source, string(domain.AssetResourceSourceAll))
	formatCategory := strings.TrimSpace(folder.FormatCategory)
	businessLane := strings.TrimSpace(folder.BusinessLane)
	queue := []string{folder.Path}
	visited := map[string]bool{}
	for len(queue) > 0 {
		current := strings.TrimSpace(queue[0])
		queue = queue[1:]
		normalized := normalizeWorkbenchMaterialPath(current)
		if visited[normalized] {
			continue
		}
		visited[normalized] = true
		if !assetWorkbenchOperationalMaterialPathVisible(normalized) {
			continue
		}
		page := 1
		for {
			browse, appErr := s.BrowseMaterials(ctx, actor, normalized, page, 100, source, formatCategory, businessLane)
			if appErr != nil {
				return false, appErr
			}
			if folder.IncludeChildren && page == 1 {
				for _, child := range browse.Folders {
					if assetWorkbenchOperationalMaterialPathVisible(child.Path) {
						queue = append(queue, child.Path)
					}
				}
			}
			for _, asset := range browse.Files {
				if !assetWorkbenchMaterialAssetVisible(asset) {
					continue
				}
				*candidates = append(*candidates, clientMaterialParamsFromAssetDetail(asset))
				if len(*candidates) > limit {
					return true, nil
				}
			}
			if len(browse.Files) == 0 || int64(page*browse.Size) >= browse.Total {
				break
			}
			page++
		}
	}
	return false, nil
}

func clientMaterialParamsFromAssetDetail(asset *assetcenter.AssetDetail) CreateClientMaterialParams {
	if asset == nil {
		return CreateClientMaterialParams{}
	}
	source := domain.NormalizeAssetResourceSource(asset.SourceType)
	if source == domain.AssetResourceSourceAll {
		source = domain.AssetResourceSourceSystem
	}
	sourceRef := strings.TrimSpace(asset.ResourceID)
	if source == domain.AssetResourceSourceExternal {
		if sourceRef == "" {
			sourceRef = domain.ExternalAssetResourceID(asset.ID)
		}
	} else {
		sourceRef = strconv.FormatInt(asset.ID, 10)
	}
	enabled := true
	return CreateClientMaterialParams{
		AssetID:     asset.ID,
		SourceType:  string(source),
		SourceRef:   sourceRef,
		ResourceID:  sourceRef,
		Title:       firstNonEmpty(asset.ProductName, asset.OriginalFilename, asset.FileName, sourceRef),
		Description: "",
		Enabled:     &enabled,
	}
}

func (s *Service) ClientMaterialDownload(ctx context.Context, actor domain.RequestActor, materialID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can download client materials.", nil)
	}
	material, appErr := s.resolveDownloadableClientMaterial(ctx, actor, materialID)
	if appErr != nil {
		return nil, appErr
	}
	info, appErr := s.clientMaterialDownloadInfo(ctx, material)
	if appErr != nil {
		return nil, appErr
	}
	if info != nil && info.DownloadURL != nil && strings.TrimSpace(*info.DownloadURL) != "" {
		_ = s.recordSystemAssetDownloadEvent(ctx, actor, domain.AssetWorkbenchEventClientMaterialDownloaded, &material.ID, map[string]interface{}{
			"material_id": material.ID,
			"asset_id":    material.AssetID,
			"source_type": material.SourceType,
			"source_ref":  material.SourceRef,
			"filename":    info.Filename,
			"mode":        info.DownloadMode,
		})
	}
	return info, nil
}

func (s *Service) ClientMaterialPreview(ctx context.Context, actor domain.RequestActor, materialID int64) (*SystemAssetPreviewMeta, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can preview client materials.", nil)
	}
	material, appErr := s.resolveDownloadableClientMaterial(ctx, actor, materialID)
	if appErr != nil {
		return nil, appErr
	}
	return s.clientMaterialPreviewMeta(ctx, material)
}

func (s *Service) ClientMaterialBatchDownloadManifest(ctx context.Context, actor domain.RequestActor, params ClientMaterialBatchDownloadParams) (*ClientMaterialBatchDownloadManifest, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset workbench users can batch download client materials.", nil)
	}
	materialIDs := positiveUniqueInt64s(params.MaterialIDs)
	if len(materialIDs) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "material_ids is required.", nil)
	}
	if len(materialIDs) > assetWorkbenchClientMaterialBatchDownloadLimit {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "material_ids exceed batch download limit", map[string]interface{}{
			"limit": assetWorkbenchClientMaterialBatchDownloadLimit,
		})
	}
	manifest := &ClientMaterialBatchDownloadManifest{
		Items:    make([]ClientMaterialBatchDownloadItem, 0, len(materialIDs)),
		Failures: make([]ClientMaterialBatchDownloadFailure, 0),
	}
	usedNames := map[string]int{}
	var totalSize int64
	for _, materialID := range materialIDs {
		material, appErr := s.resolveDownloadableClientMaterial(ctx, actor, materialID)
		if appErr != nil {
			manifest.Failures = append(manifest.Failures, ClientMaterialBatchDownloadFailure{MaterialID: materialID, Reason: appErr.Code})
			continue
		}
		info, appErr := s.clientMaterialDownloadInfo(ctx, material)
		if appErr != nil {
			manifest.Failures = append(manifest.Failures, ClientMaterialBatchDownloadFailure{
				MaterialID: material.ID,
				AssetID:    material.AssetID,
				SourceType: material.SourceType,
				SourceRef:  material.SourceRef,
				Filename:   material.FilenameSnapshot,
				Reason:     appErr.Code,
			})
			continue
		}
		if info == nil || info.DownloadURL == nil || strings.TrimSpace(*info.DownloadURL) == "" {
			manifest.Failures = append(manifest.Failures, ClientMaterialBatchDownloadFailure{
				MaterialID: material.ID,
				AssetID:    material.AssetID,
				SourceType: material.SourceType,
				SourceRef:  material.SourceRef,
				Filename:   material.FilenameSnapshot,
				Reason:     "download_url_unavailable",
			})
			continue
		}
		if info.FileSize > 0 && totalSize+info.FileSize > assetcenter.MaxBatchDownloadTotalBytes {
			manifest.Failures = append(manifest.Failures, ClientMaterialBatchDownloadFailure{
				MaterialID: material.ID,
				AssetID:    material.AssetID,
				SourceType: material.SourceType,
				SourceRef:  material.SourceRef,
				Filename:   material.FilenameSnapshot,
				Reason:     "total_size_limit_exceeded",
			})
			continue
		}
		filename := uniqueWorkbenchDownloadFilename(firstNonEmpty(info.Filename, material.FilenameSnapshot, fmt.Sprintf("client-material-%d", material.ID)), material.ID, usedNames)
		totalSize += info.FileSize
		if manifest.ExpiresAt == nil || (info.ExpiresAt != nil && info.ExpiresAt.Before(*manifest.ExpiresAt)) {
			manifest.ExpiresAt = info.ExpiresAt
		}
		manifest.Items = append(manifest.Items, ClientMaterialBatchDownloadItem{
			MaterialID:  material.ID,
			AssetID:     material.AssetID,
			SourceType:  material.SourceType,
			SourceRef:   material.SourceRef,
			Filename:    filename,
			FileSize:    info.FileSize,
			MimeType:    strings.TrimSpace(info.MimeType),
			DownloadURL: strings.TrimSpace(*info.DownloadURL),
			ExpiresAt:   info.ExpiresAt,
		})
	}
	if len(manifest.Items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeAssetMissing, "all requested client materials are unavailable for download", map[string]interface{}{
			"material_ids":   materialIDs,
			"failure_count":  len(manifest.Failures),
			"total_size_max": assetcenter.MaxBatchDownloadTotalBytes,
		})
	}
	manifest.SuccessCount = len(manifest.Items)
	manifest.FailureCount = len(manifest.Failures)
	manifest.TotalSize = totalSize
	_ = s.recordSystemAssetDownloadEvent(ctx, actor, domain.AssetWorkbenchEventClientMaterialBatchDownload, nil, map[string]interface{}{
		"material_ids":    materialIDs,
		"requested_count": len(materialIDs),
		"success_count":   manifest.SuccessCount,
		"failure_count":   manifest.FailureCount,
		"naming_mode":     assetcenter.NormalizeBatchDownloadNamingMode(params.NamingMode),
	})
	return manifest, nil
}

func (s *Service) ProcessPendingPreviews(ctx context.Context, limit int) (int, *domain.AppError) {
	if s.repo == nil || s.tx == nil {
		return 0, nil
	}
	workerID := "asset-workbench-preview-" + uuid.NewString()
	files, err := s.repo.ClaimPendingPreviewFiles(ctx, repo.AssetWorkbenchPreviewClaim{
		WorkerID: workerID,
		Now:      s.nowFn().UTC(),
		LeaseTTL: s.cfg.PreviewWorkerLeaseTTL,
		Limit:    limit,
	})
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "Failed to claim asset workbench preview files.", err.Error())
	}
	processed := 0
	for _, file := range files {
		if file == nil {
			continue
		}
		if err := s.processPreviewFile(ctx, file); err != nil {
			nextAttempts := file.PreviewAttempts + 1
			var nextRetryAt *time.Time
			if nextAttempts < s.cfg.PreviewWorkerMaxAttempts {
				value := s.nowFn().UTC().Add(previewRetryBackoff(nextAttempts))
				nextRetryAt = &value
			}
			message := err.Error()
			if markErr := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
				return s.repo.MarkPreviewFailed(ctx, tx, file.ID, nextAttempts, message, nextRetryAt)
			}); markErr != nil {
				return processed, domain.NewAppError(domain.ErrCodeInternalError, "Failed to mark asset workbench preview failure.", markErr.Error())
			}
			processed++
			continue
		}
		processed++
	}
	return processed, nil
}

func (s *Service) processPreviewFile(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) error {
	if file == nil {
		return nil
	}
	if file.FileType != "design" && file.FileType != "image" && file.FileType != "pdf" {
		return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
			return s.repo.MarkPreviewReady(ctx, tx, file.ID, file.ObjectKey)
		})
	}
	if s.oss == nil || !s.oss.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}
	if s.renderer == nil {
		return fmt.Errorf("asset preview renderer is not configured")
	}
	reader, err := s.oss.OpenObject(ctx, file.ObjectKey)
	if err != nil {
		return fmt.Errorf("open source object: %w", err)
	}
	defer reader.Close()
	inputPath, cleanup, err := writeWorkbenchPreviewSourceTempFile(reader, file.OriginalFilename, file.MimeType)
	if err != nil {
		return err
	}
	defer cleanup()
	content, err := s.renderer.Render(ctx, inputPath, baseservice.AssetPreviewSourceMeta{
		Filename: file.OriginalFilename,
		MimeType: file.MimeType,
	}, baseservice.AssetPreviewRenderSpec{MaxWidth: 1600, MaxHeight: 1600, Quality: 82})
	if err != nil {
		return fmt.Errorf("render preview: %w", err)
	}
	previewKey := s.buildPreviewKey(s.nowFn().UTC(), file.ID)
	if err := s.oss.UploadObject(ctx, previewKey, "image/webp", content); err != nil {
		return fmt.Errorf("upload preview object: %w", err)
	}
	return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.repo.MarkPreviewReady(ctx, tx, file.ID, previewKey)
	})
}

func (s *Service) resolveSubmissionTemplate(ctx context.Context, actor domain.RequestActor, profile *domain.AssetWorkbenchProfile, req CreateSubmissionItemParams) (*domain.AssetWorkbenchTemplate, *domain.AppError) {
	if req.TemplateID <= 0 {
		return nil, nil
	}
	template, err := s.repo.GetTemplate(ctx, req.TemplateID)
	if err != nil {
		return nil, mapRepoReadError(err, "Template not found.", "Failed to load asset workbench template.")
	}
	if !template.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Template is disabled.", map[string]interface{}{"template_id": req.TemplateID})
	}
	workerType := ""
	if profile != nil {
		workerType = strings.TrimSpace(profile.WorkerType)
	}
	if template.WorkerType != "" && template.WorkerType != domain.AssetWorkbenchWorkerTypeAll && workerType != "" && template.WorkerType != workerType {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Template is not available for current worker type.", map[string]interface{}{"template_id": req.TemplateID})
	}
	if !isAssetWorkbenchAdmin(actor) {
		assigned, err := s.repo.IsTemplateAssignedToUser(ctx, actor.ID, req.TemplateID)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify template assignment.", err.Error())
		}
		if !assigned {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Template is not assigned to current user.", map[string]interface{}{"template_id": req.TemplateID})
		}
	}
	return template, nil
}

func (s *Service) buildSubmissionItem(ctx context.Context, payeeUserID, submissionID int64, submittedAt time.Time, businessMonth string, profile *domain.AssetWorkbenchProfile, req CreateSubmissionItemParams, templates ...*domain.AssetWorkbenchTemplate) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	var template *domain.AssetWorkbenchTemplate
	if len(templates) > 0 {
		template = templates[0]
	}
	orderNo := strings.TrimSpace(req.OrderNo)
	if orderNo == "" {
		orderNo = "AWF" + submittedAt.Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	}
	difficulty := strings.TrimSpace(req.DifficultyClass)
	if template != nil {
		difficulty = strings.TrimSpace(template.DifficultyClass)
	}
	if difficulty == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class is required.", nil)
	}
	normalizedDifficulty, appErr := normalizeWorkbenchDifficultyCode(difficulty, false)
	if appErr != nil {
		return nil, appErr
	}
	difficulty = normalizedDifficulty
	if req.PageCount <= 0 {
		req.PageCount = 1
	}
	if req.ItemCount <= 0 {
		req.ItemCount = 1
	}
	item := &domain.AssetWorkbenchSubmissionItem{
		SubmissionID:       submissionID,
		PayeeUserID:        payeeUserID,
		OrderNo:            orderNo,
		DifficultyClass:    difficulty,
		Finalized:          req.Finalized,
		PageCount:          req.PageCount,
		ItemCount:          req.ItemCount,
		BusinessMonth:      businessMonth,
		SubmittedAt:        submittedAt,
		WorkerTypeSnapshot: strings.TrimSpace(profile.WorkerType),
		JobGradeSnapshot:   strings.TrimSpace(profile.JobGrade),
		QCStatus:           domain.AssetWorkbenchSubmissionStatusSubmitted,
		SettlementStatus:   domain.AssetWorkbenchSettlementStatusUnsettled,
	}
	if template != nil {
		templateID := template.ID
		item.TemplateID = &templateID
		item.TemplateNameSnapshot = template.Name
		item.CategorySnapshot = template.Category
	}
	if item.WorkerTypeSnapshot == "" || item.JobGradeSnapshot == "" {
		item.PricingStatus = domain.AssetWorkbenchPricingStatusPendingGrade
		item.PricingSnapshot = mustJSON(map[string]interface{}{
			"status":        "pending_grade",
			"reason":        "missing worker_type or job_grade",
			"template_id":   item.TemplateID,
			"template_name": item.TemplateNameSnapshot,
		})
		return item, nil
	}
	price, err := s.repo.FindActivePrice(ctx, item.WorkerTypeSnapshot, item.JobGradeSnapshot, item.DifficultyClass, submittedAt.In(s.loc))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			item.PricingStatus = domain.AssetWorkbenchPricingStatusUnpriced
			item.PricingSnapshot = mustJSON(map[string]interface{}{
				"status":           "unpriced",
				"worker_type":      item.WorkerTypeSnapshot,
				"job_grade":        item.JobGradeSnapshot,
				"difficulty_class": item.DifficultyClass,
				"business_month":   businessMonth,
				"template_id":      item.TemplateID,
				"template_name":    item.TemplateNameSnapshot,
				"category":         item.CategorySnapshot,
			})
			return item, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to match asset workbench price.", err.Error())
	}
	unitPrice := price.UnitPrice
	promo, appErr := s.applyPromoCoupon(ctx, payeeUserID, orderNo, price.UnitPrice, item, submittedAt)
	if appErr != nil {
		return nil, appErr
	}
	if promo != nil {
		item.PromoCouponID = &promo.Coupon.ID
		item.PromoSnapshot = promo.Snapshot
		unitPrice = promo.UnitPrice
	}
	item.BasePriceRuleID = &price.ID
	item.BaseUnitPrice = &price.UnitPrice
	item.GrossAmount = unitPrice * float64(req.PageCount)
	item.PricingStatus = domain.AssetWorkbenchPricingStatusPriced
	item.PricingSnapshot = mustJSON(map[string]interface{}{
		"status":           "priced",
		"price_rule_id":    price.ID,
		"worker_type":      price.WorkerType,
		"job_grade":        price.JobGrade,
		"difficulty_class": price.DifficultyClass,
		"unit_price":       price.UnitPrice,
		"effective_from":   price.EffectiveFrom.Format("2006-01-02"),
		"effective_to":     formatOptionalDate(price.EffectiveTo),
		"submitted_at":     submittedAt.Format(time.RFC3339),
		"business_month":   businessMonth,
		"template_id":      item.TemplateID,
		"template_name":    item.TemplateNameSnapshot,
		"category":         item.CategorySnapshot,
		"final_unit_price": unitPrice,
		"promo_applied":    promo != nil,
		"gross_formula":    "final_unit_price * page_count",
		"deduction_timing": "settlement_time",
	})
	return item, nil
}

func (s *Service) applyPromoCoupon(ctx context.Context, payeeUserID int64, orderNo string, baseUnitPrice float64, item *domain.AssetWorkbenchSubmissionItem, submittedAt time.Time) (*promoApplication, *domain.AppError) {
	coupons, err := s.repo.ListActivePromoCoupons(ctx, item.WorkerTypeSnapshot, item.JobGradeSnapshot, item.DifficultyClass, submittedAt.In(s.loc))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to match promo coupons.", err.Error())
	}
	applicable := make([]*domain.AssetWorkbenchPromoCoupon, 0, len(coupons))
	for _, coupon := range coupons {
		if promoCouponApplies(coupon, payeeUserID, orderNo) {
			applicable = append(applicable, coupon)
		}
	}
	if len(applicable) == 0 {
		return nil, nil
	}
	sort.SliceStable(applicable, func(i, j int) bool {
		if applicable[i].Priority != applicable[j].Priority {
			return applicable[i].Priority < applicable[j].Priority
		}
		return applicable[i].ID > applicable[j].ID
	})
	var winner *domain.AssetWorkbenchPromoCoupon
	for _, coupon := range applicable {
		if normalizePromoMode(coupon.Mode) == domain.AssetWorkbenchPromoModeFixedPrice {
			winner = coupon
			break
		}
	}
	if winner == nil {
		winner = applicable[0]
	}
	mode := normalizePromoMode(winner.Mode)
	unitPrice := baseUnitPrice
	switch mode {
	case domain.AssetWorkbenchPromoModeFixedPrice:
		if winner.Amount == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "fixed_price promo coupon requires amount.", map[string]string{"coupon_code": winner.CouponCode})
		}
		unitPrice = *winner.Amount
	case domain.AssetWorkbenchPromoModeMarkupAmount:
		if winner.Amount == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "markup_amount promo coupon requires amount.", map[string]string{"coupon_code": winner.CouponCode})
		}
		unitPrice = baseUnitPrice + *winner.Amount
	case domain.AssetWorkbenchPromoModeMarkupRate:
		if winner.Percent == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "markup_rate promo coupon requires percent.", map[string]string{"coupon_code": winner.CouponCode})
		}
		unitPrice = baseUnitPrice * (1 + *winner.Percent/100)
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Unsupported promo coupon mode.", map[string]string{"mode": winner.Mode})
	}
	if unitPrice < 0 {
		unitPrice = 0
	}
	return &promoApplication{
		Coupon:       winner,
		UnitPrice:    unitPrice,
		AppliedLabel: mode,
		Snapshot: mustJSON(map[string]interface{}{
			"coupon_id":        winner.ID,
			"coupon_code":      winner.CouponCode,
			"coupon_name":      winner.CouponName,
			"mode":             mode,
			"amount":           winner.Amount,
			"percent":          winner.Percent,
			"priority":         winner.Priority,
			"base_unit_price":  baseUnitPrice,
			"final_unit_price": unitPrice,
			"stack_policy":     "single_winner",
		}),
	}, nil
}

func (s *Service) appendEvent(ctx context.Context, tx repo.Tx, actor domain.RequestActor, eventType, entityType string, entityID *int64, beforeValue interface{}, afterValue interface{}, reason string) error {
	if s.repo == nil {
		return nil
	}
	var actorID *int64
	if actor.ID > 0 {
		actorID = &actor.ID
	}
	event := &domain.AssetWorkbenchEvent{
		ActorUserID: actorID,
		EventType:   strings.TrimSpace(eventType),
		EntityType:  strings.TrimSpace(entityType),
		EntityID:    entityID,
		Before:      marshalEventJSON(beforeValue),
		After:       marshalEventJSON(afterValue),
		Reason:      strings.TrimSpace(reason),
	}
	if event.EventType == "" || event.EntityType == "" {
		return nil
	}
	_, err := s.repo.AppendEvent(ctx, tx, event)
	return err
}

func (s *Service) recordFileDownloadEvent(ctx context.Context, actor domain.RequestActor, eventType string, entityID *int64, payload interface{}) error {
	if s.repo == nil || s.tx == nil {
		return nil
	}
	return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.appendEvent(ctx, tx, actor, eventType, domain.AssetWorkbenchEntitySubmissionFile, entityID, nil, payload, "download manifest issued")
	})
}

func (s *Service) recordSystemAssetDownloadEvent(ctx context.Context, actor domain.RequestActor, eventType string, entityID *int64, payload interface{}) error {
	if s.repo == nil || s.tx == nil {
		return nil
	}
	return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		return s.appendEvent(ctx, tx, actor, eventType, domain.AssetWorkbenchEntitySystemAsset, entityID, nil, payload, "system asset download manifest issued")
	})
}

func (s *Service) loadMutableSubmissionItem(ctx context.Context, itemID int64) (*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	if itemID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "item_id is required.", nil)
	}
	item, err := s.repo.GetSubmissionItem(ctx, itemID)
	if err != nil {
		return nil, mapRepoReadError(err, "Submission item not found.", "Failed to load submission item.")
	}
	if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item cannot be changed after settlement batch attachment.", map[string]interface{}{
			"item_id":           item.ID,
			"settlement_status": item.SettlementStatus,
		})
	}
	if item.EntryKind == domain.AssetWorkbenchSubmissionEntryKindSupplement {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Supplement upload files must be managed through their supplement record.", map[string]interface{}{
			"item_id":    item.ID,
			"entry_kind": item.EntryKind,
		})
	}
	if item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item is already voided.", map[string]interface{}{"item_id": item.ID})
	}
	return item, nil
}

func (s *Service) requireFileVisible(actor domain.RequestActor, file *domain.AssetWorkbenchSubmissionFile) *domain.AppError {
	if file == nil {
		return domain.NewAppError(domain.ErrCodeNotFound, "Submission file not found.", nil)
	}
	if file.OwnerUserID == actor.ID {
		return nil
	}
	if actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "Submission file is not visible to current user.", nil)
}

func (s *Service) buildFileDownloadMeta(file *domain.AssetWorkbenchSubmissionFile, usedNames map[string]int) (*FileDownloadMeta, *domain.AppError) {
	if file == nil {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "Submission file not found.", nil)
	}
	if s.oss == nil || !s.oss.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "OSS direct download is not enabled.", nil)
	}
	objectKey := strings.TrimSpace(file.ObjectKey)
	if objectKey == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Submission file object key is empty.", nil)
	}
	preferredName := firstNonEmpty(file.DisplayName, file.OriginalFilename)
	if usedNames != nil {
		preferredName = firstNonEmpty(file.RelativePath, preferredName)
	} else {
		preferredName = path.Base(preferredName)
	}
	filename := uniqueWorkbenchDownloadFilename(strings.TrimSpace(preferredName), file.ID, usedNames)
	signed := s.oss.PresignDownloadURLWithFilename(objectKey, filename)
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Download URL is unavailable.", nil)
	}
	return &FileDownloadMeta{
		FileID:      file.ID,
		Filename:    filename,
		MimeType:    file.MimeType,
		FileSize:    file.FileSize,
		DownloadURL: signed.DownloadURL,
		ExpiresAt:   signed.ExpiresAt,
	}, nil
}

func uniqueWorkbenchDownloadFilename(filename string, fileID int64, used map[string]int) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = fmt.Sprintf("asset-workbench-file-%d", fileID)
	}
	if used == nil {
		return filename
	}
	count := used[filename]
	used[filename] = count + 1
	if count == 0 {
		return filename
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("asset-workbench-file-%d", fileID)
	}
	return fmt.Sprintf("%s-%d%s", base, count+1, ext)
}

func marshalEventJSON(value interface{}) json.RawMessage {
	if value == nil {
		return nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		if len(raw) > 0 && json.Valid(raw) {
			return raw
		}
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || !json.Valid(raw) {
		return nil
	}
	return raw
}

func (s *Service) upsertProfile(ctx context.Context, actor domain.RequestActor, profile *domain.AssetWorkbenchProfile, reason string, appendGradePeriod bool) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	var saved *domain.AssetWorkbenchProfile
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		saved, err = s.repo.UpsertProfile(ctx, tx, profile)
		if err != nil {
			return err
		}
		if appendGradePeriod && (profile.WorkerType != "" || profile.JobGrade != "") {
			_, err = s.repo.AppendGradePeriod(ctx, tx, &domain.AssetWorkbenchGradePeriod{
				ProfileID:     saved.ID,
				UserID:        saved.UserID,
				WorkerType:    saved.WorkerType,
				JobGrade:      saved.JobGrade,
				EffectiveFrom: s.nowFn().In(s.loc),
				ChangedBy:     &actor.ID,
				Reason:        strings.TrimSpace(reason),
			})
		}
		if err != nil {
			return err
		}
		if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventProfileUpserted, domain.AssetWorkbenchEntityProfile, &saved.ID, nil, saved, reason); err != nil {
			return err
		}
		return s.autoRepricePendingGradeItemsForProfile(ctx, tx, actor, saved, reason)
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to save asset workbench profile.", err.Error())
	}
	s.notifyProfileCompletionRequired(ctx, saved)
	return saved, nil
}

func (s *Service) autoRepricePendingGradeItemsForProfile(ctx context.Context, tx repo.Tx, actor domain.RequestActor, profile *domain.AssetWorkbenchProfile, reason string) error {
	if profile == nil || profile.UserID <= 0 {
		return nil
	}
	workerType := strings.TrimSpace(profile.WorkerType)
	jobGrade := strings.TrimSpace(profile.JobGrade)
	if workerType == "" || jobGrade == "" {
		return nil
	}
	pricingProfile := &domain.AssetWorkbenchProfile{
		UserID:     profile.UserID,
		WorkerType: workerType,
		JobGrade:   jobGrade,
	}
	eventReason := strings.TrimSpace(reason)
	if eventReason == "" {
		eventReason = "profile grade auto reprice"
	} else {
		eventReason += " | profile grade auto reprice"
	}
	for {
		items, err := s.repo.ListPendingGradeSubmissionItemsForPayee(ctx, tx, profile.UserID, assetWorkbenchProfileAutoRepriceBatchSize)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		refreshIDs := map[int64]struct{}{}
		for _, before := range items {
			if before == nil {
				continue
			}
			repriced, appErr := s.buildSubmissionItem(ctx, before.PayeeUserID, before.SubmissionID, before.SubmittedAt, before.BusinessMonth, pricingProfile, CreateSubmissionItemParams{
				OrderNo:         before.OrderNo,
				DifficultyClass: before.DifficultyClass,
				Finalized:       before.Finalized,
				PageCount:       before.PageCount,
				ItemCount:       before.ItemCount,
			}, nil)
			if appErr != nil {
				return appErr
			}
			repriced.ID = before.ID
			repriced.QCStatus = before.QCStatus
			repriced.SettlementStatus = before.SettlementStatus
			repriced.CurrentSettlementBatchID = before.CurrentSettlementBatchID
			updated, err := s.repo.UpdateSubmissionItemPricing(ctx, tx, repriced)
			if err != nil {
				return err
			}
			refreshIDs[before.SubmissionID] = struct{}{}
			itemID := before.ID
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemRepriced, domain.AssetWorkbenchEntitySubmissionItem, &itemID, before, updated, eventReason); err != nil {
				return err
			}
		}
		submissionIDs := make([]int64, 0, len(refreshIDs))
		for submissionID := range refreshIDs {
			submissionIDs = append(submissionIDs, submissionID)
		}
		sort.Slice(submissionIDs, func(i, j int) bool { return submissionIDs[i] < submissionIDs[j] })
		for _, submissionID := range submissionIDs {
			if err := s.repo.RefreshSubmissionTotals(ctx, tx, submissionID); err != nil {
				return err
			}
		}
		if len(items) < assetWorkbenchProfileAutoRepriceBatchSize {
			return nil
		}
	}
}

func (s *Service) notifyProfileCompletionRequired(ctx context.Context, profile *domain.AssetWorkbenchProfile) {
	if s.notifications == nil || profile == nil || profile.UserID <= 0 || profile.PIICompleted {
		return
	}
	payload := mustJSON(map[string]interface{}{
		"source":         "asset_workbench",
		"reason":         "missing_pii",
		"action":         "complete_profile",
		"profile_id":     profile.ID,
		"user_id":        profile.UserID,
		"missing_fields": missingProfileFields(profile),
	})
	_, _, _ = s.notifications.CreateDedupedNotification(
		ctx,
		profile.UserID,
		domain.NotificationTypeAssetWorkbenchProfileIncomplete,
		payload,
		"asset_workbench_profile_completion",
		fmt.Sprintf("asset_workbench_profile_completion:%d", profile.UserID),
	)
}

func (s *Service) notifySubmissionCreated(ctx context.Context, actor domain.RequestActor, profile *domain.AssetWorkbenchProfile, detail *SubmissionDetail) {
	if s.notifications == nil || s.repo == nil || detail == nil || detail.Submission == nil {
		return
	}
	fileCount := 0
	for _, item := range detail.Items {
		fileCount += len(item.Files)
	}
	uploaderName := strings.TrimSpace(actor.Username)
	if profile != nil && strings.TrimSpace(profile.RealName) != "" {
		uploaderName = strings.TrimSpace(profile.RealName)
	}
	if uploaderName == "" {
		uploaderName = fmt.Sprintf("用户 %d", actor.ID)
	}
	payload := mustJSON(map[string]interface{}{
		"source":         "asset_workbench",
		"action":         "open_upload_overview",
		"submission_id":  detail.Submission.ID,
		"submission_no":  detail.Submission.SubmissionNo,
		"business_month": detail.Submission.BusinessMonth,
		"uploader_id":    actor.ID,
		"uploader_name":  uploaderName,
		"item_count":     len(detail.Items),
		"file_count":     fileCount,
	})
	pageSize := 100
	for page := 1; ; page++ {
		members, total, err := s.repo.ListMembers(ctx, repo.AssetWorkbenchMemberFilter{
			Status:   domain.AppMembershipStatusActive,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return
		}
		for _, member := range members {
			if member == nil || member.UserID <= 0 || member.UserID == actor.ID || !assetWorkbenchNotificationManager(member.Roles) {
				continue
			}
			_, _, _ = s.notifications.CreateDedupedNotification(
				ctx,
				member.UserID,
				domain.NotificationTypeAssetWorkbenchSubmissionCreated,
				payload,
				"asset_workbench_submission_created",
				fmt.Sprintf("asset_workbench_submission_created:%d", detail.Submission.ID),
			)
		}
		if int64(page*pageSize) >= total || len(members) == 0 {
			return
		}
	}
}

func assetWorkbenchNotificationManager(roles []domain.Role) bool {
	for _, role := range roles {
		switch role {
		case domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin:
			return true
		}
	}
	return false
}

func missingProfileFields(profile *domain.AssetWorkbenchProfile) []string {
	if profile == nil {
		return nil
	}
	missing := []string{}
	if strings.TrimSpace(profile.RealName) == "" {
		missing = append(missing, "real_name")
	}
	if profile.Phone == nil || strings.TrimSpace(*profile.Phone) == "" {
		missing = append(missing, "phone")
	}
	if strings.TrimSpace(profile.Province) == "" {
		missing = append(missing, "province")
	}
	if strings.TrimSpace(profile.City) == "" {
		missing = append(missing, "city")
	}
	if profile.IDCard == nil || strings.TrimSpace(*profile.IDCard) == "" {
		missing = append(missing, "id_card")
	}
	if strings.TrimSpace(profile.Gender) == "" {
		missing = append(missing, "gender")
	}
	if strings.TrimSpace(profile.AlipayAccount) == "" {
		missing = append(missing, "alipay_account")
	}
	return missing
}

func validateRequiredClientProfile(profile *domain.AssetWorkbenchProfile) *domain.AppError {
	if missing := missingProfileFields(profile); len(missing) > 0 {
		return domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"请完整填写姓名、手机号、省份、城市、身份证号、性别和支付宝账号。",
			map[string]interface{}{"missing_fields": missing},
		)
	}
	return nil
}

func refreshProfileCompletion(profile *domain.AssetWorkbenchProfile) {
	if profile == nil {
		return
	}
	profile.PIICompleted = len(missingProfileFields(profile)) == 0 &&
		profile.IDCard != nil && assetWorkbenchIDCardPattern.MatchString(strings.TrimSpace(*profile.IDCard)) &&
		(profile.Gender == "female" || profile.Gender == "male")
}

func (s *Service) normalizeProfile(userID, actorID int64, params UpsertProfileParams) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	workerType := normalizeWorkerType(params.WorkerType)
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = domain.AssetWorkbenchProfileStatusPending
	}
	if workerType != "" && workerType != domain.AssetWorkbenchWorkerTypeFulltime && workerType != domain.AssetWorkbenchWorkerTypeParttime {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "worker_type must be fulltime or parttime.", nil)
	}
	jobGrade := strings.TrimSpace(params.JobGrade)
	if jobGrade != "" && !validWorkbenchJobGrade(workerType, jobGrade, false) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_grade is not valid for worker_type.", map[string]string{"worker_type": workerType, "job_grade": jobGrade})
	}
	idCard := strings.TrimSpace(params.IDCard)
	if idCard != "" && !assetWorkbenchIDCardPattern.MatchString(idCard) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "身份证号必须为 18 位数字。", nil)
	}
	gender := strings.TrimSpace(params.Gender)
	if gender != "" && gender != "female" && gender != "male" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "性别只能选择女或男。", nil)
	}
	createdBy := actorID
	updatedBy := actorID
	profile := &domain.AssetWorkbenchProfile{
		UserID:        userID,
		WorkerType:    workerType,
		JobGrade:      jobGrade,
		RealName:      strings.TrimSpace(params.RealName),
		Phone:         stringPtr(strings.TrimSpace(params.Phone)),
		Province:      strings.TrimSpace(params.Province),
		City:          strings.TrimSpace(params.City),
		IDCard:        stringPtr(idCard),
		Gender:        gender,
		AlipayAccount: strings.TrimSpace(params.AlipayAccount),
		OnboardedAt:   params.OnboardedAt,
		GradeHidden:   params.GradeHidden,
		Status:        status,
		CreatedBy:     &createdBy,
		UpdatedBy:     &updatedBy,
	}
	refreshProfileCompletion(profile)
	return profile, nil
}

func (s *Service) loadSettleableItems(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, *domain.AppError) {
	var items []*domain.AssetWorkbenchSubmissionItem
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = s.repo.LockSettleableItems(ctx, tx, businessMonth)
		return err
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load settleable submission items.", err.Error())
	}
	return items, nil
}

func (s *Service) loadSettleableSupplements(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchSettlementSupplement, *domain.AppError) {
	var items []*domain.AssetWorkbenchSettlementSupplement
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		items, err = s.repo.LockSettleableSupplements(ctx, tx, businessMonth)
		return err
	}); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load settleable settlement supplements.", err.Error())
	}
	return items, nil
}

func (s *Service) buildSettlementPreview(ctx context.Context, businessMonth string, items []*domain.AssetWorkbenchSubmissionItem, errorRecords []*domain.AssetWorkbenchErrorRecord, supplements []*domain.AssetWorkbenchSettlementSupplement) (*SettlementPreview, *domain.AppError) {
	rowsByPayee := map[int64]*SettlementPreviewRow{}
	normalPayrollRowsByPayee := map[int64]*SettlementPayrollRow{}
	supplementPayrollRowsByPayee := map[int64]*SettlementPayrollRow{}
	profiles := map[int64]*domain.AssetWorkbenchProfile{}
	deductionCache := map[string]deductionRuleCacheEntry{}
	total := SettlementPreviewRow{}
	ensureNormalPayrollRow := func(payeeID int64) *SettlementPayrollRow {
		row := normalPayrollRowsByPayee[payeeID]
		if row == nil {
			row = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeNormalPiecework,
			}
			normalPayrollRowsByPayee[payeeID] = row
		}
		return row
	}
	ensureSupplementPayrollRow := func(payeeID int64) *SettlementPayrollRow {
		row := supplementPayrollRowsByPayee[payeeID]
		if row == nil {
			row = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeSupplementPiecework,
			}
			supplementPayrollRowsByPayee[payeeID] = row
		}
		return row
	}
	for _, item := range items {
		row := rowsByPayee[item.PayeeUserID]
		if row == nil {
			row = &SettlementPreviewRow{PayeeUserID: item.PayeeUserID}
			rowsByPayee[item.PayeeUserID] = row
		}
		normalPayrollRow := ensureNormalPayrollRow(item.PayeeUserID)
		row.ItemCount++
		row.PageCount += item.PageCount
		row.GrossAmount += item.GrossAmount
		normalPayrollRow.ItemCount++
		normalPayrollRow.PageCount += item.PageCount
		normalPayrollRow.GrossAmount += item.GrossAmount
		errorCount := matchedErrorCount(errorRecords, item)
		row.ErrorCount += errorCount
		normalPayrollRow.ErrorCount += errorCount
		if errorCount > 0 {
			deduction, _, appErr := s.calculateDeduction(ctx, item, errorCount)
			if appErr != nil {
				return nil, appErr
			}
			row.DeductionAmount += deduction
			normalPayrollRow.DeductionAmount += deduction
		}
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount
		normalPayrollRow.NetAmount = normalPayrollRow.GrossAmount - normalPayrollRow.DeductionAmount + normalPayrollRow.WelfareAmount + normalPayrollRow.AdjustmentAmount
	}
	for _, record := range errorRecords {
		if record == nil || record.PayeeUserID == nil || record.MatchStatus != domain.AssetWorkbenchErrorMatchStatusMatched || strings.TrimSpace(record.DifficultyClass) == "" || record.ErrorCount <= 0 {
			continue
		}
		payeeID := *record.PayeeUserID
		profile, appErr := s.settlementReportProfile(ctx, payeeID, profiles)
		if appErr != nil {
			return nil, appErr
		}
		row := rowsByPayee[payeeID]
		if row == nil {
			row = &SettlementPreviewRow{PayeeUserID: payeeID}
			rowsByPayee[payeeID] = row
		}
		normalPayrollRow := ensureNormalPayrollRow(payeeID)
		deduction, _, appErr := s.calculateQualityErrorDeductionCached(ctx, businessMonth, record, profile, deductionCache)
		if appErr != nil {
			return nil, appErr
		}
		row.ErrorCount += record.ErrorCount
		row.DeductionAmount += deduction
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount
		normalPayrollRow.ErrorCount += record.ErrorCount
		normalPayrollRow.DeductionAmount += deduction
		normalPayrollRow.NetAmount = normalPayrollRow.GrossAmount - normalPayrollRow.DeductionAmount + normalPayrollRow.WelfareAmount + normalPayrollRow.AdjustmentAmount
	}
	welfareLines, appErr := s.buildWelfareLines(ctx, businessMonth, items)
	if appErr != nil {
		return nil, appErr
	}
	for _, line := range welfareLines {
		row := rowsByPayee[line.PayeeUserID]
		if row == nil {
			row = &SettlementPreviewRow{PayeeUserID: line.PayeeUserID}
			rowsByPayee[line.PayeeUserID] = row
		}
		normalPayrollRow := ensureNormalPayrollRow(line.PayeeUserID)
		row.WelfareAmount += line.Amount
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount
		normalPayrollRow.WelfareAmount += line.Amount
		normalPayrollRow.NetAmount = normalPayrollRow.GrossAmount - normalPayrollRow.DeductionAmount + normalPayrollRow.WelfareAmount + normalPayrollRow.AdjustmentAmount
	}
	for _, supplement := range supplements {
		row := rowsByPayee[supplement.PayeeUserID]
		if row == nil {
			row = &SettlementPreviewRow{PayeeUserID: supplement.PayeeUserID}
			rowsByPayee[supplement.PayeeUserID] = row
		}
		supplementPayrollRow := ensureSupplementPayrollRow(supplement.PayeeUserID)
		row.PageCount += supplement.PageCount
		row.SupplementAmount += supplement.GrossAmount
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount
		supplementPayrollRow.ItemCount++
		supplementPayrollRow.PageCount += supplement.PageCount
		supplementPayrollRow.SupplementAmount += supplement.GrossAmount
		supplementPayrollRow.NetAmount = supplementPayrollRow.SupplementAmount
	}
	rows := make([]SettlementPreviewRow, 0, len(rowsByPayee))
	for _, row := range rowsByPayee {
		rows = append(rows, *row)
		total.ItemCount += row.ItemCount
		total.PageCount += row.PageCount
		total.GrossAmount += row.GrossAmount
		total.ErrorCount += row.ErrorCount
		total.DeductionAmount += row.DeductionAmount
		total.WelfareAmount += row.WelfareAmount
		total.SupplementAmount += row.SupplementAmount
		total.NetAmount += row.NetAmount
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].PayeeUserID < rows[j].PayeeUserID
	})
	payrollRows := make([]SettlementPayrollRow, 0, len(rowsByPayee)*2)
	for index := range rows {
		row := &rows[index]
		profile, appErr := s.settlementReportProfile(ctx, row.PayeeUserID, profiles)
		if appErr != nil {
			return nil, appErr
		}
		applySettlementPreviewProfile(row, profile)
		normalPayrollRow := normalPayrollRowsByPayee[row.PayeeUserID]
		if normalPayrollRow == nil {
			normalPayrollRow = &SettlementPayrollRow{
				PayeeUserID:   row.PayeeUserID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeNormalPiecework,
			}
		}
		supplementPayrollRow := supplementPayrollRowsByPayee[row.PayeeUserID]
		if supplementPayrollRow == nil {
			supplementPayrollRow = &SettlementPayrollRow{
				PayeeUserID:   row.PayeeUserID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeSupplementPiecework,
			}
		}
		applySettlementPayrollProfile(normalPayrollRow, profile)
		applySettlementPayrollProfile(supplementPayrollRow, profile)
		payrollRows = append(payrollRows, *normalPayrollRow, *supplementPayrollRow)
	}
	return &SettlementPreview{BusinessMonth: businessMonth, Rows: rows, Totals: total, PayrollRows: payrollRows}, nil
}

func (s *Service) buildSettlementReport(ctx context.Context, businessMonth string, items []*domain.AssetWorkbenchSubmissionItem, errorRecords []*domain.AssetWorkbenchErrorRecord, supplements []*domain.AssetWorkbenchSettlementSupplement) (*SettlementReport, *domain.AppError) {
	rowsByKey := map[string]*SettlementReportRow{}
	metricsByKey := map[string]map[string]*SettlementReportDifficultyMetric{}
	orderNosByKey := map[string]map[string]struct{}{}
	orderNosByKeyDifficulty := map[string]map[string]map[string]struct{}{}
	allDifficulties := map[string]struct{}{}
	totalOrderNos := map[string]struct{}{}
	totalMetrics := map[string]*SettlementReportDifficultyMetric{}
	totalOrderNosByDifficulty := map[string]map[string]struct{}{}
	profiles := map[int64]*domain.AssetWorkbenchProfile{}
	deductionCache := map[string]deductionRuleCacheEntry{}

	ensureRow := func(payeeID int64, rowType string) *SettlementReportRow {
		key := settlementReportRowKey(payeeID, rowType)
		row := rowsByKey[key]
		if row == nil {
			row = &SettlementReportRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       rowType,
			}
			rowsByKey[key] = row
		}
		return row
	}
	ensureMetric := func(key string, difficultyClass string) *SettlementReportDifficultyMetric {
		byDifficulty := metricsByKey[key]
		if byDifficulty == nil {
			byDifficulty = map[string]*SettlementReportDifficultyMetric{}
			metricsByKey[key] = byDifficulty
		}
		metric := byDifficulty[difficultyClass]
		if metric == nil {
			metric = &SettlementReportDifficultyMetric{DifficultyClass: difficultyClass}
			byDifficulty[difficultyClass] = metric
		}
		allDifficulties[difficultyClass] = struct{}{}
		return metric
	}
	ensureTotalMetric := func(difficultyClass string) *SettlementReportDifficultyMetric {
		metric := totalMetrics[difficultyClass]
		if metric == nil {
			metric = &SettlementReportDifficultyMetric{DifficultyClass: difficultyClass}
			totalMetrics[difficultyClass] = metric
		}
		allDifficulties[difficultyClass] = struct{}{}
		return metric
	}
	addOrderNo := func(bucket map[string]struct{}, orderNo string) {
		orderNo = strings.TrimSpace(orderNo)
		if orderNo == "" {
			return
		}
		bucket[orderNo] = struct{}{}
	}
	addOrderNoByKey := func(key string, orderNo string) {
		bucket := orderNosByKey[key]
		if bucket == nil {
			bucket = map[string]struct{}{}
			orderNosByKey[key] = bucket
		}
		addOrderNo(bucket, orderNo)
		addOrderNo(totalOrderNos, orderNo)
	}
	addOrderNoByDifficulty := func(key string, difficultyClass string, orderNo string) {
		orderNo = strings.TrimSpace(orderNo)
		if orderNo == "" {
			return
		}
		byDifficulty := orderNosByKeyDifficulty[key]
		if byDifficulty == nil {
			byDifficulty = map[string]map[string]struct{}{}
			orderNosByKeyDifficulty[key] = byDifficulty
		}
		bucket := byDifficulty[difficultyClass]
		if bucket == nil {
			bucket = map[string]struct{}{}
			byDifficulty[difficultyClass] = bucket
		}
		bucket[orderNo] = struct{}{}
		totalBucket := totalOrderNosByDifficulty[difficultyClass]
		if totalBucket == nil {
			totalBucket = map[string]struct{}{}
			totalOrderNosByDifficulty[difficultyClass] = totalBucket
		}
		totalBucket[orderNo] = struct{}{}
	}

	for _, item := range items {
		if item == nil {
			continue
		}
		rowType := domain.AssetWorkbenchPayrollRowTypeNormalPiecework
		key := settlementReportRowKey(item.PayeeUserID, rowType)
		row := ensureRow(item.PayeeUserID, rowType)
		difficultyClass := settlementReportDifficultyClass(item.DifficultyClass)
		metric := ensureMetric(key, difficultyClass)
		totalMetric := ensureTotalMetric(difficultyClass)
		row.ItemCount++
		row.PageCount += item.PageCount
		row.GrossAmount += item.GrossAmount
		if strings.TrimSpace(row.JobGrade) == "" {
			row.JobGrade = strings.TrimSpace(item.JobGradeSnapshot)
		}
		setSettlementReportCreatedDate(row, item.SubmittedAt, s.loc)
		metric.ItemCount++
		metric.PageCount += item.PageCount
		metric.GrossAmount += item.GrossAmount
		totalMetric.ItemCount++
		totalMetric.PageCount += item.PageCount
		totalMetric.GrossAmount += item.GrossAmount
		addOrderNoByKey(key, item.OrderNo)
		addOrderNoByDifficulty(key, difficultyClass, item.OrderNo)
		errorCount := matchedErrorCount(errorRecords, item)
		row.ErrorCount += errorCount
		metric.ErrorCount += errorCount
		totalMetric.ErrorCount += errorCount
		if errorCount > 0 {
			deduction, _, appErr := s.calculateDeductionCached(ctx, item, errorCount, deductionCache)
			if appErr != nil {
				return nil, appErr
			}
			row.DeductionAmount += deduction
			metric.DeductionAmount += deduction
			totalMetric.DeductionAmount += deduction
		}
	}
	for _, record := range errorRecords {
		if record == nil || record.PayeeUserID == nil || record.MatchStatus != domain.AssetWorkbenchErrorMatchStatusMatched || strings.TrimSpace(record.DifficultyClass) == "" || record.ErrorCount <= 0 {
			continue
		}
		payeeID := *record.PayeeUserID
		rowType := domain.AssetWorkbenchPayrollRowTypeNormalPiecework
		key := settlementReportRowKey(payeeID, rowType)
		row := ensureRow(payeeID, rowType)
		difficultyClass := settlementReportDifficultyClass(record.DifficultyClass)
		metric := ensureMetric(key, difficultyClass)
		totalMetric := ensureTotalMetric(difficultyClass)
		profile, appErr := s.settlementReportProfile(ctx, payeeID, profiles)
		if appErr != nil {
			return nil, appErr
		}
		if strings.TrimSpace(row.JobGrade) == "" && profile != nil {
			row.JobGrade = strings.TrimSpace(profile.JobGrade)
		}
		setSettlementReportCreatedDate(row, qualityErrorRecordAsOf(record, businessMonth, s.loc), s.loc)
		row.ErrorCount += record.ErrorCount
		metric.ErrorCount += record.ErrorCount
		totalMetric.ErrorCount += record.ErrorCount
		addOrderNoByKey(key, record.OrderNo)
		addOrderNoByDifficulty(key, difficultyClass, record.OrderNo)
		deduction, _, appErr := s.calculateQualityErrorDeductionCached(ctx, businessMonth, record, profile, deductionCache)
		if appErr != nil {
			return nil, appErr
		}
		row.DeductionAmount += deduction
		metric.DeductionAmount += deduction
		totalMetric.DeductionAmount += deduction
	}

	welfareLines, appErr := s.buildWelfareLines(ctx, businessMonth, items)
	if appErr != nil {
		return nil, appErr
	}
	for _, line := range welfareLines {
		row := ensureRow(line.PayeeUserID, domain.AssetWorkbenchPayrollRowTypeNormalPiecework)
		row.WelfareAmount += line.Amount
	}

	for _, supplement := range supplements {
		if supplement == nil {
			continue
		}
		rowType := domain.AssetWorkbenchPayrollRowTypeSupplementPiecework
		key := settlementReportRowKey(supplement.PayeeUserID, rowType)
		row := ensureRow(supplement.PayeeUserID, rowType)
		difficultyClass := settlementReportDifficultyClass(supplement.DifficultyClass)
		metric := ensureMetric(key, difficultyClass)
		totalMetric := ensureTotalMetric(difficultyClass)
		row.ItemCount++
		row.PageCount += supplement.PageCount
		row.SupplementAmount += supplement.GrossAmount
		setSettlementReportCreatedDate(row, supplement.CreatedAt, s.loc)
		metric.ItemCount++
		metric.PageCount += supplement.PageCount
		metric.GrossAmount += supplement.GrossAmount
		totalMetric.ItemCount++
		totalMetric.PageCount += supplement.PageCount
		totalMetric.GrossAmount += supplement.GrossAmount
		addOrderNoByKey(key, supplement.OrderNo)
		addOrderNoByDifficulty(key, difficultyClass, supplement.OrderNo)
	}

	difficultyRanks, appErr := s.settlementReportDifficultyRanks(ctx)
	if appErr != nil {
		return nil, appErr
	}
	difficultyClasses := settlementReportDifficultyClasses(allDifficulties, difficultyRanks)
	rows := make([]SettlementReportRow, 0, len(rowsByKey))
	total := SettlementReportRow{
		BusinessMonth: businessMonth,
		RowType:       "total",
		CreatorName:   "Total",
	}
	for key, row := range rowsByKey {
		row.OrderCount = len(orderNosByKey[key])
		row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount
		profile, appErr := s.settlementReportProfile(ctx, row.PayeeUserID, profiles)
		if appErr != nil {
			return nil, appErr
		}
		row.CreatorName = settlementReportCreatorName(row.PayeeUserID, profile)
		if strings.TrimSpace(row.JobGrade) == "" && profile != nil {
			row.JobGrade = strings.TrimSpace(profile.JobGrade)
		}
		total.ItemCount += row.ItemCount
		total.PageCount += row.PageCount
		total.GrossAmount += row.GrossAmount
		total.ErrorCount += row.ErrorCount
		total.DeductionAmount += row.DeductionAmount
		total.WelfareAmount += row.WelfareAmount
		total.SupplementAmount += row.SupplementAmount
		total.NetAmount += row.NetAmount
		rows = append(rows, *row)
	}
	total.OrderCount = len(totalOrderNos)
	total.ErrorRate = safeReportRatio(total.ErrorCount, total.PageCount)
	total.PageCountShare = boolReportShare(total.PageCount > 0)
	total.ErrorCountShare = boolReportShare(total.ErrorCount > 0)
	total.MonthAmountShare = boolReportShare(total.NetAmount != 0)
	total.DifficultyMetrics = materializeSettlementReportMetrics(totalMetrics, totalOrderNosByDifficulty, difficultyClasses, total.PageCount, total.ErrorCount, total.PageCount)

	for index := range rows {
		key := settlementReportRowKey(rows[index].PayeeUserID, rows[index].RowType)
		rows[index].ErrorRate = safeReportRatio(rows[index].ErrorCount, rows[index].PageCount)
		rows[index].PageCountShare = safeReportRatio(rows[index].PageCount, total.PageCount)
		rows[index].ErrorCountShare = safeReportRatio(rows[index].ErrorCount, total.ErrorCount)
		rows[index].MonthAmountShare = safeReportAmountRatio(rows[index].NetAmount, total.NetAmount)
		rows[index].DifficultyMetrics = materializeSettlementReportMetrics(metricsByKey[key], orderNosByKeyDifficulty[key], difficultyClasses, rows[index].PageCount, rows[index].ErrorCount, total.PageCount)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RowType != rows[j].RowType {
			return settlementReportRowRank(rows[i].RowType) < settlementReportRowRank(rows[j].RowType)
		}
		if rows[i].CreatorName != rows[j].CreatorName {
			return rows[i].CreatorName < rows[j].CreatorName
		}
		return rows[i].PayeeUserID < rows[j].PayeeUserID
	})

	return &SettlementReport{
		BusinessMonth:      businessMonth,
		DifficultyClasses:  difficultyClasses,
		Rows:               rows,
		Totals:             total,
		GeneratedAt:        s.nowFn().In(s.loc),
		OrderCountPolicy:   "distinct_non_empty_order_no",
		SettlementDataMode: "unconfirmed_settleable_items_and_approved_supplements",
	}, nil
}

func (s *Service) buildSettlementPayrollRowsFromItems(ctx context.Context, businessMonth string, items []*domain.AssetWorkbenchSettlementItem) ([]SettlementPayrollRow, *domain.AppError) {
	rows := buildSettlementPayrollRowsFromItems(businessMonth, items)
	profiles := map[int64]*domain.AssetWorkbenchProfile{}
	for index := range rows {
		profile, appErr := s.settlementReportProfile(ctx, rows[index].PayeeUserID, profiles)
		if appErr != nil {
			return nil, appErr
		}
		applySettlementPayrollProfile(&rows[index], profile)
	}
	return rows, nil
}

func buildSettlementPayrollRowsFromItems(businessMonth string, items []*domain.AssetWorkbenchSettlementItem) []SettlementPayrollRow {
	normalRowsByPayee := map[int64]*SettlementPayrollRow{}
	supplementRowsByPayee := map[int64]*SettlementPayrollRow{}
	payees := map[int64]struct{}{}
	ensureNormal := func(payeeID int64) *SettlementPayrollRow {
		payees[payeeID] = struct{}{}
		row := normalRowsByPayee[payeeID]
		if row == nil {
			row = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeNormalPiecework,
			}
			normalRowsByPayee[payeeID] = row
		}
		return row
	}
	ensureSupplement := func(payeeID int64) *SettlementPayrollRow {
		payees[payeeID] = struct{}{}
		row := supplementRowsByPayee[payeeID]
		if row == nil {
			row = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeSupplementPiecework,
			}
			supplementRowsByPayee[payeeID] = row
		}
		return row
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		switch item.ItemType {
		case domain.AssetWorkbenchItemTypeGrossPiecework:
			row := ensureNormal(item.PayeeUserID)
			row.ItemCount++
			row.PageCount += int(item.Quantity)
			row.GrossAmount += item.Amount
			row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.AdjustmentAmount
		case domain.AssetWorkbenchItemTypeErrorDeduction:
			row := ensureNormal(item.PayeeUserID)
			row.ErrorCount += int(item.Quantity)
			row.DeductionAmount += item.Amount
			row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.AdjustmentAmount
		case domain.AssetWorkbenchItemTypeWelfare:
			row := ensureNormal(item.PayeeUserID)
			row.WelfareAmount += item.Amount
			row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.AdjustmentAmount
		case domain.AssetWorkbenchItemTypeSupplement:
			row := ensureSupplement(item.PayeeUserID)
			row.ItemCount++
			row.PageCount += int(item.Quantity)
			row.SupplementAmount += item.Amount
			row.NetAmount = row.SupplementAmount
		case domain.AssetWorkbenchItemTypeAdjustment, domain.AssetWorkbenchItemTypeReversal:
			row := ensureNormal(item.PayeeUserID)
			signedAmount := item.Amount
			if item.Direction == "debit" {
				signedAmount = -item.Amount
			}
			row.AdjustmentAmount += signedAmount
			row.NetAmount = row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.AdjustmentAmount
		default:
			payees[item.PayeeUserID] = struct{}{}
		}
	}
	payeeIDs := make([]int64, 0, len(payees))
	for payeeID := range payees {
		payeeIDs = append(payeeIDs, payeeID)
	}
	sort.Slice(payeeIDs, func(i, j int) bool {
		return payeeIDs[i] < payeeIDs[j]
	})
	rows := make([]SettlementPayrollRow, 0, len(payeeIDs)*2)
	for _, payeeID := range payeeIDs {
		normal := normalRowsByPayee[payeeID]
		if normal == nil {
			normal = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeNormalPiecework,
			}
		}
		supplement := supplementRowsByPayee[payeeID]
		if supplement == nil {
			supplement = &SettlementPayrollRow{
				PayeeUserID:   payeeID,
				BusinessMonth: businessMonth,
				RowType:       domain.AssetWorkbenchPayrollRowTypeSupplementPiecework,
			}
		}
		rows = append(rows, *normal, *supplement)
	}
	return rows
}

func applySettlementPreviewProfile(row *SettlementPreviewRow, profile *domain.AssetWorkbenchProfile) {
	if row == nil || profile == nil {
		return
	}
	row.PayeeName = strings.TrimSpace(profile.RealName)
	row.WorkerType = strings.TrimSpace(profile.WorkerType)
}

func applySettlementPayrollProfile(row *SettlementPayrollRow, profile *domain.AssetWorkbenchProfile) {
	if row == nil || profile == nil {
		return
	}
	row.PayeeName = strings.TrimSpace(profile.RealName)
	row.WorkerType = strings.TrimSpace(profile.WorkerType)
}

func (s *Service) settlementReportProfile(ctx context.Context, payeeUserID int64, cache map[int64]*domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, *domain.AppError) {
	if profile, ok := cache[payeeUserID]; ok {
		return profile, nil
	}
	if s.repo == nil {
		cache[payeeUserID] = nil
		return nil, nil
	}
	profile, err := s.repo.GetProfileByUserID(ctx, payeeUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cache[payeeUserID] = nil
			return nil, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load settlement report profile.", err.Error())
	}
	cache[payeeUserID] = profile
	return profile, nil
}

func settlementReportRowKey(payeeID int64, rowType string) string {
	return fmt.Sprintf("%d:%s", payeeID, strings.TrimSpace(rowType))
}

func settlementReportCreatorName(payeeID int64, profile *domain.AssetWorkbenchProfile) string {
	if profile != nil && strings.TrimSpace(profile.RealName) != "" {
		return strings.TrimSpace(profile.RealName)
	}
	return fmt.Sprintf("User %d", payeeID)
}

func settlementReportDifficultyClass(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unclassified"
	}
	return value
}

func (s *Service) settlementReportDifficultyRanks(ctx context.Context) (map[string]int, *domain.AppError) {
	items, err := s.repo.ListDifficultyClasses(ctx, repo.AssetWorkbenchDifficultyClassFilter{})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench difficulty classes.", err.Error())
	}
	ranks := map[string]int{
		"unclassified": 1_000_000,
	}
	for index, item := range items {
		if item == nil {
			continue
		}
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		rank := item.SortOrder
		if rank <= 0 {
			rank = (index + 1) * 100
		}
		ranks[code] = rank
	}
	return ranks, nil
}

func settlementReportDifficultyClasses(values map[string]struct{}, ranks map[string]int) []string {
	classes := make([]string, 0, len(values))
	for value := range values {
		classes = append(classes, value)
	}
	sort.Slice(classes, func(i, j int) bool {
		leftRank := settlementReportDifficultyRank(classes[i], ranks)
		rightRank := settlementReportDifficultyRank(classes[j], ranks)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return classes[i] < classes[j]
	})
	return classes
}

func settlementReportDifficultyRank(value string, ranks map[string]int) int {
	value = strings.TrimSpace(value)
	if rank, ok := ranks[value]; ok {
		return rank
	}
	if value == "unclassified" {
		return 1_000_000
	}
	return 100_000
}

func settlementReportRowRank(rowType string) int {
	switch rowType {
	case domain.AssetWorkbenchPayrollRowTypeNormalPiecework:
		return 10
	case domain.AssetWorkbenchPayrollRowTypeSupplementPiecework:
		return 20
	default:
		return 100
	}
}

func setSettlementReportCreatedDate(row *SettlementReportRow, value time.Time, loc *time.Location) {
	if row == nil || value.IsZero() {
		return
	}
	if loc == nil {
		loc = time.UTC
	}
	date := value.In(loc).Format("2006-01-02")
	if row.CreatedDate == "" || date < row.CreatedDate {
		row.CreatedDate = date
	}
	if row.CreatedDateEnd == "" || date > row.CreatedDateEnd {
		row.CreatedDateEnd = date
	}
}

func materializeSettlementReportMetrics(metrics map[string]*SettlementReportDifficultyMetric, orderNos map[string]map[string]struct{}, classes []string, rowPageCount int, rowErrorCount int, totalPageCount int) []SettlementReportDifficultyMetric {
	rows := make([]SettlementReportDifficultyMetric, 0, len(metrics))
	for _, difficultyClass := range classes {
		metric := metrics[difficultyClass]
		if metric == nil {
			continue
		}
		row := *metric
		row.OrderCount = len(orderNos[difficultyClass])
		row.ErrorRate = safeReportRatio(row.ErrorCount, row.PageCount)
		row.PageCountShare = safeReportRatio(row.PageCount, rowPageCount)
		row.ErrorCountShare = safeReportRatio(row.ErrorCount, rowErrorCount)
		row.MonthPageCountShare = safeReportRatio(row.PageCount, totalPageCount)
		rows = append(rows, row)
	}
	return rows
}

func safeReportRatio(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func safeReportAmountRatio(numerator float64, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func boolReportShare(enabled bool) float64 {
	if enabled {
		return 1
	}
	return 0
}

func (s *Service) buildSettlementSupplementDuplicateHint(ctx context.Context, supplement *domain.AssetWorkbenchSettlementSupplement) (json.RawMessage, *domain.AppError) {
	if supplement == nil {
		return nil, nil
	}
	orderNo := strings.TrimSpace(supplement.OrderNo)
	if orderNo == "" || supplement.PayeeUserID <= 0 || strings.TrimSpace(supplement.BusinessMonth) == "" {
		return nil, nil
	}
	submissionItems, err := s.repo.ListSubmissionItemsByMonth(ctx, supplement.BusinessMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to check supplement duplicate submission items.", err.Error())
	}
	submissionItemIDs := make([]int64, 0)
	for _, item := range submissionItems {
		if item == nil || item.PayeeUserID != supplement.PayeeUserID || strings.TrimSpace(item.OrderNo) != orderNo {
			continue
		}
		if item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		submissionItemIDs = append(submissionItemIDs, item.ID)
	}
	payeeID := supplement.PayeeUserID
	existingSupplements, _, err := s.repo.ListSettlementSupplements(ctx, repo.AssetWorkbenchSettlementSupplementFilter{
		PayeeUserID:   &payeeID,
		BusinessMonth: supplement.BusinessMonth,
		OrderNo:       orderNo,
		Page:          1,
		PageSize:      100,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to check supplement duplicate supplements.", err.Error())
	}
	supplementIDs := make([]int64, 0, len(existingSupplements))
	for _, item := range existingSupplements {
		if item == nil {
			continue
		}
		supplementIDs = append(supplementIDs, item.ID)
	}
	return mustJSON(map[string]interface{}{
		"has_duplicates":      len(submissionItemIDs) > 0 || len(supplementIDs) > 0,
		"submission_item_ids": submissionItemIDs,
		"supplement_ids":      supplementIDs,
		"order_no":            orderNo,
		"supplement_date":     strings.TrimSpace(supplement.SupplementDate),
		"business_month":      supplement.BusinessMonth,
		"payee_user_id":       supplement.PayeeUserID,
	}), nil
}

func (s *Service) ensureSupplementPermissionOpen(ctx context.Context, payeeUserID int64, businessMonth string) *domain.AppError {
	permission, err := s.repo.GetSupplementPermission(ctx, payeeUserID, businessMonth)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement upload is not open for this payee and business month.", map[string]interface{}{
				"payee_user_id":  payeeUserID,
				"business_month": businessMonth,
			})
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check supplement permission.", err.Error())
	}
	if permission == nil || !permission.Enabled {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement upload is not open for this payee and business month.", map[string]interface{}{
			"payee_user_id":  payeeUserID,
			"business_month": businessMonth,
		})
	}
	return nil
}

func (s *Service) ensureSupplementPermissionOpenForUpdate(ctx context.Context, tx repo.Tx, payeeUserID int64, businessMonth string) *domain.AppError {
	permission, err := s.repo.GetSupplementPermissionForUpdate(ctx, tx, payeeUserID, businessMonth)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement upload was closed before submission completed.", map[string]interface{}{
				"payee_user_id":  payeeUserID,
				"business_month": businessMonth,
			})
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to lock supplement permission.", err.Error())
	}
	if permission == nil || !permission.Enabled {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Supplement upload was closed before submission completed.", map[string]interface{}{
			"payee_user_id":  payeeUserID,
			"business_month": businessMonth,
		})
	}
	return nil
}

func (s *Service) buildWelfareLines(ctx context.Context, businessMonth string, items []*domain.AssetWorkbenchSubmissionItem) ([]welfareSettlementLine, *domain.AppError) {
	if len(items) == 0 {
		return nil, nil
	}
	asOf, err := time.ParseInLocation("2006-01", businessMonth, s.loc)
	if err != nil {
		asOf = s.nowFn().In(s.loc)
	}
	type payeeGrade struct {
		workerType string
		jobGrade   string
	}
	byPayee := map[int64]payeeGrade{}
	for _, item := range items {
		if _, ok := byPayee[item.PayeeUserID]; ok {
			continue
		}
		byPayee[item.PayeeUserID] = payeeGrade{
			workerType: item.WorkerTypeSnapshot,
			jobGrade:   item.JobGradeSnapshot,
		}
	}
	lines := []welfareSettlementLine{}
	for payeeID, grade := range byPayee {
		if strings.TrimSpace(grade.workerType) == "" || strings.TrimSpace(grade.jobGrade) == "" {
			continue
		}
		rules, err := s.repo.FindActiveWelfareRules(ctx, grade.workerType, grade.jobGrade, asOf)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to match welfare rules.", err.Error())
		}
		for _, rule := range rules {
			if rule.Amount == 0 {
				continue
			}
			lines = append(lines, welfareSettlementLine{
				PayeeUserID:   payeeID,
				RuleID:        rule.ID,
				RuleName:      rule.RuleName,
				BusinessMonth: businessMonth,
				Amount:        rule.Amount,
				Snapshot: mustJSON(map[string]interface{}{
					"welfare_rule_id": rule.ID,
					"rule_name":       rule.RuleName,
					"rule_type":       rule.RuleType,
					"worker_type":     rule.WorkerType,
					"job_grade":       rule.JobGrade,
					"amount":          rule.Amount,
					"business_month":  businessMonth,
				}),
			})
		}
	}
	return lines, nil
}

func (s *Service) calculateDeduction(ctx context.Context, item *domain.AssetWorkbenchSubmissionItem, errorCount int) (float64, json.RawMessage, *domain.AppError) {
	return s.calculateDeductionCached(ctx, item, errorCount, nil)
}

type deductionRuleCacheEntry struct {
	rule    *domain.AssetWorkbenchDeductionRule
	missing bool
}

func (s *Service) calculateDeductionCached(ctx context.Context, item *domain.AssetWorkbenchSubmissionItem, errorCount int, cache map[string]deductionRuleCacheEntry) (float64, json.RawMessage, *domain.AppError) {
	if errorCount <= 0 {
		return 0, nil, nil
	}
	asOf := item.SubmittedAt.In(s.loc)
	var entry deductionRuleCacheEntry
	var ok bool
	if cache != nil {
		entry, ok = cache[deductionRuleCacheKey(item, asOf)]
	}
	if !ok {
		rule, err := s.repo.FindActiveDeductionRule(ctx, item.WorkerTypeSnapshot, item.JobGradeSnapshot, item.DifficultyClass, asOf)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				entry = deductionRuleCacheEntry{missing: true}
			} else {
				return 0, nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to match deduction rule.", err.Error())
			}
		} else {
			entry = deductionRuleCacheEntry{rule: rule}
		}
		if cache != nil {
			cache[deductionRuleCacheKey(item, asOf)] = entry
		}
	}
	if entry.missing || entry.rule == nil {
		return 0, mustJSON(map[string]interface{}{
			"status":           "deduction_rule_missing",
			"worker_type":      item.WorkerTypeSnapshot,
			"job_grade":        item.JobGradeSnapshot,
			"difficulty_class": item.DifficultyClass,
			"error_count":      errorCount,
		}), nil
	}
	rule := entry.rule
	amount := rule.DeductionAmount * float64(errorCount)
	return amount, mustJSON(map[string]interface{}{
		"status":            "matched",
		"deduction_rule_id": rule.ID,
		"worker_type":       rule.WorkerType,
		"job_grade":         rule.JobGrade,
		"difficulty_class":  rule.DifficultyClass,
		"deduction_amount":  rule.DeductionAmount,
		"error_count":       errorCount,
		"calculated_at":     s.nowFn().UTC().Format(time.RFC3339),
	}), nil
}

func deductionRuleCacheKey(item *domain.AssetWorkbenchSubmissionItem, asOf time.Time) string {
	if item == nil {
		return "::::" + asOf.Format("2006-01-02")
	}
	return strings.Join([]string{
		strings.TrimSpace(item.WorkerTypeSnapshot),
		strings.TrimSpace(item.JobGradeSnapshot),
		strings.TrimSpace(item.DifficultyClass),
		asOf.Format("2006-01-02"),
	}, "\x1f")
}

func matchedErrorCount(records []*domain.AssetWorkbenchErrorRecord, item *domain.AssetWorkbenchSubmissionItem) int {
	count := 0
	for _, record := range records {
		if strings.TrimSpace(record.DifficultyClass) != "" {
			continue
		}
		matchStatus := strings.TrimSpace(record.MatchStatus)
		if matchStatus != "" && matchStatus != domain.AssetWorkbenchErrorMatchStatusMatched {
			continue
		}
		if record.SubmissionItemID != nil && *record.SubmissionItemID == item.ID {
			count += record.ErrorCount
			continue
		}
		if matchStatus == domain.AssetWorkbenchErrorMatchStatusMatched {
			continue
		}
		if record.OrderNo != item.OrderNo {
			continue
		}
		if record.PayeeUserID != nil && *record.PayeeUserID != item.PayeeUserID {
			continue
		}
		count += record.ErrorCount
	}
	return count
}

func (s *Service) calculateQualityErrorDeductionCached(ctx context.Context, businessMonth string, record *domain.AssetWorkbenchErrorRecord, profile *domain.AssetWorkbenchProfile, cache map[string]deductionRuleCacheEntry) (float64, json.RawMessage, *domain.AppError) {
	if record == nil || profile == nil || record.ErrorCount <= 0 {
		return 0, nil, nil
	}
	asOf := qualityErrorRecordAsOf(record, businessMonth, s.loc)
	item := &domain.AssetWorkbenchSubmissionItem{
		PayeeUserID:        derefInt64(record.PayeeUserID),
		OrderNo:            record.OrderNo,
		DifficultyClass:    strings.TrimSpace(record.DifficultyClass),
		SubmittedAt:        asOf,
		WorkerTypeSnapshot: strings.TrimSpace(profile.WorkerType),
		JobGradeSnapshot:   strings.TrimSpace(profile.JobGrade),
		PricingStatus:      domain.AssetWorkbenchPricingStatusPriced,
		QCStatus:           domain.AssetWorkbenchSubmissionStatusChecked,
		SettlementStatus:   domain.AssetWorkbenchSettlementStatusUnsettled,
	}
	amount, snapshot, appErr := s.calculateDeductionCached(ctx, item, record.ErrorCount, cache)
	if appErr != nil {
		return 0, nil, appErr
	}
	return amount, enrichQualityErrorDeductionSnapshot(snapshot, record, amount, asOf), nil
}

func enrichQualityErrorDeductionSnapshot(snapshot json.RawMessage, record *domain.AssetWorkbenchErrorRecord, amount float64, asOf time.Time) json.RawMessage {
	payload := map[string]interface{}{}
	if len(snapshot) > 0 && json.Valid(snapshot) {
		_ = json.Unmarshal(snapshot, &payload)
	}
	payload["source"] = "quality_error_import"
	payload["error_record_id"] = record.ID
	payload["order_no"] = record.OrderNo
	payload["difficulty_class"] = record.DifficultyClass
	payload["error_count"] = record.ErrorCount
	payload["calculated_amount"] = amount
	payload["rule_as_of"] = asOf.Format("2006-01-02")
	if len(record.RawPayload) > 0 && json.Valid(record.RawPayload) {
		var raw interface{}
		if err := json.Unmarshal(record.RawPayload, &raw); err == nil {
			payload["import_payload"] = raw
		}
	}
	return mustJSON(payload)
}

func qualityErrorRecordAsOf(record *domain.AssetWorkbenchErrorRecord, businessMonth string, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	if record != nil && record.OccurredDate != nil && !record.OccurredDate.IsZero() {
		return record.OccurredDate.In(loc)
	}
	if t, err := time.ParseInLocation("2006-01", strings.TrimSpace(businessMonth), loc); err == nil {
		return t
	}
	return time.Now().In(loc)
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func matchImportedErrorRecord(items []*domain.AssetWorkbenchSubmissionItem, input ImportErrorRecordInput) errorRecordImportMatch {
	orderNo := strings.TrimSpace(input.OrderNo)
	if orderNo == "" {
		return errorRecordImportMatch{Status: domain.AssetWorkbenchErrorMatchStatusUnmatched}
	}
	candidates := make([]*domain.AssetWorkbenchSubmissionItem, 0, 2)
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.OrderNo) != orderNo {
			continue
		}
		if item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		if input.PayeeUserID != nil && item.PayeeUserID != *input.PayeeUserID {
			continue
		}
		candidates = append(candidates, item)
	}
	switch len(candidates) {
	case 0:
		return errorRecordImportMatch{Status: domain.AssetWorkbenchErrorMatchStatusUnmatched, Reason: "order_no_not_found"}
	case 1:
		itemID := candidates[0].ID
		payeeUserID := candidates[0].PayeeUserID
		return errorRecordImportMatch{
			Status:           domain.AssetWorkbenchErrorMatchStatusMatched,
			SubmissionItemID: &itemID,
			CandidateItemIDs: []int64{itemID},
			PayeeUserID:      &payeeUserID,
		}
	default:
		ids := make([]int64, 0, len(candidates))
		for _, item := range candidates {
			ids = append(ids, item.ID)
		}
		return errorRecordImportMatch{
			Status:           domain.AssetWorkbenchErrorMatchStatusAmbiguous,
			CandidateItemIDs: ids,
			Reason:           "order_no_ambiguous",
		}
	}
}

func isQualityErrorImportInput(input ImportErrorRecordInput) bool {
	return strings.TrimSpace(input.DifficultyClass) != "" ||
		strings.TrimSpace(input.PayeeName) != "" ||
		strings.TrimSpace(input.IssueDescription) != "" ||
		strings.TrimSpace(input.SourceType) != "" ||
		strings.TrimSpace(input.HandlingMethod) != "" ||
		strings.TrimSpace(input.ReporterName) != "" ||
		strings.TrimSpace(input.Remark) != "" ||
		strings.TrimSpace(input.OccurredDate) != "" ||
		len(input.RawPayload) > 0
}

func (s *Service) errorImportDifficultyIndex(ctx context.Context) (map[string]string, *domain.AppError) {
	items, err := s.repo.ListDifficultyClasses(ctx, repo.AssetWorkbenchDifficultyClassFilter{})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench difficulty classes.", err.Error())
	}
	index := map[string]string{}
	for _, item := range items {
		if item == nil || !item.Enabled || strings.TrimSpace(item.Code) == "" {
			continue
		}
		code := strings.TrimSpace(item.Code)
		index[normalizeErrorImportLookupKey(code)] = code
		if name := strings.TrimSpace(item.Name); name != "" {
			index[normalizeErrorImportLookupKey(name)] = code
		}
	}
	return index, nil
}

func (s *Service) matchQualityErrorRecord(ctx context.Context, input ImportErrorRecordInput, difficultyIndex map[string]string) (errorRecordImportMatch, *domain.AppError) {
	match := errorRecordImportMatch{Status: domain.AssetWorkbenchErrorMatchStatusMatched}
	difficulty := normalizeErrorImportDifficulty(input.DifficultyClass, difficultyIndex)
	if difficulty == "" {
		match.Status = domain.AssetWorkbenchErrorMatchStatusUnmatched
		match.Reason = "difficulty_class_not_found"
		return match, nil
	}
	match.DifficultyClass = difficulty
	occurredDate, appErr := parseErrorImportDate(input.OccurredDate, s.loc)
	if appErr != nil {
		return match, appErr
	}
	match.OccurredDate = occurredDate
	payeeUserID, candidates, reason, appErr := s.matchErrorImportPayee(ctx, input)
	if appErr != nil {
		return match, appErr
	}
	match.CandidateUserIDs = candidates
	if payeeUserID == nil {
		if len(candidates) > 1 {
			match.Status = domain.AssetWorkbenchErrorMatchStatusAmbiguous
		} else {
			match.Status = domain.AssetWorkbenchErrorMatchStatusUnmatched
		}
		match.Reason = reason
		return match, nil
	}
	match.PayeeUserID = payeeUserID
	return match, nil
}

func (s *Service) matchErrorImportPayee(ctx context.Context, input ImportErrorRecordInput) (*int64, []int64, string, *domain.AppError) {
	if input.PayeeUserID != nil && *input.PayeeUserID > 0 {
		profile, err := s.repo.GetProfileByUserID(ctx, *input.PayeeUserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, "payee_user_id_not_found", nil
			}
			return nil, nil, "", domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench profile.", err.Error())
		}
		if strings.TrimSpace(input.PayeeName) != "" && normalizeHumanName(profile.RealName) != normalizeHumanName(input.PayeeName) {
			return nil, []int64{profile.UserID}, "payee_name_mismatch", nil
		}
		value := profile.UserID
		return &value, []int64{value}, "", nil
	}
	payeeName := strings.TrimSpace(input.PayeeName)
	if payeeName == "" {
		return nil, nil, "payee_name_required", nil
	}
	profiles, _, err := s.repo.ListProfiles(ctx, repo.AssetWorkbenchProfileFilter{Keyword: payeeName, Page: 1, PageSize: 500})
	if err != nil {
		return nil, nil, "", domain.NewAppError(domain.ErrCodeInternalError, "Failed to match asset workbench profile.", err.Error())
	}
	candidates := make([]int64, 0, 2)
	needle := normalizeHumanName(payeeName)
	for _, profile := range profiles {
		if profile == nil || normalizeHumanName(profile.RealName) != needle {
			continue
		}
		candidates = append(candidates, profile.UserID)
	}
	switch len(candidates) {
	case 0:
		return nil, nil, "payee_name_not_found", nil
	case 1:
		value := candidates[0]
		return &value, candidates, "", nil
	default:
		return nil, candidates, "payee_name_ambiguous", nil
	}
}

func normalizeErrorImportDifficulty(raw string, index map[string]string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if code, ok := index[normalizeErrorImportLookupKey(value)]; ok {
		return code
	}
	return ""
}

func normalizeErrorImportLookupKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func normalizeHumanName(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func errorRecordRawPayload(input ImportErrorRecordInput, match errorRecordImportMatch) json.RawMessage {
	raw := map[string]interface{}{
		"input":                input,
		"match_status":         match.Status,
		"match_reason":         match.Reason,
		"candidate_item_ids":   match.CandidateItemIDs,
		"candidate_user_ids":   match.CandidateUserIDs,
		"resolved_payee_id":    match.PayeeUserID,
		"resolved_difficulty":  match.DifficultyClass,
		"resolved_occurred_at": formatOptionalDate(match.OccurredDate),
	}
	if len(input.RawPayload) > 0 && json.Valid(input.RawPayload) {
		var payload interface{}
		if err := json.Unmarshal(input.RawPayload, &payload); err == nil {
			raw["source_row"] = payload
		}
	}
	return mustJSON(raw)
}

func (s *Service) resolveUploadDirectoryForSession(ctx context.Context, directoryID int64) (*domain.AssetWorkbenchUploadDirectory, *domain.AppError) {
	if directoryID > 0 {
		directory, err := s.repo.GetUploadDirectory(ctx, directoryID)
		if err != nil {
			return nil, mapRepoReadError(err, "Upload directory not found.", "Failed to load upload directory.")
		}
		if directory == nil || !directory.Enabled {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_directory_id is not enabled.", map[string]interface{}{"upload_directory_id": directoryID})
		}
		return directory, nil
	}
	enabled := true
	directories, err := s.repo.ListUploadDirectories(ctx, repo.AssetWorkbenchUploadDirectoryFilter{Enabled: &enabled})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list asset workbench upload directories.", err.Error())
	}
	if len(directories) > 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_directory_id is required.", nil)
	}
	return nil, nil
}

func (s *Service) buildObjectKey(now time.Time, sessionID, filename string, directory *domain.AssetWorkbenchUploadDirectory, relativePath string) string {
	clean := strings.TrimSpace(strings.Trim(relativePath, "/"))
	if clean == "" {
		clean = strings.TrimSpace(filepath.Base(filename))
	}
	if clean == "." || clean == string(filepath.Separator) || clean == "" {
		clean = "upload.bin"
	}
	base := strings.Trim(s.cfg.OSSPrefix, "/")
	if directory == nil || strings.TrimSpace(directory.OSSPrefix) == "" {
		return fmt.Sprintf("%s/uploads/%s/%s/%s", base, now.Format("2006/01"), sessionID, clean)
	}
	return fmt.Sprintf("%s/uploads/%s/%s/%s/%s", base, directory.OSSPrefix, now.Format("2006/01"), sessionID, clean)
}

func (s *Service) buildMovedFileObjectKey(now time.Time, file *domain.AssetWorkbenchSubmissionFile, directory *domain.AssetWorkbenchUploadDirectory) string {
	filename := "upload.bin"
	if file != nil {
		filename = firstNonEmpty(file.OriginalFilename, filepath.Base(file.ObjectKey))
	}
	clean := strings.TrimSpace(filepath.Base(filename))
	if clean == "." || clean == string(filepath.Separator) || clean == "" {
		clean = "upload.bin"
	}
	base := strings.Trim(s.cfg.OSSPrefix, "/")
	prefix := ""
	if directory != nil {
		prefix = strings.Trim(directory.OSSPrefix, "/")
	}
	if prefix == "" {
		return fmt.Sprintf("%s/uploads/%s/moved/%s-%s", base, now.Format("2006/01"), uuid.NewString(), clean)
	}
	return fmt.Sprintf("%s/uploads/%s/%s/moved/%s-%s", base, prefix, now.Format("2006/01"), uuid.NewString(), clean)
}

func normalizeUploadBatchID(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return uuid.NewString()
	}
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return -1
		}
	}, value)
	if value == "" {
		return uuid.NewString()
	}
	if len(value) > 64 {
		return value[:64]
	}
	return value
}

func normalizeUploadRelativePath(raw string, filename string) (string, *domain.AppError) {
	fallback := strings.TrimSpace(filepath.Base(filename))
	if fallback == "" || fallback == "." || fallback == string(filepath.Separator) {
		fallback = "upload.bin"
	}
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		value = fallback
	}
	value = strings.Trim(value, "/")
	if value == "" {
		value = fallback
	}
	if strings.Contains(value, "\x00") || strings.Contains(value, ":") {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "relative_path contains invalid characters.", nil)
	}
	clean := path.Clean(value)
	if clean == "." {
		clean = fallback
	}
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "relative_path cannot escape the upload folder.", nil)
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "relative_path contains invalid path segments.", nil)
		}
	}
	if len(clean) > 1024 {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "relative_path is too long.", map[string]int{"max_length": 1024})
	}
	return clean, nil
}

func normalizeUploadDirectoryPrefix(raw string) (string, *domain.AppError) {
	value := strings.Trim(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")), "/")
	if value == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_prefix is required.", nil)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_prefix cannot escape the asset workbench upload namespace.", nil)
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.Contains(part, "\x00") {
			return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "oss_prefix contains invalid path segments.", nil)
		}
	}
	return clean, nil
}

func normalizeUploadDirectoryDifficulty(raw string) (string, *domain.AppError) {
	return normalizeWorkbenchDifficultyCode(raw, false)
}

func normalizeUploadDirectoryFileTypes(values []string) ([]string, *domain.AppError) {
	if values == nil {
		return nil, nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if strings.Contains(value, "\x00") || strings.Contains(value, "\\") {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "allowed_file_types contains invalid file type.", nil)
		}
		if strings.HasPrefix(value, ".") {
			value = strings.TrimLeft(value, ".")
		}
		if strings.Contains(value, " ") || strings.Contains(value, ",") || strings.Count(value, "/") > 1 || value == "*" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "allowed_file_types contains invalid file type.", nil)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func uploadDirectoryAllowsFile(directory *domain.AssetWorkbenchUploadDirectory, filename string, mimeType string) bool {
	if directory == nil || len(directory.AllowedFileTypes) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	for _, allowed := range directory.AllowedFileTypes {
		allowed = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(allowed, ".")))
		if allowed == "" {
			continue
		}
		if ext != "" && allowed == ext {
			return true
		}
		if mimeType != "" && allowed == mimeType {
			return true
		}
		if strings.HasSuffix(allowed, "/*") && mimeType != "" && strings.HasPrefix(mimeType, strings.TrimSuffix(allowed, "*")) {
			return true
		}
	}
	return false
}

func boolValueDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func trimString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func normalizeServicePage(page int, pageSize int, defaultPageSize int, maxPageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if defaultPageSize <= 0 {
		defaultPageSize = 50
	}
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func clientMaterialMatchesQuery(item *domain.AssetWorkbenchClientMaterial, query string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		item.Title,
		item.Description,
		item.FilenameSnapshot,
		item.MimeTypeSnapshot,
		item.SourceLabel,
		item.ResourceID,
		item.SourceRef,
		item.ScopeSKUCode,
		item.SKUCode,
		item.PrimarySKUCode,
	}, " "))
	return strings.Contains(haystack, query)
}

func clientMaterialMatchesSKU(item *domain.AssetWorkbenchClientMaterial, sku string) bool {
	return strings.Contains(strings.ToLower(item.ScopeSKUCode), sku) ||
		strings.Contains(strings.ToLower(item.SKUCode), sku) ||
		strings.Contains(strings.ToLower(item.PrimarySKUCode), sku)
}

var (
	assetWorkbenchIDCardPattern = regexp.MustCompile(`^[0-9]{18}$`)
	externalMaterialCodePattern = regexp.MustCompile(`(?i)([A-Z]{1,8}[-_]?\d{3,}[A-Z0-9_-]*|\d{6}-\d{8,}|[A-Z0-9]{6,}[-_][A-Z0-9_-]{2,})`)
)

func groupMaterialAssets(items []*assetcenter.AssetDetail) []*MaterialGroupRow {
	byKey := map[string]*MaterialGroupRow{}
	order := []string{}
	for _, asset := range items {
		if asset == nil {
			continue
		}
		key := assetMaterialGroupKey(asset)
		if key == "" {
			key = fmt.Sprintf("system-asset:%d", asset.ID)
		}
		group := byKey[key]
		if group == nil {
			group = &MaterialGroupRow{
				GroupKey:   key,
				GroupCode:  materialGroupCodeFromKey(key),
				GroupType:  materialGroupTypeFromKey(key),
				Title:      materialGroupTitle(asset, key),
				SourceType: firstNonEmpty(asset.SourceType, string(domain.AssetResourceSourceSystem)),
			}
			byKey[key] = group
			order = append(order, key)
		}
		group.FileTotal++
		if len(group.PreviewFiles) < 5 {
			group.PreviewFiles = append(group.PreviewFiles, asset)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := byKey[order[i]]
		right := byKey[order[j]]
		if left.FileTotal == right.FileTotal {
			return left.GroupKey < right.GroupKey
		}
		return left.FileTotal > right.FileTotal
	})
	out := make([]*MaterialGroupRow, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

func assetMaterialGroupKey(asset *assetcenter.AssetDetail) string {
	if asset == nil {
		return ""
	}
	source := domain.NormalizeAssetResourceSource(asset.SourceType)
	if source == domain.AssetResourceSourceExternal {
		if code := inferExternalMaterialCode(asset.OriginPath); code != "" {
			return "ext-sku:" + code
		}
		parent := normalizeExternalMaterialParent(asset.OriginPath)
		if parent != "" {
			return "ext-dir:" + parent
		}
	}
	if code := firstNonEmpty(asset.ScopeSKUCode, asset.SKUCode, asset.PrimarySKUCode); code != "" {
		return "sku:" + strings.TrimSpace(code)
	}
	return fmt.Sprintf("system-asset:%d", asset.ID)
}

func materialGroupSearchKeyword(groupKey string) string {
	groupKey = strings.TrimSpace(groupKey)
	for _, prefix := range []string{"sku:", "ext-sku:", "ext-dir:", "system-asset:"} {
		if strings.HasPrefix(groupKey, prefix) {
			return strings.TrimPrefix(groupKey, prefix)
		}
	}
	return groupKey
}

func materialGroupCodeFromKey(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[idx+1:]
	}
	return key
}

func materialGroupTypeFromKey(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[:idx]
	}
	return "unknown"
}

func materialGroupTitle(asset *assetcenter.AssetDetail, key string) string {
	code := materialGroupCodeFromKey(key)
	switch materialGroupTypeFromKey(key) {
	case "ext-dir":
		return path.Base(strings.Trim(code, "/"))
	case "system-asset":
		return firstNonEmpty(asset.ProductName, asset.OriginalFilename, asset.FileName, code)
	default:
		return code
	}
}

func inferExternalMaterialCode(originPath string) string {
	parts := strings.Split(strings.ReplaceAll(originPath, "\\", "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if lower == "quark" || lower == "p1" || lower == "p2" || lower == "p3" || lower == "#recycle" || lower == "@eadir" {
			continue
		}
		match := externalMaterialCodePattern.FindString(part)
		if match != "" && !looksLikePlainDateCode(match) {
			return strings.ToUpper(match)
		}
	}
	return ""
}

func looksLikePlainDateCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return false
	}
	if _, err := time.Parse("20060102", value); err == nil {
		return true
	}
	return false
}

func normalizeExternalMaterialParent(originPath string) string {
	value := strings.Trim(strings.ReplaceAll(strings.TrimSpace(originPath), "\\", "/"), "/")
	if value == "" {
		return ""
	}
	parent := path.Dir(value)
	if parent == "." || parent == "/" {
		return value
	}
	return "/" + strings.Trim(parent, "/")
}

func filterVisibleWorkbenchMaterialAssets(items []*assetcenter.AssetDetail) []*assetcenter.AssetDetail {
	out := make([]*assetcenter.AssetDetail, 0, len(items))
	for _, item := range items {
		if assetWorkbenchMaterialAssetVisible(item) {
			out = append(out, rewriteWorkbenchVirtualQuarkAsset(item))
		}
	}
	return out
}

func normalizeAssetWorkbenchBusinessLane(value string) domain.TaskBusinessLane {
	lane := domain.TaskBusinessLane(strings.TrimSpace(value))
	if lane.Valid() {
		return lane
	}
	return ""
}

func filterVisibleWorkbenchMaterialFolders(folders []assetcenter.MaterialFolder) []assetcenter.MaterialFolder {
	out := make([]assetcenter.MaterialFolder, 0, len(folders))
	for _, folder := range folders {
		if assetWorkbenchOperationalMaterialPathVisible(folder.Path) {
			out = append(out, folder)
		}
	}
	return out
}

const (
	workbenchQuarkMaterialVirtualRoot = "/quark"
	workbenchQuarkMaterialActualBase  = "/quark/我的备份/来自：ASUS Administrator 电脑备份"
)

var workbenchQuarkMaterialVirtualFolders = []string{"电视投屏", "海报", "kt板", "闲置kt板"}

func assetWorkbenchMaterialAssetVisible(asset *assetcenter.AssetDetail) bool {
	if asset == nil {
		return false
	}
	if domain.NormalizeAssetResourceSource(asset.SourceType) != domain.AssetResourceSourceExternal {
		return true
	}
	return assetWorkbenchOperationalMaterialPathVisible(asset.OriginPath)
}

func assetWorkbenchOperationalMaterialPathVisible(raw string) bool {
	pathValue := normalizeWorkbenchMaterialPath(raw)
	if pathValue == "" {
		return true
	}
	parts := workbenchMaterialPathParts(pathValue)
	if len(parts) == 0 || !strings.EqualFold(parts[0], "quark") {
		return true
	}
	if workbenchIsVirtualQuarkRoot(pathValue) {
		return true
	}
	for _, virtual := range workbenchQuarkMaterialVirtualFolders {
		if pathValue == workbenchQuarkMaterialVirtualRoot+"/"+virtual || strings.HasPrefix(pathValue, workbenchQuarkMaterialVirtualRoot+"/"+virtual+"/") {
			return true
		}
		actualPrefix := workbenchQuarkMaterialActualBase + "/" + virtual
		if pathValue == actualPrefix || strings.HasPrefix(pathValue, actualPrefix+"/") {
			return true
		}
	}
	return false
}

func workbenchIsVirtualQuarkRoot(pathValue string) bool {
	return strings.EqualFold(normalizeWorkbenchMaterialPath(pathValue), workbenchQuarkMaterialVirtualRoot)
}

func workbenchMaterialActualPath(pathValue string) string {
	pathValue = normalizeWorkbenchMaterialPath(pathValue)
	for _, virtual := range workbenchQuarkMaterialVirtualFolders {
		virtualPrefix := workbenchQuarkMaterialVirtualRoot + "/" + virtual
		if pathValue == virtualPrefix {
			return workbenchQuarkMaterialActualBase + "/" + virtual
		}
		if strings.HasPrefix(pathValue, virtualPrefix+"/") {
			return workbenchQuarkMaterialActualBase + "/" + virtual + strings.TrimPrefix(pathValue, virtualPrefix)
		}
	}
	return pathValue
}

func workbenchMaterialVirtualPath(pathValue string) string {
	pathValue = normalizeWorkbenchMaterialPath(pathValue)
	for _, virtual := range workbenchQuarkMaterialVirtualFolders {
		actualPrefix := workbenchQuarkMaterialActualBase + "/" + virtual
		if pathValue == actualPrefix {
			return workbenchQuarkMaterialVirtualRoot + "/" + virtual
		}
		if strings.HasPrefix(pathValue, actualPrefix+"/") {
			return workbenchQuarkMaterialVirtualRoot + "/" + virtual + strings.TrimPrefix(pathValue, actualPrefix)
		}
	}
	return pathValue
}

func rewriteWorkbenchVirtualQuarkFolders(folders []assetcenter.MaterialFolder) []assetcenter.MaterialFolder {
	for i := range folders {
		folders[i].Path = workbenchMaterialVirtualPath(folders[i].Path)
		if strings.TrimSpace(folders[i].Name) == "" {
			folders[i].Name = path.Base(folders[i].Path)
		}
	}
	return folders
}

func rewriteWorkbenchVirtualQuarkAsset(asset *assetcenter.AssetDetail) *assetcenter.AssetDetail {
	if asset == nil {
		return nil
	}
	virtualPath := workbenchMaterialVirtualPath(asset.OriginPath)
	if virtualPath == asset.OriginPath {
		return asset
	}
	copy := *asset
	copy.OriginPath = virtualPath
	return &copy
}

func workbenchQuarkVirtualFolderName(pathValue string) (string, bool) {
	pathValue = normalizeWorkbenchMaterialPath(pathValue)
	for _, virtual := range workbenchQuarkMaterialVirtualFolders {
		if pathValue == workbenchQuarkMaterialVirtualRoot+"/"+virtual || strings.HasPrefix(pathValue, workbenchQuarkMaterialVirtualRoot+"/"+virtual+"/") ||
			pathValue == workbenchQuarkMaterialActualBase+"/"+virtual || strings.HasPrefix(pathValue, workbenchQuarkMaterialActualBase+"/"+virtual+"/") {
			return virtual, true
		}
	}
	return "", false
}

func workbenchQuarkVirtualFoldersSet() map[string]struct{} {
	out := make(map[string]struct{}, len(workbenchQuarkMaterialVirtualFolders))
	for _, folder := range workbenchQuarkMaterialVirtualFolders {
		out[strings.ToLower(folder)] = struct{}{}
	}
	return out
}

func assetWorkbenchOperationalMaterialRootName(pathValue string) string {
	if name, ok := workbenchQuarkVirtualFolderName(pathValue); ok {
		return name
	}
	parts := workbenchMaterialPathParts(pathValue)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func assetWorkbenchOperationalMaterialPathNameAllowed(name string) bool {
	_, ok := workbenchQuarkVirtualFoldersSet()[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func assetWorkbenchOperationalMaterialPathUnderAllowedRoot(pathValue string) bool {
	name := assetWorkbenchOperationalMaterialRootName(pathValue)
	if name == "" {
		return true
	}
	return assetWorkbenchOperationalMaterialPathNameAllowed(name)
}

func normalizeWorkbenchMaterialPath(raw string) string {
	parts := workbenchMaterialPathParts(raw)
	if len(parts) == 0 {
		return ""
	}
	return "/" + strings.Join(parts, "/")
}

func workbenchMaterialPathParts(raw string) []string {
	value := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	rawParts := strings.Split(value, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func (s *Service) systemDownloadSnapshot(ctx context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.systemDownloads == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset downloader is not configured.", nil)
	}
	info, appErr := s.systemDownloads.DownloadLatest(ctx, assetID)
	if appErr != nil {
		return nil, appErr
	}
	if info == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "System asset download info is empty.", nil)
	}
	return info, nil
}

type clientMaterialSourceRef struct {
	SourceType string
	SourceRef  string
	AssetID    int64
}

type clientMaterialSourceSnapshot struct {
	AssetID          int64
	SourceType       string
	SourceRef        string
	ResourceID       string
	SourceLabel      string
	Filename         string
	MimeType         string
	FileSize         int64
	ScopeSKUCode     string
	SKUCode          string
	PrimarySKUCode   string
	PreviewAvailable bool
}

func resolveClientMaterialSourceInput(assetID int64, sourceType, sourceRef, resourceID string) (clientMaterialSourceRef, *domain.AppError) {
	sourceType = strings.TrimSpace(strings.ToLower(sourceType))
	rawRef := firstNonEmpty(sourceRef, resourceID)
	normalized := domain.NormalizeAssetResourceSource(sourceType)
	if sourceType == "" || normalized == domain.AssetResourceSourceAll {
		if _, ok := domain.ParseExternalAssetResourceID(rawRef); ok {
			normalized = domain.AssetResourceSourceExternal
		} else {
			normalized = domain.AssetResourceSourceSystem
		}
	}
	switch normalized {
	case domain.AssetResourceSourceExternal:
		id, ok := domain.ParseExternalAssetResourceID(rawRef)
		if !ok && assetID > 0 {
			id = assetID
			ok = true
		}
		if !ok || id <= 0 {
			return clientMaterialSourceRef{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "external source_ref is required.", nil)
		}
		return clientMaterialSourceRef{
			SourceType: string(domain.AssetResourceSourceExternal),
			SourceRef:  domain.ExternalAssetResourceID(id),
			AssetID:    id,
		}, nil
	case domain.AssetResourceSourceSystem:
		if _, ok := domain.ParseExternalAssetResourceID(rawRef); ok {
			return clientMaterialSourceRef{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "system client material requires a numeric asset_id.", nil)
		}
		id := assetID
		if strings.TrimSpace(rawRef) != "" {
			parsed, err := strconv.ParseInt(strings.TrimSpace(rawRef), 10, 64)
			if err != nil || parsed <= 0 {
				return clientMaterialSourceRef{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required.", nil)
			}
			id = parsed
		}
		if id <= 0 {
			return clientMaterialSourceRef{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_id is required.", nil)
		}
		return clientMaterialSourceRef{
			SourceType: string(domain.AssetResourceSourceSystem),
			SourceRef:  strconv.FormatInt(id, 10),
			AssetID:    id,
		}, nil
	default:
		return clientMaterialSourceRef{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_type must be system or external.", nil)
	}
}

func normalizeClientMaterialRow(material *domain.AssetWorkbenchClientMaterial) {
	if material == nil {
		return
	}
	sourceType := domain.NormalizeAssetResourceSource(material.SourceType)
	if sourceType == domain.AssetResourceSourceAll {
		sourceType = domain.AssetResourceSourceSystem
	}
	material.SourceType = string(sourceType)
	switch sourceType {
	case domain.AssetResourceSourceExternal:
		id, ok := domain.ParseExternalAssetResourceID(material.SourceRef)
		if !ok && material.AssetID > 0 {
			id = material.AssetID
		}
		if id > 0 {
			material.AssetID = id
			material.SourceRef = domain.ExternalAssetResourceID(id)
			material.ResourceID = material.SourceRef
		}
		material.SourceLabel = "外部资源"
	case domain.AssetResourceSourceSystem:
		if strings.TrimSpace(material.SourceRef) == "" && material.AssetID > 0 {
			material.SourceRef = strconv.FormatInt(material.AssetID, 10)
		}
		if strings.TrimSpace(material.ResourceID) == "" {
			material.ResourceID = material.SourceRef
		}
		material.SourceLabel = "系统资源"
	}
}

func clientMaterialSourceKey(sourceType, sourceRef string) string {
	normalized := domain.NormalizeAssetResourceSource(sourceType)
	if normalized == domain.AssetResourceSourceAll {
		normalized = domain.AssetResourceSourceSystem
	}
	return string(normalized) + ":" + strings.TrimSpace(sourceRef)
}

func (r *ClientMaterialBatchUpdateResult) addClientMaterialBatchFailure(index int, item CreateClientMaterialParams, reason string) {
	r.Failures = append(r.Failures, ClientMaterialBatchUpdateFailure{
		Index:      index,
		AssetID:    item.AssetID,
		SourceType: strings.TrimSpace(item.SourceType),
		SourceRef:  strings.TrimSpace(item.SourceRef),
		ResourceID: strings.TrimSpace(item.ResourceID),
		Reason:     strings.TrimSpace(reason),
	})
}

func (s *Service) clientMaterialSourceSnapshot(ctx context.Context, source clientMaterialSourceRef) (*clientMaterialSourceSnapshot, *domain.AppError) {
	switch domain.NormalizeAssetResourceSource(source.SourceType) {
	case domain.AssetResourceSourceExternal:
		detailer, _ := s.systemAssets.(ExternalAssetDetailer)
		if detailer == nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "External asset detailer is not configured.", nil)
		}
		detail, appErr := detailer.GetExternalDetail(ctx, source.AssetID)
		if appErr != nil {
			return nil, appErr
		}
		if detail == nil {
			return nil, domain.ErrNotFound
		}
		fileSize := int64(0)
		if detail.FileSize != nil {
			fileSize = *detail.FileSize
		}
		resourceID := firstNonEmpty(detail.ResourceID, source.SourceRef, domain.ExternalAssetResourceID(source.AssetID))
		return &clientMaterialSourceSnapshot{
			AssetID:          source.AssetID,
			SourceType:       string(domain.AssetResourceSourceExternal),
			SourceRef:        resourceID,
			ResourceID:       resourceID,
			SourceLabel:      firstNonEmpty(detail.SourceLabel, "外部资源"),
			Filename:         firstNonEmpty(detail.OriginalFilename, detail.FileName, fmt.Sprintf("external-material-%d", source.AssetID)),
			MimeType:         strings.TrimSpace(detail.MimeType),
			FileSize:         fileSize,
			ScopeSKUCode:     strings.TrimSpace(detail.ScopeSKUCode),
			SKUCode:          strings.TrimSpace(detail.SKUCode),
			PrimarySKUCode:   strings.TrimSpace(detail.PrimarySKUCode),
			PreviewAvailable: detail.PreviewAvailable,
		}, nil
	default:
		info, appErr := s.systemDownloadSnapshot(ctx, source.AssetID)
		if appErr != nil {
			return nil, appErr
		}
		snapshot := &clientMaterialSourceSnapshot{
			AssetID:          source.AssetID,
			SourceType:       string(domain.AssetResourceSourceSystem),
			SourceRef:        strconv.FormatInt(source.AssetID, 10),
			ResourceID:       strconv.FormatInt(source.AssetID, 10),
			SourceLabel:      "系统资源",
			Filename:         strings.TrimSpace(info.Filename),
			MimeType:         strings.TrimSpace(info.MimeType),
			FileSize:         info.FileSize,
			PreviewAvailable: info.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(info.MimeType, info.Filename),
		}
		if detailer, _ := s.systemAssets.(SystemAssetDetailer); detailer != nil {
			if detail, detailErr := detailer.GetDetail(ctx, source.AssetID); detailErr == nil && detail != nil {
				snapshot.ScopeSKUCode = strings.TrimSpace(detail.ScopeSKUCode)
				snapshot.SKUCode = strings.TrimSpace(detail.SKUCode)
				snapshot.PrimarySKUCode = strings.TrimSpace(detail.PrimarySKUCode)
				snapshot.Filename = firstNonEmpty(detail.OriginalFilename, detail.FileName, snapshot.Filename)
				snapshot.MimeType = firstNonEmpty(detail.MimeType, snapshot.MimeType)
				if detail.FileSize != nil {
					snapshot.FileSize = *detail.FileSize
				}
				snapshot.PreviewAvailable = detail.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(snapshot.MimeType, snapshot.Filename)
			}
		}
		return snapshot, nil
	}
}

func (s *Service) clientMaterialDownloadInfo(ctx context.Context, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetDownloadInfo, *domain.AppError) {
	normalizeClientMaterialRow(material)
	switch domain.NormalizeAssetResourceSource(material.SourceType) {
	case domain.AssetResourceSourceExternal:
		downloader, _ := s.systemAssets.(ExternalAssetDownloader)
		if downloader == nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "External asset downloader is not configured.", nil)
		}
		return downloader.DownloadExternal(ctx, material.AssetID)
	default:
		return s.systemDownloadSnapshot(ctx, material.AssetID)
	}
}

func (s *Service) clientMaterialPreviewMeta(ctx context.Context, material *domain.AssetWorkbenchClientMaterial) (*SystemAssetPreviewMeta, *domain.AppError) {
	normalizeClientMaterialRow(material)
	if domain.NormalizeAssetResourceSource(material.SourceType) != domain.AssetResourceSourceExternal {
		meta, appErr := s.systemAssetPreviewMeta(ctx, material.AssetID)
		if meta != nil {
			meta.SourceType = material.SourceType
			meta.SourceRef = material.SourceRef
		}
		return meta, appErr
	}
	downloader, _ := s.systemAssets.(ExternalAssetDownloader)
	if downloader == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "External asset downloader is not configured.", nil)
	}
	info, appErr := downloader.PreviewExternal(ctx, material.AssetID)
	if appErr != nil {
		if appErr.Code == domain.ErrCodeInvalidRequest {
			return clientMaterialPreviewMetaFromDownloadInfo(material, nil), nil
		}
		return nil, appErr
	}
	return clientMaterialPreviewMetaFromDownloadInfo(material, info), nil
}

func clientMaterialPreviewMetaFromDownloadInfo(material *domain.AssetWorkbenchClientMaterial, info *domain.AssetDownloadInfo) *SystemAssetPreviewMeta {
	meta := &SystemAssetPreviewMeta{
		AssetID:          material.AssetID,
		SourceType:       material.SourceType,
		SourceRef:        material.SourceRef,
		Status:           domain.AssetWorkbenchPreviewStatusNotApplicable,
		Preparing:        false,
		Filename:         material.FilenameSnapshot,
		MimeType:         material.MimeTypeSnapshot,
		PreviewAvailable: false,
	}
	if info == nil {
		return meta
	}
	meta.Filename = firstNonEmpty(info.Filename, meta.Filename)
	meta.MimeType = firstNonEmpty(info.MimeType, meta.MimeType)
	meta.ExpiresAt = info.ExpiresAt
	if info.DownloadURL != nil {
		meta.DownloadURL = strings.TrimSpace(*info.DownloadURL)
	}
	if meta.DownloadURL != "" && (info.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(meta.MimeType, meta.Filename)) {
		meta.Status = domain.AssetWorkbenchPreviewStatusReady
		meta.PreviewURL = meta.DownloadURL
		meta.PreviewAvailable = true
		return meta
	}
	if strings.Contains(strings.TrimSpace(info.AccessHint), "prepare_required") {
		meta.Status = domain.AssetWorkbenchPreviewStatusPending
		meta.Preparing = true
	}
	return meta
}

func normalizeOverviewSearchScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "operational", "materials", "material", "assets":
		return "operational"
	case "files", "submission_file", "submission_files":
		return "files"
	case "orders", "items", "piecework":
		return "orders"
	default:
		return "all"
	}
}

func overviewSearchIncludes(scope string) (submissions bool, items bool, files bool, operational bool) {
	switch normalizeOverviewSearchScope(scope) {
	case "operational":
		return false, false, false, true
	case "files":
		return false, false, true, false
	case "orders":
		return true, true, false, false
	default:
		return true, true, true, true
	}
}

func runAssetWorkbenchSearchJobs(jobs ...func() error) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := job(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func decorateOverviewRow(row *domain.AssetWorkbenchOverviewRow) {
	if row == nil {
		return
	}
	row.Scope = overviewScopeForSource(row.Source)
	row.SourceLabel = overviewSourceLabel(row.Source)
	row.Locate = overviewLocate(row)
}

func overviewScopeForSource(source string) string {
	switch source {
	case "system_asset", "client_material":
		return "operational"
	case "submission_file":
		return "files"
	default:
		return "orders"
	}
}

func overviewSourceLabel(source string) string {
	switch source {
	case "system_asset":
		return "运营素材"
	case "client_material":
		return "可下载素材"
	case "submission_file":
		return "交稿文件"
	case "piecework_item":
		return "订单·计件"
	case "submission":
		return "提交记录"
	default:
		return source
	}
}

func overviewMetaMap(row *domain.AssetWorkbenchOverviewRow) map[string]interface{} {
	if row == nil || len(row.Meta) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(row.Meta, &out); err != nil {
		return nil
	}
	return out
}

func overviewLocate(row *domain.AssetWorkbenchOverviewRow) json.RawMessage {
	if row == nil {
		return nil
	}
	meta := overviewMetaMap(row)
	locate := map[string]interface{}{"source": row.Source}
	switch row.Source {
	case "system_asset":
		locate["source_type"] = firstNonEmpty(stringFromMeta(meta, "source_type"), string(domain.AssetResourceSourceSystem))
		locate["source_ref"] = firstNonEmpty(stringFromMeta(meta, "resource_id"), strconv.FormatInt(row.ID, 10))
		locate["resource_id"] = firstNonEmpty(stringFromMeta(meta, "resource_id"), strconv.FormatInt(row.ID, 10))
	case "client_material":
		locate["material_id"] = row.ID
		locate["source_type"] = stringFromMeta(meta, "source_type")
		locate["source_ref"] = stringFromMeta(meta, "source_ref")
		locate["resource_id"] = stringFromMeta(meta, "resource_id")
	case "submission_file":
		locate["file_id"] = row.ID
		locate["submission_id"] = numberFromMeta(meta, "submission_id")
		locate["item_id"] = numberFromMeta(meta, "submission_item_id")
	case "piecework_item":
		locate["item_id"] = row.ID
		locate["submission_id"] = numberFromMeta(meta, "submission_id")
		locate["order_no"] = row.OrderNo
	case "submission":
		locate["submission_id"] = row.ID
		locate["query"] = firstNonEmpty(row.PrimaryCode, row.Title)
	}
	return mustJSON(locate)
}

func stringFromMeta(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	switch value := meta[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func numberFromMeta(meta map[string]interface{}, key string) int64 {
	if meta == nil {
		return 0
	}
	switch value := meta[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	default:
		return 0
	}
}

func (s *Service) searchClientMaterialsForOverview(ctx context.Context, keyword string, creator string, limit int) ([]*domain.AssetWorkbenchOverviewRow, int64, error) {
	if strings.TrimSpace(creator) != "" {
		return []*domain.AssetWorkbenchOverviewRow{}, 0, nil
	}
	enabled := true
	rows, err := s.repo.ListClientMaterials(ctx, repo.AssetWorkbenchClientMaterialFilter{Enabled: &enabled})
	if err != nil {
		return nil, 0, err
	}
	items := []*domain.AssetWorkbenchOverviewRow{}
	seen := make(map[int64]struct{}, len(rows))
	publishedByAsset := make(map[string][]*domain.AssetWorkbenchClientMaterial, len(rows))
	for _, material := range rows {
		normalizeClientMaterialRow(material)
		if material != nil && material.AssetID > 0 {
			key := clientMaterialAssetIndexKey(material.SourceType, material.AssetID)
			publishedByAsset[key] = append(publishedByAsset[key], material)
		}
		if !clientMaterialMatchesOverviewKeyword(material, keyword) {
			continue
		}
		row := overviewRowFromClientMaterial(material)
		if row != nil {
			items = append(items, row)
			seen[material.ID] = struct{}{}
		}
	}
	if strings.TrimSpace(keyword) != "" && externalMaterialCodePattern.MatchString(strings.TrimSpace(keyword)) && s.systemAssets != nil {
		searchSize := limit
		if searchSize <= 0 {
			searchSize = 50
		}
		indexed, appErr := s.systemAssets.Search(ctx, domain.AssetSearchQuery{
			Keyword:        strings.TrimSpace(keyword),
			Page:           1,
			Size:           searchSize,
			Source:         domain.AssetResourceSourceAll,
			UsableState:    domain.AssetUsableStateFilterAll,
			FormatCategory: domain.AssetFormatCategoryAll,
			IsArchived:     domain.AssetArchiveFilterFalse,
			TaskStatus:     domain.AssetTaskStatusFilterAll,
		})
		if appErr != nil {
			return nil, 0, appErr
		}
		if indexed != nil {
			for _, asset := range indexed.Items {
				if asset == nil || asset.ID <= 0 {
					continue
				}
				for _, material := range publishedByAsset[clientMaterialAssetIndexKey(asset.SourceType, asset.ID)] {
					if _, ok := seen[material.ID]; ok {
						continue
					}
					copyMaterial := *material
					copyMaterial.ScopeSKUCode = strings.TrimSpace(asset.ScopeSKUCode)
					copyMaterial.SKUCode = strings.TrimSpace(asset.SKUCode)
					copyMaterial.PrimarySKUCode = strings.TrimSpace(asset.PrimarySKUCode)
					copyMaterial.BusinessLane = string(asset.BusinessLane)
					if !clientMaterialMatchesOverviewKeyword(&copyMaterial, keyword) {
						continue
					}
					if row := overviewRowFromClientMaterial(&copyMaterial); row != nil {
						items = append(items, row)
						seen[material.ID] = struct{}{}
					}
				}
			}
		}
	}
	total := int64(len(items))
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, total, nil
}

func clientMaterialAssetIndexKey(sourceType string, assetID int64) string {
	source := domain.NormalizeAssetResourceSource(sourceType)
	if source == domain.AssetResourceSourceAll {
		source = domain.AssetResourceSourceSystem
	}
	return fmt.Sprintf("%s:%d", source, assetID)
}

func clientMaterialMatchesOverviewKeyword(material *domain.AssetWorkbenchClientMaterial, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	if material == nil {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{
		material.Title,
		material.Description,
		material.FilenameSnapshot,
		material.SourceRef,
		material.ResourceID,
		material.ScopeSKUCode,
		material.SKUCode,
		material.PrimarySKUCode,
	}, " "))
	return strings.Contains(haystack, keyword)
}

func overviewRowFromClientMaterial(material *domain.AssetWorkbenchClientMaterial) *domain.AssetWorkbenchOverviewRow {
	if material == nil {
		return nil
	}
	title := firstNonEmpty(material.Title, material.FilenameSnapshot, material.ResourceID, fmt.Sprintf("素材 %d", material.ID))
	updatedAt := material.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = material.PublishedAt
	}
	row := &domain.AssetWorkbenchOverviewRow{
		Source:        "client_material",
		Scope:         "operational",
		SourceLabel:   "可下载素材",
		ID:            material.ID,
		Title:         title,
		PrimaryCode:   firstNonEmpty(material.ResourceID, material.SourceRef, strconv.FormatInt(material.AssetID, 10)),
		SecondaryCode: firstNonEmpty(material.ScopeSKUCode, material.SKUCode, material.PrimarySKUCode),
		Status:        map[bool]string{true: "enabled", false: "disabled"}[material.Enabled],
		CreatedAt:     material.PublishedAt,
		UpdatedAt:     updatedAt,
		RoutePath:     fmt.Sprintf("/drive?scope=operational&material_id=%d", material.ID),
		Meta: mustJSON(map[string]interface{}{
			"material_id":       material.ID,
			"asset_id":          material.AssetID,
			"source_type":       material.SourceType,
			"source_ref":        material.SourceRef,
			"resource_id":       material.ResourceID,
			"source_label":      material.SourceLabel,
			"filename":          material.FilenameSnapshot,
			"mime_type":         material.MimeTypeSnapshot,
			"file_size":         material.FileSizeSnapshot,
			"preview_available": material.PreviewAvailable,
			"scope_sku_code":    material.ScopeSKUCode,
			"sku_code":          material.SKUCode,
			"primary_sku_code":  material.PrimarySKUCode,
			"business_lane":     material.BusinessLane,
		}),
	}
	row.Locate = overviewLocate(row)
	return row
}

func overviewRowFromSystemAsset(asset *assetcenter.AssetDetail) *domain.AssetWorkbenchOverviewRow {
	if asset == nil {
		return nil
	}
	title := firstNonEmpty(asset.ProductName, asset.OriginalFilename, asset.FileName, asset.TaskNo, fmt.Sprintf("素材 %d", asset.ID))
	primaryCode := firstNonEmpty(asset.AssetNo, asset.ResourceID, fmt.Sprintf("%d", asset.ID))
	secondaryCode := firstNonEmpty(asset.ScopeSKUCode, asset.SKUCode, asset.PrimarySKUCode)
	creatorID := asset.CreatedBy
	creatorName := firstNonEmpty(asset.CreatedByName, asset.CreatedByUsername, asset.TaskCreatorName, asset.TaskCreatorUsername)
	if creatorID <= 0 {
		creatorID = asset.TaskCreatorID
	}
	if creatorName == "" && creatorID > 0 {
		creatorName = fmt.Sprintf("用户 %d", creatorID)
	}
	previewURL := ""
	if asset.PreviewURL != nil {
		previewURL = strings.TrimSpace(*asset.PreviewURL)
	}
	status := strings.TrimSpace(string(asset.UsableState))
	if status == "" {
		status = strings.TrimSpace(string(asset.TaskStatus))
	}
	row := &domain.AssetWorkbenchOverviewRow{
		Source:        "system_asset",
		Scope:         "operational",
		SourceLabel:   firstNonEmpty(asset.SourceLabel, "运营素材"),
		ID:            asset.ID,
		Title:         title,
		PrimaryCode:   primaryCode,
		SecondaryCode: secondaryCode,
		OrderNo:       asset.TaskNo,
		CreatorUserID: creatorID,
		CreatorName:   creatorName,
		Status:        status,
		CreatedAt:     asset.CreatedAt,
		UpdatedAt:     asset.UpdatedAt,
		RoutePath:     fmt.Sprintf("/drive?scope=operational&asset_id=%d", asset.ID),
		Meta: mustJSON(map[string]interface{}{
			"asset_no":            asset.AssetNo,
			"resource_id":         asset.ResourceID,
			"source_type":         firstNonEmpty(asset.SourceType, string(domain.AssetResourceSourceSystem)),
			"source_label":        asset.SourceLabel,
			"task_no":             asset.TaskNo,
			"product_name":        asset.ProductName,
			"file_name":           asset.FileName,
			"original_filename":   asset.OriginalFilename,
			"mime_type":           asset.MimeType,
			"scope_sku_code":      asset.ScopeSKUCode,
			"sku_code":            asset.SKUCode,
			"primary_sku_code":    asset.PrimarySKUCode,
			"business_lane":       asset.BusinessLane,
			"preview_available":   asset.PreviewAvailable,
			"preview_url":         previewURL,
			"task_creator_name":   asset.TaskCreatorName,
			"created_by_name":     asset.CreatedByName,
			"created_by_username": asset.CreatedByUsername,
		}),
	}
	row.Locate = overviewLocate(row)
	return row
}

func overviewSystemAssetMatchesCreator(row *domain.AssetWorkbenchOverviewRow, creator string) bool {
	creator = strings.TrimSpace(strings.ToLower(creator))
	if creator == "" {
		return true
	}
	if row == nil {
		return false
	}
	return strings.Contains(strings.ToLower(row.CreatorName), creator)
}

func (s *Service) resolveDownloadableClientMaterial(ctx context.Context, actor domain.RequestActor, materialID int64) (*domain.AssetWorkbenchClientMaterial, *domain.AppError) {
	if materialID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "material_id is required.", nil)
	}
	material, err := s.repo.GetClientMaterial(ctx, materialID)
	if err != nil {
		return nil, mapRepoReadError(err, "Client material not found.", "Failed to load client material.")
	}
	if material == nil || (!material.Enabled && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin)) {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "Client material not found.", nil)
	}
	return material, nil
}

func (s *Service) hydrateClientMaterialRows(ctx context.Context, items []*domain.AssetWorkbenchClientMaterial) {
	if len(items) == 0 {
		return
	}
	detailer, _ := s.systemAssets.(SystemAssetDetailer)
	for _, item := range items {
		if item == nil {
			continue
		}
		normalizeClientMaterialRow(item)
		item.PreviewAvailable = item.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(item.MimeTypeSnapshot, item.FilenameSnapshot)
		if domain.NormalizeAssetResourceSource(item.SourceType) == domain.AssetResourceSourceExternal {
			externalDetailer, _ := s.systemAssets.(ExternalAssetDetailer)
			if externalDetailer == nil || item.AssetID <= 0 {
				continue
			}
			detail, appErr := externalDetailer.GetExternalDetail(ctx, item.AssetID)
			if appErr != nil || detail == nil {
				continue
			}
			item.ResourceID = firstNonEmpty(detail.ResourceID, item.ResourceID, item.SourceRef)
			item.SourceLabel = firstNonEmpty(detail.SourceLabel, item.SourceLabel, "外部资源")
			item.ScopeSKUCode = strings.TrimSpace(detail.ScopeSKUCode)
			item.SKUCode = strings.TrimSpace(detail.SKUCode)
			item.PrimarySKUCode = strings.TrimSpace(detail.PrimarySKUCode)
			filename := firstNonEmpty(detail.OriginalFilename, detail.FileName, item.FilenameSnapshot)
			mimeType := firstNonEmpty(detail.MimeType, item.MimeTypeSnapshot)
			item.PreviewAvailable = detail.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(mimeType, filename)
			continue
		}
		if detailer == nil || item.AssetID <= 0 {
			continue
		}
		detail, appErr := detailer.GetDetail(ctx, item.AssetID)
		if appErr != nil || detail == nil {
			continue
		}
		item.ScopeSKUCode = strings.TrimSpace(detail.ScopeSKUCode)
		item.SKUCode = strings.TrimSpace(detail.SKUCode)
		item.PrimarySKUCode = strings.TrimSpace(detail.PrimarySKUCode)
		item.BusinessLane = string(detail.BusinessLane)
		filename := firstNonEmpty(detail.OriginalFilename, detail.FileName, item.FilenameSnapshot)
		mimeType := firstNonEmpty(detail.MimeType, item.MimeTypeSnapshot)
		item.PreviewAvailable = detail.PreviewAvailable || isWorkbenchSystemAssetDirectPreviewable(mimeType, filename)
	}
}

func clientMaterialTitle(raw string, assetID int64, info *domain.AssetDownloadInfo) string {
	title := strings.TrimSpace(raw)
	if title != "" {
		return title
	}
	if info != nil && strings.TrimSpace(info.Filename) != "" {
		return strings.TrimSpace(info.Filename)
	}
	return fmt.Sprintf("素材 %d", assetID)
}

func (s *Service) buildPreviewKey(now time.Time, fileID int64) string {
	return fmt.Sprintf("%s/previews/%s/%d-%s.webp", strings.Trim(s.cfg.OSSPrefix, "/"), now.Format("2006/01"), fileID, uuid.NewString())
}

func (s *Service) businessMonth(t time.Time) string {
	return t.In(s.loc).Format("2006-01")
}

func (s *Service) resolveSubmissionBusinessMonth(ctx context.Context, actor domain.RequestActor, params CreateSubmissionParams, now time.Time) (string, *domain.AppError) {
	currentMonth := s.businessMonth(now)
	expectedMonth := strings.TrimSpace(params.ExpectedBusinessMonth)
	if expectedMonth != "" {
		if _, err := time.Parse("2006-01", expectedMonth); err != nil {
			return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_business_month must use YYYY-MM format.", map[string]string{"expected_business_month": expectedMonth})
		}
	}
	overrideMonth := strings.TrimSpace(params.BusinessMonthOverride)
	if overrideMonth != "" {
		if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
			return "", domain.NewAppError(domain.ErrCodePermissionDenied, "Only asset managers can override business_month.", nil)
		}
		if _, err := time.Parse("2006-01", overrideMonth); err != nil {
			return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month_override must use YYYY-MM format.", map[string]string{"business_month_override": overrideMonth})
		}
		if appErr := s.ensureBusinessMonthOpenForManualSubmission(ctx, overrideMonth); appErr != nil {
			return "", appErr
		}
		return overrideMonth, nil
	}
	if expectedMonth != "" && expectedMonth != currentMonth && !params.MonthRolloverAck {
		return "", domain.NewAppError("MONTH_ROLLOVER_REQUIRED", "Business month changed before submission; confirm the upload should count to the current month.", map[string]string{
			"expected_business_month": expectedMonth,
			"current_business_month":  currentMonth,
		})
	}
	return currentMonth, nil
}

func (s *Service) ensureBusinessMonthOpenForManualSubmission(ctx context.Context, businessMonth string) *domain.AppError {
	if s.repo == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench repository is not configured.", nil)
	}
	batches, _, err := s.repo.ListSettlementBatches(ctx, repo.AssetWorkbenchSettlementBatchFilter{
		BusinessMonth: businessMonth,
		Page:          1,
		PageSize:      100,
	})
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check settlement batch state.", err.Error())
	}
	for _, batch := range batches {
		if batch == nil {
			continue
		}
		if batch.Status != domain.AssetWorkbenchBatchStatusCancelled {
			return domain.NewAppError(domain.ErrCodeConflict, "business_month_override is locked by an existing settlement batch.", map[string]interface{}{
				"business_month": businessMonth,
				"batch_id":       batch.ID,
				"batch_status":   batch.Status,
			})
		}
	}
	return nil
}

func (s *Service) requireRepo() *domain.AppError {
	if s.repo == nil || s.tx == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench repository is not configured.", nil)
	}
	return nil
}

func (s *Service) ensureDifficultyClass(ctx context.Context, difficulty string, allowAll bool) *domain.AppError {
	code, appErr := normalizeWorkbenchDifficultyCode(difficulty, allowAll)
	if appErr != nil {
		return appErr
	}
	if allowAll && code == domain.AssetWorkbenchWorkerTypeAll {
		return nil
	}
	item, err := s.repo.GetDifficultyClass(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class is not configured.", map[string]string{"difficulty_class": code})
		}
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to load difficulty class.", err.Error())
	}
	if !item.Enabled {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class is disabled.", map[string]string{"difficulty_class": code})
	}
	return nil
}

func normalizePriceMatrix(actorID int64, params CreatePriceMatrixParams) (*domain.AssetWorkbenchPriceMatrix, *domain.AppError) {
	workerType := normalizeWorkerType(params.WorkerType)
	jobGrade := strings.TrimSpace(params.JobGrade)
	difficulty, appErr := normalizeWorkbenchDifficultyCode(params.DifficultyClass, false)
	if appErr != nil {
		return nil, appErr
	}
	if workerType == "" || jobGrade == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "worker_type, job_grade and difficulty_class are required.", nil)
	}
	if !validWorkbenchJobGrade(workerType, jobGrade, true) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_grade is not valid for worker_type.", map[string]string{"worker_type": workerType, "job_grade": jobGrade})
	}
	if params.UnitPrice < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unit_price must be non-negative.", nil)
	}
	if params.EffectiveFrom.IsZero() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_from is required.", nil)
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_to must be after effective_from.", nil)
	}
	return &domain.AssetWorkbenchPriceMatrix{
		WorkerType:      workerType,
		JobGrade:        jobGrade,
		DifficultyClass: difficulty,
		UnitPrice:       params.UnitPrice,
		EffectiveFrom:   truncateDate(params.EffectiveFrom),
		EffectiveTo:     truncateOptionalDate(params.EffectiveTo),
		Enabled:         true,
		RevisionNo:      1,
		CreatedBy:       actorID,
		Remark:          strings.TrimSpace(params.Remark),
	}, nil
}

func normalizeDeductionRule(actorID int64, params CreateDeductionRuleParams) (*domain.AssetWorkbenchDeductionRule, *domain.AppError) {
	workerType := normalizeWorkerType(defaultAll(params.WorkerType))
	jobGrade := strings.TrimSpace(defaultAll(params.JobGrade))
	difficulty, appErr := normalizeWorkbenchDifficultyCode(params.DifficultyClass, true)
	if appErr != nil {
		return nil, appErr
	}
	if workerType == "" || jobGrade == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "worker_type, job_grade and difficulty_class are required.", nil)
	}
	if !validWorkbenchJobGrade(workerType, jobGrade, true) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_grade is not valid for worker_type.", map[string]string{"worker_type": workerType, "job_grade": jobGrade})
	}
	if params.DeductionAmount < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "deduction_amount must be non-negative.", nil)
	}
	if params.EffectiveFrom.IsZero() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_from is required.", nil)
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_to must be after effective_from.", nil)
	}
	return &domain.AssetWorkbenchDeductionRule{
		WorkerType:      workerType,
		JobGrade:        jobGrade,
		DifficultyClass: difficulty,
		DeductionAmount: params.DeductionAmount,
		EffectiveFrom:   truncateDate(params.EffectiveFrom),
		EffectiveTo:     truncateOptionalDate(params.EffectiveTo),
		Enabled:         true,
		RevisionNo:      1,
		CreatedBy:       actorID,
		Remark:          strings.TrimSpace(params.Remark),
	}, nil
}

func normalizeWelfareRule(actorID int64, params CreateWelfareRuleParams) (*domain.AssetWorkbenchWelfareRule, *domain.AppError) {
	ruleName := strings.TrimSpace(params.RuleName)
	if ruleName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_name is required.", nil)
	}
	if params.Amount < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "amount must be non-negative.", nil)
	}
	if params.EffectiveFrom.IsZero() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_from is required.", nil)
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_to must be after effective_from.", nil)
	}
	ruleType := strings.TrimSpace(params.RuleType)
	if ruleType == "" {
		ruleType = "manual"
	}
	workerType := normalizeWorkerType(defaultAll(params.WorkerType))
	jobGrade := strings.TrimSpace(defaultAll(params.JobGrade))
	if !validWorkbenchJobGrade(workerType, jobGrade, true) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_grade is not valid for worker_type.", map[string]string{"worker_type": workerType, "job_grade": jobGrade})
	}
	return &domain.AssetWorkbenchWelfareRule{
		RuleName:      ruleName,
		WorkerType:    workerType,
		JobGrade:      jobGrade,
		RuleType:      ruleType,
		Amount:        params.Amount,
		Config:        normalizeJSON(params.Config),
		EffectiveFrom: truncateDate(params.EffectiveFrom),
		EffectiveTo:   truncateOptionalDate(params.EffectiveTo),
		Enabled:       true,
		CreatedBy:     actorID,
		Remark:        strings.TrimSpace(params.Remark),
	}, nil
}

func normalizePromoCoupon(actorID int64, params CreatePromoCouponParams) (*domain.AssetWorkbenchPromoCoupon, *domain.AppError) {
	code := strings.TrimSpace(params.CouponCode)
	name := strings.TrimSpace(params.CouponName)
	mode := normalizePromoMode(params.Mode)
	if code == "" || name == "" || mode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "coupon_code, coupon_name and mode are required.", nil)
	}
	if params.EffectiveFrom.IsZero() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_from is required.", nil)
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "effective_to must be after effective_from.", nil)
	}
	switch mode {
	case domain.AssetWorkbenchPromoModeFixedPrice, domain.AssetWorkbenchPromoModeMarkupAmount:
		if params.Amount == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "amount is required for this promo mode.", nil)
		}
	case domain.AssetWorkbenchPromoModeMarkupRate:
		if params.Percent == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "percent is required for markup_rate mode.", nil)
		}
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported promo mode.", map[string]string{"mode": params.Mode})
	}
	if params.Priority == 0 {
		params.Priority = 100
	}
	workerType := normalizeWorkerType(defaultAll(params.WorkerType))
	jobGrade := strings.TrimSpace(defaultAll(params.JobGrade))
	difficulty, appErr := normalizeWorkbenchDifficultyCode(defaultAll(params.DifficultyClass), true)
	if appErr != nil {
		return nil, appErr
	}
	if !validWorkbenchJobGrade(workerType, jobGrade, true) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_grade is not valid for worker_type.", map[string]string{"worker_type": workerType, "job_grade": jobGrade})
	}
	return &domain.AssetWorkbenchPromoCoupon{
		CouponCode:      code,
		CouponName:      name,
		Mode:            mode,
		Amount:          params.Amount,
		Percent:         params.Percent,
		Priority:        params.Priority,
		WorkerType:      workerType,
		JobGrade:        jobGrade,
		DifficultyClass: difficulty,
		EligibleUserIDs: normalizeJSON(params.EligibleUserIDs),
		EligibleCodes:   normalizeJSON(params.EligibleCodes),
		EffectiveFrom:   params.EffectiveFrom,
		EffectiveTo:     params.EffectiveTo,
		Enabled:         true,
		StackPolicy:     "single_winner",
		CreatedBy:       actorID,
		Remark:          strings.TrimSpace(params.Remark),
	}, nil
}

func normalizeSettlementSupplement(actorID int64, params CreateSettlementSupplementParams) (*domain.AssetWorkbenchSettlementSupplement, *domain.AppError) {
	businessMonth := strings.TrimSpace(params.BusinessMonth)
	supplementDate := strings.TrimSpace(params.SupplementDate)
	if supplementDate != "" {
		normalizedDate, _, ok := normalizeSupplementDate(supplementDate)
		if !ok {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplement_date must use YYYY-MM-DD.", nil)
		}
		supplementDate = normalizedDate
	}
	if _, err := time.Parse("2006-01", businessMonth); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month must use YYYY-MM.", nil)
	}
	orderNo := strings.TrimSpace(params.OrderNo)
	difficulty, appErr := normalizeWorkbenchDifficultyCode(params.DifficultyClass, false)
	if appErr != nil {
		return nil, appErr
	}
	if params.PayeeUserID <= 0 || orderNo == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id, order_no and difficulty_class are required.", nil)
	}
	if params.PageCount <= 0 {
		params.PageCount = 1
	}
	if params.GrossAmount < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "gross_amount must be non-negative.", nil)
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = domain.AssetWorkbenchSupplementStatusApproved
	}
	if status != domain.AssetWorkbenchSupplementStatusDraft && status != domain.AssetWorkbenchSupplementStatusApproved {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplement status must be draft or approved.", nil)
	}
	return &domain.AssetWorkbenchSettlementSupplement{
		PayeeUserID:     params.PayeeUserID,
		BusinessMonth:   businessMonth,
		Status:          status,
		OrderNo:         orderNo,
		SupplementDate:  supplementDate,
		DifficultyClass: difficulty,
		Finalized:       params.Finalized,
		PageCount:       params.PageCount,
		GrossAmount:     params.GrossAmount,
		DuplicateHint: mustJSON(map[string]interface{}{
			"dedupe_scope":    "payee_user_id + business_month + order_no",
			"strength":        "hint",
			"supplement_date": supplementDate,
		}),
		CreatedBy: actorID,
	}, nil
}

func normalizeSettlementSupplementFilter(filter repo.AssetWorkbenchSettlementSupplementFilter) (repo.AssetWorkbenchSettlementSupplementFilter, *domain.AppError) {
	filter.BusinessMonth = strings.TrimSpace(filter.BusinessMonth)
	filter.OrderNo = strings.TrimSpace(filter.OrderNo)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.SupplementDate = strings.TrimSpace(filter.SupplementDate)
	filter.SupplementDateFrom = strings.TrimSpace(filter.SupplementDateFrom)
	filter.SupplementDateTo = strings.TrimSpace(filter.SupplementDateTo)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	filter.SortDir = strings.TrimSpace(strings.ToLower(filter.SortDir))
	if filter.BusinessMonth != "" {
		if _, err := time.Parse("2006-01", filter.BusinessMonth); err != nil {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month must use YYYY-MM.", nil)
		}
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "supplement_date", value: filter.SupplementDate},
		{name: "supplement_date_from", value: filter.SupplementDateFrom},
		{name: "supplement_date_to", value: filter.SupplementDateTo},
	} {
		if item.value == "" {
			continue
		}
		normalized, _, ok := normalizeSupplementDate(item.value)
		if !ok {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, item.name+" must use YYYY-MM-DD.", nil)
		}
		switch item.name {
		case "supplement_date":
			filter.SupplementDate = normalized
		case "supplement_date_from":
			filter.SupplementDateFrom = normalized
		case "supplement_date_to":
			filter.SupplementDateTo = normalized
		}
	}
	if filter.SupplementDateFrom != "" && filter.SupplementDateTo != "" && filter.SupplementDateFrom > filter.SupplementDateTo {
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplement_date_from must not be after supplement_date_to.", nil)
	}
	if filter.SortDir != "" && filter.SortDir != "asc" && filter.SortDir != "desc" {
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "sort_dir must be asc or desc.", nil)
	}
	if filter.SortBy != "" && !allowedSettlementSupplementSorts[filter.SortBy] {
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "sort_by is not supported.", map[string][]string{
			"allowed": settlementSupplementSortKeys(),
		})
	}
	return filter, nil
}

var allowedSettlementSupplementSorts = map[string]bool{
	"id":              true,
	"business_month":  true,
	"payee_user_id":   true,
	"order_no":        true,
	"supplement_date": true,
	"status":          true,
	"gross_amount":    true,
	"created_at":      true,
	"updated_at":      true,
}

func settlementSupplementSortKeys() []string {
	keys := make([]string, 0, len(allowedSettlementSupplementSorts))
	for key := range allowedSettlementSupplementSorts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeSupplementPermission(actorID int64, params UpsertSupplementPermissionParams, now time.Time) (*domain.AssetWorkbenchSupplementPermission, *domain.AppError) {
	businessMonth := strings.TrimSpace(params.BusinessMonth)
	if _, err := time.Parse("2006-01", businessMonth); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month must use YYYY-MM.", nil)
	}
	if params.PayeeUserID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id is required.", nil)
	}
	item := &domain.AssetWorkbenchSupplementPermission{
		PayeeUserID:   params.PayeeUserID,
		BusinessMonth: businessMonth,
		Enabled:       params.Enabled,
		Reason:        strings.TrimSpace(params.Reason),
		GrantedBy:     actorID,
	}
	if !params.Enabled {
		item.RevokedBy = &actorID
		item.RevokedAt = &now
	}
	return item, nil
}

func normalizeSupplementDate(raw string) (string, time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", time.Time{}, true
	}
	layouts := []string{"2006-01-02", "2006/01/02", "2006.01.02", "2006-1-2", "2006/1/2", "2006.1.2"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.Format("2006-01-02"), parsed, true
		}
	}
	return "", time.Time{}, false
}

func normalizeSettlementAdjustmentType(raw string) string {
	switch strings.TrimSpace(raw) {
	case domain.AssetWorkbenchAdjustmentTypeReversal:
		return domain.AssetWorkbenchAdjustmentTypeReversal
	default:
		return domain.AssetWorkbenchAdjustmentTypeAdjustment
	}
}

func normalizeSettlementAdjustmentDirection(raw string, adjustmentType string) string {
	switch strings.TrimSpace(raw) {
	case "credit":
		return "credit"
	case "debit":
		return "debit"
	default:
		if adjustmentType == domain.AssetWorkbenchAdjustmentTypeReversal {
			return "debit"
		}
		return "credit"
	}
}

func overlapsPricePeriod(existing []*domain.AssetWorkbenchPriceMatrix, start time.Time, end *time.Time) bool {
	for _, item := range existing {
		if !item.Enabled {
			continue
		}
		if periodsOverlap(start, end, item.EffectiveFrom, item.EffectiveTo) {
			return true
		}
	}
	return false
}

func overlapsDeductionPeriod(existing []*domain.AssetWorkbenchDeductionRule, start time.Time, end *time.Time) bool {
	for _, item := range existing {
		if !item.Enabled {
			continue
		}
		if periodsOverlap(start, end, item.EffectiveFrom, item.EffectiveTo) {
			return true
		}
	}
	return false
}

func nextPriceRevision(existing []*domain.AssetWorkbenchPriceMatrix) int {
	maxRevision := 0
	for _, item := range existing {
		if item.RevisionNo > maxRevision {
			maxRevision = item.RevisionNo
		}
	}
	return maxRevision + 1
}

func nextDeductionRevision(existing []*domain.AssetWorkbenchDeductionRule) int {
	maxRevision := 0
	for _, item := range existing {
		if item.RevisionNo > maxRevision {
			maxRevision = item.RevisionNo
		}
	}
	return maxRevision + 1
}

func periodsOverlap(aStart time.Time, aEnd *time.Time, bStart time.Time, bEnd *time.Time) bool {
	aEndValue := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	if aEnd != nil {
		aEndValue = *aEnd
	}
	bEndValue := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	if bEnd != nil {
		bEndValue = *bEnd
	}
	return !aEndValue.Before(bStart) && !bEndValue.Before(aStart)
}

func initialPreviewStatus(filename, mimeType string) string {
	fileType := inferFileType(filename, mimeType)
	switch fileType {
	case "image", "pdf", "design", "video":
		return domain.AssetWorkbenchPreviewStatusPending
	default:
		return domain.AssetWorkbenchPreviewStatusNotApplicable
	}
}

func inferFileType(filename, mimeType string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return "image"
	}
	if strings.HasPrefix(strings.ToLower(mimeType), "video/") {
		return "video"
	}
	switch ext {
	case "psd", "ai", "cdr", "sketch", "fig":
		return "design"
	case "pdf":
		return "pdf"
	case "mp4", "webm", "mov", "m4v":
		return "video"
	case "zip", "rar", "7z", "tar", "gz", "tgz":
		return "archive"
	default:
		if ext != "" {
			return ext
		}
		return "file"
	}
}

func isWorkbenchSystemAssetDirectPreviewable(mimeType string, filename string) bool {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mime, "image/") {
		return !strings.Contains(mime, "photoshop") && !strings.Contains(mime, "vnd.adobe")
	}
	if mime == "application/pdf" {
		return true
	}
	if strings.HasPrefix(mime, "video/") {
		return true
	}
	switch strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".") {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "tif", "tiff", "pdf", "mp4", "webm", "mov", "m4v":
		return true
	default:
		return false
	}
}

func normalizeWorkerType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full_time", "fulltime", "full-time", "全职":
		return domain.AssetWorkbenchWorkerTypeFulltime
	case "part_time", "parttime", "part-time", "兼职":
		return domain.AssetWorkbenchWorkerTypeParttime
	case domain.AssetWorkbenchWorkerTypeAll:
		return domain.AssetWorkbenchWorkerTypeAll
	default:
		return strings.TrimSpace(value)
	}
}

func defaultAll(value string) string {
	if strings.TrimSpace(value) == "" {
		return domain.AssetWorkbenchWorkerTypeAll
	}
	return value
}

func validWorkbenchJobGrade(workerType, jobGrade string, allowAll bool) bool {
	workerType = normalizeWorkerType(workerType)
	rawJobGrade := strings.TrimSpace(jobGrade)
	if allowAll && (workerType == domain.AssetWorkbenchWorkerTypeAll || strings.EqualFold(rawJobGrade, domain.AssetWorkbenchWorkerTypeAll)) {
		return workerType == domain.AssetWorkbenchWorkerTypeAll && strings.EqualFold(rawJobGrade, domain.AssetWorkbenchWorkerTypeAll)
	}
	jobGrade = strings.ToUpper(rawJobGrade)
	switch workerType {
	case domain.AssetWorkbenchWorkerTypeParttime:
		return jobGrade == "J1" || jobGrade == "J2" || jobGrade == "J3"
	case domain.AssetWorkbenchWorkerTypeFulltime:
		switch jobGrade {
		case "P1", "P2", "P3", "P4", "S1", "S2", "M1", "M2":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func validWorkbenchDifficulty(value string, allowAll bool) bool {
	_, err := normalizeWorkbenchDifficultyCode(value, allowAll)
	return err == nil
}

func normalizeWorkbenchDifficultyCode(value string, allowAll bool) (string, *domain.AppError) {
	code := strings.TrimSpace(value)
	if allowAll && strings.EqualFold(code, domain.AssetWorkbenchWorkerTypeAll) {
		return domain.AssetWorkbenchWorkerTypeAll, nil
	}
	if code == "" {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class is required.", nil)
	}
	if strings.EqualFold(code, domain.AssetWorkbenchWorkerTypeAll) {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class cannot use reserved all value.", map[string]string{"difficulty_class": code})
	}
	if len([]rune(code)) > 64 {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class cannot exceed 64 characters.", map[string]string{"difficulty_class": code})
	}
	if strings.ContainsAny(code, "\x00\r\n\t") {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "difficulty_class cannot contain control characters.", map[string]string{"difficulty_class": code})
	}
	return code, nil
}

func normalizePromoMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fixed", "fixed_price", "flat", "one_price", "一口价":
		return domain.AssetWorkbenchPromoModeFixedPrice
	case "markup_amount", "increase_amount", "amount", "涨额":
		return domain.AssetWorkbenchPromoModeMarkupAmount
	case "markup_rate", "markup_percent", "increase_percent", "percent", "涨幅":
		return domain.AssetWorkbenchPromoModeMarkupRate
	default:
		return strings.TrimSpace(value)
	}
}

func promoCouponApplies(coupon *domain.AssetWorkbenchPromoCoupon, payeeUserID int64, orderNo string) bool {
	if !jsonListContainsInt64OrEmpty(coupon.EligibleUserIDs, payeeUserID) {
		return false
	}
	return jsonListContainsStringOrEmpty(coupon.EligibleCodes, orderNo)
}

func jsonListContainsInt64OrEmpty(raw json.RawMessage, value int64) bool {
	if len(raw) == 0 || !json.Valid(raw) {
		return true
	}
	values := []int64{}
	if err := json.Unmarshal(raw, &values); err == nil {
		if len(values) == 0 {
			return true
		}
		for _, candidate := range values {
			if candidate == value {
				return true
			}
		}
		return false
	}
	stringsValue := []string{}
	if err := json.Unmarshal(raw, &stringsValue); err != nil {
		return true
	}
	if len(stringsValue) == 0 {
		return true
	}
	needle := fmt.Sprintf("%d", value)
	for _, candidate := range stringsValue {
		if strings.TrimSpace(candidate) == needle {
			return true
		}
	}
	return false
}

func jsonListContainsStringOrEmpty(raw json.RawMessage, value string) bool {
	if len(raw) == 0 || !json.Valid(raw) {
		return true
	}
	values := []string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return true
	}
	if len(values) == 0 {
		return true
	}
	value = strings.TrimSpace(value)
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == value {
			return true
		}
	}
	return false
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func previewRetryBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return time.Minute
	}
	if attempt == 2 {
		return 5 * time.Minute
	}
	return 15 * time.Minute
}

func writeWorkbenchPreviewSourceTempFile(reader io.Reader, filename, mimeType string) (string, func(), error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	if ext == "" {
		switch {
		case strings.Contains(strings.ToLower(mimeType), "photoshop"):
			ext = ".psd"
		case strings.Contains(strings.ToLower(mimeType), "illustrator"):
			ext = ".ai"
		case strings.Contains(strings.ToLower(mimeType), "pdf"):
			ext = ".pdf"
		default:
			ext = ".bin"
		}
	}
	if strings.ContainsAny(ext, `/\`) {
		ext = ".bin"
	}
	file, err := os.CreateTemp("", "asset-workbench-preview-*"+ext)
	if err != nil {
		return "", func() {}, fmt.Errorf("create preview source temp file: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write preview source temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close preview source temp file: %w", err)
	}
	return path, cleanup, nil
}

func parseErrorRecordsExcel(reader io.Reader) ([]ImportErrorRecordInput, *domain.AppError) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read error Excel file.", err.Error())
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no readable sheet.", nil)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read error Excel rows.", err.Error())
	}
	if len(rows) < 2 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file must include a header row and at least one data row.", nil)
	}
	headerRow, headers, ok := findErrorImportHeaderRow(rows)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing quality error import headers.", nil)
	}
	orderIndex, hasOrder := firstExcelColumn(headers, "order_no", "orderno", "订单号", "线上订单号", "线上单号", "文件名")
	errorIndex, hasErrorCount := firstExcelColumn(headers, "error_count", "errorcount", "errors", "出错数", "错误数", "出错数量", "错误件数", "错误张数", "出错张数")
	payeeIndex, hasPayee := firstExcelColumn(headers, "payee_user_id", "payeeuserid", "user_id", "userid", "人员id", "用户id")
	payeeNameIndex, hasPayeeName := firstExcelColumn(headers, "payee_name", "payeename", "出错人", "人员", "姓名", "计件人")
	difficultyIndex, hasDifficulty := firstExcelColumn(headers, "difficulty_class", "difficultyclass", "分类", "出错分类", "难度", "难度类", "难度类别")
	dateIndex, hasDate := firstExcelColumn(headers, "occurred_date", "occurreddate", "日期", "出错日期", "发生日期")
	issueIndex, hasIssue := firstExcelColumn(headers, "issue_description", "issuedescription", "问题描述", "问题", "错误描述")
	sourceIndex, hasSource := firstExcelColumn(headers, "source_type", "sourcetype", "抽查/售后", "来源", "类型")
	methodIndex, hasMethod := firstExcelColumn(headers, "handling_method", "handlingmethod", "处理方法", "处理方式")
	reporterIndex, hasReporter := firstExcelColumn(headers, "reporter_name", "reportername", "登记人", "记录人")
	remarkIndex, hasRemark := firstExcelColumn(headers, "remark", "remarks", "备注", "说明")
	records := make([]ImportErrorRecordInput, 0, len(rows)-headerRow-1)
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		currentRow := rowIndex + 1
		if len(records) >= 50000 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel rows exceed error import limit.", map[string]int{"limit": 50000})
		}
		orderNo := excelOptionalCell(row, orderIndex, hasOrder)
		difficulty := excelOptionalCell(row, difficultyIndex, hasDifficulty)
		payeeName := excelOptionalCell(row, payeeNameIndex, hasPayeeName)
		errorRaw := excelOptionalCell(row, errorIndex, hasErrorCount)
		issue := excelOptionalCell(row, issueIndex, hasIssue)
		isQualityRow := difficulty != "" || payeeName != "" || issue != ""
		if orderNo == "" && difficulty == "" && payeeName == "" && errorRaw == "" && issue == "" {
			continue
		}
		if orderNo == "" && !isQualityRow {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "order_no or quality import columns are required.", map[string]int{"row": currentRow})
		}
		errorCount := 1
		if errorRaw != "" {
			value, appErr := parseExcelNonNegativeInt(errorRaw, "error_count", currentRow)
			if appErr != nil {
				return nil, appErr
			}
			errorCount = value
		}
		var payeeUserID *int64
		if hasPayee {
			payeeRaw := strings.TrimSpace(excelCell(row, payeeIndex))
			if payeeRaw != "" {
				value, appErr := parseExcelNonNegativeInt64(payeeRaw, "payee_user_id", currentRow)
				if appErr != nil {
					return nil, appErr
				}
				payeeUserID = &value
			}
		}
		rawPayload := map[string]interface{}{
			"row":   currentRow,
			"日期":    excelOptionalCell(row, dateIndex, hasDate),
			"线上订单号": orderNo,
			"分类":    difficulty,
			"出错人":   payeeName,
			"问题描述":  issue,
			"抽查/售后": excelOptionalCell(row, sourceIndex, hasSource),
			"处理方法":  excelOptionalCell(row, methodIndex, hasMethod),
			"登记人":   excelOptionalCell(row, reporterIndex, hasReporter),
			"备注":    excelOptionalCell(row, remarkIndex, hasRemark),
			"出错数":   errorCount,
		}
		records = append(records, ImportErrorRecordInput{
			PayeeUserID:      payeeUserID,
			PayeeName:        payeeName,
			OrderNo:          orderNo,
			DifficultyClass:  difficulty,
			OccurredDate:     excelOptionalCell(row, dateIndex, hasDate),
			ErrorCount:       errorCount,
			IssueDescription: issue,
			SourceType:       excelOptionalCell(row, sourceIndex, hasSource),
			HandlingMethod:   excelOptionalCell(row, methodIndex, hasMethod),
			ReporterName:     excelOptionalCell(row, reporterIndex, hasReporter),
			Remark:           excelOptionalCell(row, remarkIndex, hasRemark),
			RawPayload:       mustJSON(rawPayload),
		})
	}
	if len(records) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no valid error rows.", nil)
	}
	return records, nil
}

type settlementSupplementExcelRow struct {
	row    int
	params CreateSettlementSupplementParams
}

type submissionItemQCExcelRow struct {
	row          int
	itemID       int64
	fileID       int64
	submissionNo string
	filename     string
	orderNo      string
	qcStatus     string
	reason       string
}

func parseSubmissionItemQCExcel(businessMonth string, reader io.Reader) ([]submissionItemQCExcelRow, *domain.AppError) {
	if strings.TrimSpace(businessMonth) != "" {
		if _, err := time.Parse("2006-01", strings.TrimSpace(businessMonth)); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month must use YYYY-MM.", nil)
		}
	}
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read QC Excel file.", err.Error())
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no readable sheet.", nil)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read QC Excel rows.", err.Error())
	}
	if len(rows) < 2 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file must include a header row and at least one data row.", nil)
	}
	headers := map[string]int{}
	for index, cell := range rows[0] {
		headers[normalizeExcelHeader(cell)] = index
	}
	itemIDIndex, hasItemID := firstExcelColumn(headers, "item_id", "itemid", "明细id", "计件明细id")
	fileIDIndex, hasFileID := firstExcelColumn(headers, "file_id", "fileid", "文件id", "素材id", "作品id")
	submissionIndex, hasSubmission := firstExcelColumn(headers, "submission_no", "submissionno", "提交编号", "提交单号", "批次号")
	filenameIndex, hasFilename := firstExcelColumn(headers, "filename", "file_name", "original_filename", "文件名", "作品名称", "素材名称")
	orderIndex, hasOrder := firstExcelColumn(headers, "order_no", "orderno", "订单号", "单号")
	statusIndex, ok := firstExcelColumn(headers, "qc_status", "qcstatus", "status", "质检状态", "状态")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing qc_status column.", nil)
	}
	reasonIndex, hasReason := firstExcelColumn(headers, "reason", "message", "驳回原因", "原因", "备注")
	parsed := make([]submissionItemQCExcelRow, 0, len(rows)-1)
	for rowIndex, row := range rows[1:] {
		currentRow := rowIndex + 2
		itemIDRaw := ""
		if hasItemID {
			itemIDRaw = strings.TrimSpace(excelCell(row, itemIDIndex))
		}
		fileIDRaw := ""
		if hasFileID {
			fileIDRaw = strings.TrimSpace(excelCell(row, fileIDIndex))
		}
		submissionNo := excelOptionalCell(row, submissionIndex, hasSubmission)
		filename := excelOptionalCell(row, filenameIndex, hasFilename)
		orderNo := ""
		if hasOrder {
			orderNo = strings.TrimSpace(excelCell(row, orderIndex))
		}
		status := normalizeQCStatusForImport(excelCell(row, statusIndex))
		reason := ""
		if hasReason {
			reason = strings.TrimSpace(excelCell(row, reasonIndex))
		}
		if itemIDRaw == "" && fileIDRaw == "" && submissionNo == "" && filename == "" && orderNo == "" && status == "" && reason == "" {
			continue
		}
		var itemID int64
		if itemIDRaw != "" {
			value, appErr := parseExcelNonNegativeInt64(itemIDRaw, "item_id", currentRow)
			if appErr != nil {
				return nil, appErr
			}
			itemID = value
		}
		var fileID int64
		if fileIDRaw != "" {
			value, appErr := parseExcelNonNegativeInt64(fileIDRaw, "file_id", currentRow)
			if appErr != nil {
				return nil, appErr
			}
			fileID = value
		}
		if status == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "qc_status must be submitted, checked or needs_fix.", map[string]int{"row": currentRow})
		}
		parsed = append(parsed, submissionItemQCExcelRow{
			row:          currentRow,
			itemID:       itemID,
			fileID:       fileID,
			submissionNo: submissionNo,
			filename:     filename,
			orderNo:      orderNo,
			qcStatus:     status,
			reason:       reason,
		})
	}
	if len(parsed) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no valid QC rows.", nil)
	}
	return parsed, nil
}

func parseSettlementSupplementExcel(businessMonth string, reader io.Reader) ([]settlementSupplementExcelRow, *domain.AppError) {
	businessMonth = strings.TrimSpace(businessMonth)
	if _, err := time.Parse("2006-01", businessMonth); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "business_month must use YYYY-MM.", nil)
	}
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read supplement Excel file.", err.Error())
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no readable sheet.", nil)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Failed to read supplement Excel rows.", err.Error())
	}
	if len(rows) < 2 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file must include a header row and at least one data row.", nil)
	}
	headers := map[string]int{}
	for index, cell := range rows[0] {
		headers[normalizeExcelHeader(cell)] = index
	}
	payeeIndex, ok := firstExcelColumn(headers, "payee_user_id", "payeeuserid", "user_id", "userid", "人员id", "用户id", "员工id")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing payee_user_id column.", nil)
	}
	orderIndex, ok := firstExcelColumn(headers, "order_no", "orderno", "订单号", "单号")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing order_no column.", nil)
	}
	difficultyIndex, ok := firstExcelColumn(headers, "difficulty_class", "difficulty", "难度")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing difficulty_class column.", nil)
	}
	pageIndex, ok := firstExcelColumn(headers, "page_count", "pagecount", "pages", "页数", "作图量")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing page_count column.", nil)
	}
	amountIndex, ok := firstExcelColumn(headers, "gross_amount", "grossamount", "amount", "补录金额", "金额")
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file is missing gross_amount column.", nil)
	}
	supplementDateIndex, hasSupplementDate := firstExcelColumn(headers, "supplement_date", "supplementdate", "补录日期", "想补日期", "日期")
	finalizedIndex, hasFinalized := firstExcelColumn(headers, "finalized", "定稿", "是否定稿")
	parsed := make([]settlementSupplementExcelRow, 0, len(rows)-1)
	for rowIndex, row := range rows[1:] {
		if len(parsed) >= 50000 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel rows exceed supplement import limit.", map[string]int{"limit": 50000})
		}
		payeeRaw := strings.TrimSpace(excelCell(row, payeeIndex))
		orderNo := strings.TrimSpace(excelCell(row, orderIndex))
		difficulty := strings.TrimSpace(excelCell(row, difficultyIndex))
		pageRaw := strings.TrimSpace(excelCell(row, pageIndex))
		amountRaw := strings.TrimSpace(excelCell(row, amountIndex))
		if payeeRaw == "" && orderNo == "" && difficulty == "" && pageRaw == "" && amountRaw == "" {
			continue
		}
		currentRow := rowIndex + 2
		payeeUserID, appErr := parseExcelNonNegativeInt64(payeeRaw, "payee_user_id", currentRow)
		if appErr != nil {
			return nil, appErr
		}
		pageCount, appErr := parseExcelNonNegativeInt(pageRaw, "page_count", currentRow)
		if appErr != nil {
			return nil, appErr
		}
		grossAmount, appErr := parseExcelNonNegativeDecimal(amountRaw, "gross_amount", currentRow)
		if appErr != nil {
			return nil, appErr
		}
		finalized := true
		if hasFinalized {
			finalized = parseExcelBoolDefault(excelCell(row, finalizedIndex), true)
		}
		supplementDate := ""
		if hasSupplementDate {
			supplementDate = strings.TrimSpace(excelCell(row, supplementDateIndex))
		}
		parsed = append(parsed, settlementSupplementExcelRow{
			row: currentRow,
			params: CreateSettlementSupplementParams{
				PayeeUserID:     payeeUserID,
				BusinessMonth:   businessMonth,
				OrderNo:         orderNo,
				SupplementDate:  supplementDate,
				DifficultyClass: difficulty,
				Finalized:       finalized,
				PageCount:       pageCount,
				GrossAmount:     grossAmount,
				Status:          domain.AssetWorkbenchSupplementStatusApproved,
			},
		})
	}
	if len(parsed) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel file has no valid supplement rows.", nil)
	}
	return parsed, nil
}

func normalizeQCStatusForImport(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case domain.AssetWorkbenchSubmissionStatusSubmitted, "待检", "待质检":
		return domain.AssetWorkbenchSubmissionStatusSubmitted
	case domain.AssetWorkbenchSubmissionStatusChecked, "通过", "合格", "已通过":
		return domain.AssetWorkbenchSubmissionStatusChecked
	case domain.AssetWorkbenchSubmissionStatusNeedsFix, "需修", "驳回", "不合格", "退回":
		return domain.AssetWorkbenchSubmissionStatusNeedsFix
	default:
		return ""
	}
}

func findErrorImportHeaderRow(rows [][]string) (int, map[string]int, bool) {
	limit := len(rows)
	if limit > 10 {
		limit = 10
	}
	for rowIndex := 0; rowIndex < limit; rowIndex++ {
		headers := map[string]int{}
		for index, cell := range rows[rowIndex] {
			key := normalizeExcelHeader(cell)
			if key == "" {
				continue
			}
			headers[key] = index
		}
		_, hasError := firstExcelColumn(headers, "error_count", "errorcount", "errors", "出错数", "错误数", "出错数量", "错误件数", "错误张数", "出错张数")
		_, hasPayee := firstExcelColumn(headers, "payee_name", "payeename", "出错人", "人员", "姓名", "计件人")
		_, hasDifficulty := firstExcelColumn(headers, "difficulty_class", "difficultyclass", "分类", "出错分类", "难度", "难度类", "难度类别")
		if hasError || hasPayee || hasDifficulty {
			return rowIndex, headers, true
		}
	}
	return 0, nil, false
}

func normalizeExcelHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func firstExcelColumn(headers map[string]int, names ...string) (int, bool) {
	for _, name := range names {
		if index, ok := headers[normalizeExcelHeader(name)]; ok {
			return index, true
		}
	}
	return 0, false
}

func excelCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func excelOptionalCell(row []string, index int, ok bool) string {
	if !ok {
		return ""
	}
	return strings.TrimSpace(excelCell(row, index))
}

func parseErrorImportDate(raw string, loc *time.Location) (*time.Time, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if loc == nil {
		loc = time.UTC
	}
	layouts := []string{"2006-01-02", "2006/01/02", "2006.01.02", "2006-1-2", "2006/1/2", "2006.1.2"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			value := truncateDate(t)
			return &value, nil
		}
	}
	if serial, err := strconv.ParseFloat(raw, 64); err == nil && serial > 0 {
		days := int(serial)
		if float64(days) == serial {
			value := time.Date(1899, 12, 30, 0, 0, 0, 0, loc).AddDate(0, 0, days)
			return &value, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "occurred_date must use YYYY-MM-DD or Excel date serial.", nil)
}

func parseExcelNonNegativeInt(raw string, field string, row int) (int, *domain.AppError) {
	value, appErr := parseExcelNonNegativeFloat(raw, field, row)
	if appErr != nil {
		return 0, appErr
	}
	return int(value), nil
}

func parseExcelNonNegativeInt64(raw string, field string, row int) (int64, *domain.AppError) {
	value, appErr := parseExcelNonNegativeFloat(raw, field, row)
	if appErr != nil {
		return 0, appErr
	}
	return int64(value), nil
}

func parseExcelNonNegativeFloat(raw string, field string, row int) (float64, *domain.AppError) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be a non-negative number.", map[string]int{"row": row})
	}
	if value != float64(int64(value)) {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be an integer.", map[string]int{"row": row})
	}
	return value, nil
}

func parseExcelNonNegativeDecimal(raw string, field string, row int) (float64, *domain.AppError) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be a non-negative number.", map[string]int{"row": row})
	}
	return value, nil
}

func parseExcelBoolDefault(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "否", "不", "no", "n", "未定稿":
		return false
	case "1", "true", "是", "已定稿", "定稿", "yes", "y":
		return true
	default:
		return fallback
	}
}

func truncateDate(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func truncateOptionalDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := truncateDate(*t)
	return &value
}

func formatOptionalDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mustJSON(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func mapRepoReadError(err error, notFoundMessage, internalMessage string) *domain.AppError {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeNotFound, notFoundMessage, nil)
	}
	return domain.NewAppError(domain.ErrCodeInternalError, internalMessage, err.Error())
}

func asAppError(err error) *domain.AppError {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

func actorHasAny(actor domain.RequestActor, roles ...domain.Role) bool {
	return domain.ActorHasAnyRole(actor, roles)
}

func isAssetWorkbenchAdmin(actor domain.RequestActor) bool {
	return actorHasAny(actor,
		domain.RoleAssetManager,
		domain.RoleAssetTemplateAdmin,
		domain.RoleAssetSettlement,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
	)
}

func assetWorkbenchRolesForService() []domain.Role {
	return []domain.Role{
		domain.RoleAssetSubmitter,
		domain.RoleAssetManager,
		domain.RoleAssetTemplateAdmin,
		domain.RoleAssetSettlement,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
	}
}

func assetWorkbenchCapabilities(actor domain.RequestActor) []string {
	actions := append([]string{}, actor.FrontendAccess.Actions...)
	if actorHasAny(actor, domain.RoleAssetSubmitter) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.submit",
			"asset.workbench.profile",
			"asset.workbench.material.download",
			"asset.workbench.settlement.self",
		)
	}
	if actorHasAny(actor, domain.RoleAssetManager) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.manage",
			"asset.workbench.group.manage",
			"asset.workbench.member.manage",
			"asset.workbench.system_search",
			"asset.workbench.download",
		)
	}
	if actorHasAny(actor, domain.RoleAssetTemplateAdmin) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.cost_center.manage",
		)
	}
	if actorHasAny(actor, domain.RoleAssetSettlement) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.settlement",
			"asset.workbench.export",
			"asset.workbench.profile.manage",
		)
	}
	if actorHasAny(actor, domain.RoleHRAdmin) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.profile.manage",
		)
	}
	if actorHasAny(actor, domain.RoleSuperAdmin) {
		actions = append(actions,
			"asset.workbench.bootstrap",
			"asset.workbench.submit",
			"asset.workbench.profile",
			"asset.workbench.material.download",
			"asset.workbench.settlement.self",
			"asset.workbench.profile.manage",
			"asset.workbench.manage",
			"asset.workbench.group.manage",
			"asset.workbench.member.manage",
			"asset.workbench.member.identity",
			"asset.workbench.system_search",
			"asset.workbench.download",
			"asset.workbench.cost_center.manage",
			"asset.workbench.settlement",
			"asset.workbench.export",
		)
	}
	return dedupeStrings(actions)
}

func applyAssetWorkbenchIdentity(current []domain.Role, identity string) []domain.Role {
	next := make([]domain.Role, 0, len(current)+4)
	for _, role := range domain.NormalizeRoleValues(current) {
		if isAssetWorkbenchRoleManagedByIdentity(role) {
			continue
		}
		next = append(next, role)
	}
	next = append(next, domain.RoleAssetSubmitter)
	if identity == "admin" {
		next = append(next,
			domain.RoleAssetManager,
			domain.RoleAssetTemplateAdmin,
			domain.RoleAssetSettlement,
		)
	}
	return domain.NormalizeRoleValues(next)
}

func isAssetWorkbenchRoleManagedByIdentity(role domain.Role) bool {
	switch role {
	case domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement:
		return true
	default:
		return false
	}
}

func workbenchIdentityFromRoles(roles []domain.Role) string {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleHRAdmin, domain.RoleSuperAdmin:
			return "admin"
		}
	}
	return "normal"
}

func normalizeMembershipIdentityType(value string) string {
	switch strings.TrimSpace(value) {
	case domain.AppMembershipIdentityStaff, domain.AppMembershipIdentityExternal, domain.AppMembershipIdentityContractor:
		return strings.TrimSpace(value)
	default:
		return domain.AppMembershipIdentityStaff
	}
}

func assetRolesFromActor(actor domain.RequestActor) []domain.Role {
	return assetRolesFromRoles(actor.Roles)
}

func assetRolesFromRoles(roles []domain.Role) []domain.Role {
	out := make([]domain.Role, 0, 6)
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleHRAdmin, domain.RoleSuperAdmin:
			out = append(out, role)
		}
	}
	return domain.NormalizeRoleValues(out)
}

func normalizeAssetWorkbenchRoleSet(roles []domain.Role, keepSubmitter bool) []domain.Role {
	out := make([]domain.Role, 0, len(roles)+1)
	if keepSubmitter {
		out = append(out, domain.RoleAssetSubmitter)
	}
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement:
			out = append(out, role)
		}
	}
	return domain.NormalizeRoleValues(out)
}

func mergeAssetWorkbenchRoles(current []domain.Role, assetRoles []domain.Role) []domain.Role {
	next := removeAssetWorkbenchRoles(current)
	next = append(next, normalizeAssetWorkbenchRoleSet(assetRoles, true)...)
	return domain.NormalizeRoleValues(next)
}

func removeAssetWorkbenchRoles(current []domain.Role) []domain.Role {
	next := make([]domain.Role, 0, len(current))
	for _, role := range domain.NormalizeRoleValues(current) {
		switch role {
		case domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement:
			continue
		default:
			next = append(next, role)
		}
	}
	return domain.NormalizeRoleValues(next)
}

func containsManagementAssetRole(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement:
			return true
		}
	}
	return false
}

func roleLabelsForActor(actor domain.RequestActor) []string {
	return roleLabelsForRoles(actor.Roles)
}

func roleLabelsForRoles(roles []domain.Role) []string {
	labels := make([]string, 0, len(roles))
	for _, role := range domain.NormalizeRoleValues(roles) {
		if label := assetWorkbenchRoleLabel(role); label != "" {
			labels = append(labels, label)
		}
	}
	return dedupeStrings(labels)
}

func assetWorkbenchRoleLabel(role domain.Role) string {
	switch role {
	case domain.RoleAssetSubmitter:
		return "交付人员"
	case domain.RoleAssetManager:
		return "作品管理"
	case domain.RoleAssetTemplateAdmin:
		return "计价配置"
	case domain.RoleAssetSettlement:
		return "结算财务"
	case domain.RoleSuperAdmin:
		return "超级管理员"
	case domain.RoleHRAdmin:
		return "人员档案"
	default:
		return ""
	}
}

func decorateMemberForActor(member *domain.AssetWorkbenchMember, actor domain.RequestActor) {
	if member == nil {
		return
	}
	member.RoleLabels = roleLabelsForRoles(member.Roles)
	member.CanEditRoles = member.Status == domain.AppMembershipStatusActive && actorHasAny(actor, domain.RoleSuperAdmin)
}

func (s *Service) loadMemberByID(ctx context.Context, actor domain.RequestActor, userID int64) (*domain.AssetWorkbenchMember, *domain.AppError) {
	items, _, err := s.repo.ListMembers(ctx, repo.AssetWorkbenchMemberFilter{UserID: userID, Page: 1, PageSize: 1})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load asset workbench member.", err.Error())
	}
	for _, item := range items {
		if item != nil && item.UserID == userID {
			decorateMemberForActor(item, actor)
			return item, nil
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeNotFound, "Asset workbench member not found.", nil)
}

func (s *Service) appendIdentityEvent(ctx context.Context, tx repo.Tx, actorID, targetID int64, action string, before interface{}, after interface{}, reason string) error {
	var actorPtr, targetPtr *int64
	if actorID > 0 {
		actorCopy := actorID
		actorPtr = &actorCopy
	}
	if targetID > 0 {
		targetCopy := targetID
		targetPtr = &targetCopy
	}
	_, err := s.repo.CreateAppIdentityEvent(ctx, tx, &domain.AppIdentityEvent{
		ActorUserID:  actorPtr,
		TargetUserID: targetPtr,
		SourceApp:    "main_ops",
		TargetApp:    domain.AssetWorkbenchAppCode,
		Action:       action,
		Before:       mustJSON(before),
		After:        mustJSON(after),
		Reason:       strings.TrimSpace(reason),
	})
	return err
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func positiveUniqueInt64s(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
