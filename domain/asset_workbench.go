package domain

import (
	"encoding/json"
	"time"
)

const (
	AssetWorkbenchItemTypeGrossPiecework = "gross_piecework"
	AssetWorkbenchItemTypeErrorDeduction = "error_deduction"
	AssetWorkbenchItemTypeWelfare        = "welfare"
	AssetWorkbenchItemTypeSupplement     = "supplement"
	AssetWorkbenchItemTypeAdjustment     = "adjustment"
	AssetWorkbenchItemTypeReversal       = "reversal"

	AssetWorkbenchPayrollRowTypeNormalPiecework     = "normal_piecework"
	AssetWorkbenchPayrollRowTypeSupplementPiecework = "supplement_piecework"

	AssetWorkbenchAssignmentTargetUser  = "user"
	AssetWorkbenchAssignmentTargetGroup = "group"
)

func DefaultAssetWorkbenchSettlementItemTypes() []string {
	return []string{
		AssetWorkbenchItemTypeGrossPiecework,
		AssetWorkbenchItemTypeErrorDeduction,
		AssetWorkbenchItemTypeWelfare,
		AssetWorkbenchItemTypeSupplement,
		AssetWorkbenchItemTypeAdjustment,
		AssetWorkbenchItemTypeReversal,
	}
}

const (
	AssetWorkbenchWorkerTypeFulltime = "fulltime"
	AssetWorkbenchWorkerTypeParttime = "parttime"
	AssetWorkbenchWorkerTypeAll      = "all"

	AssetWorkbenchProfileStatusPending   = "pending"
	AssetWorkbenchProfileStatusActive    = "active"
	AssetWorkbenchProfileStatusSuspended = "suspended"

	AssetWorkbenchUploadStatusCreated   = "created"
	AssetWorkbenchUploadStatusUploading = "uploading"
	AssetWorkbenchUploadStatusUploaded  = "uploaded"
	AssetWorkbenchUploadStatusCancelled = "cancelled"
	AssetWorkbenchUploadStatusExpired   = "expired"
	AssetWorkbenchUploadStatusSubmitted = "submitted"

	AssetWorkbenchSubmissionStatusSubmitted = "submitted"
	AssetWorkbenchSubmissionStatusChecked   = "checked"
	AssetWorkbenchSubmissionStatusNeedsFix  = "needs_fix"
	AssetWorkbenchSubmissionStatusVoided    = "voided"

	AssetWorkbenchPricingStatusPriced       = "priced"
	AssetWorkbenchPricingStatusUnpriced     = "unpriced"
	AssetWorkbenchPricingStatusPendingGrade = "pending_grade"

	AssetWorkbenchSettlementStatusUnsettled = "unsettled"
	AssetWorkbenchSettlementStatusInBatch   = "in_batch"
	AssetWorkbenchSettlementStatusSettled   = "settled"
	AssetWorkbenchSettlementStatusReversed  = "reversed"

	AssetWorkbenchPreviewStatusPending       = "pending"
	AssetWorkbenchPreviewStatusProcessing    = "processing"
	AssetWorkbenchPreviewStatusReady         = "ready"
	AssetWorkbenchPreviewStatusFailed        = "failed"
	AssetWorkbenchPreviewStatusNotApplicable = "not_applicable"

	AssetWorkbenchBatchStatusGenerated = "generated"
	AssetWorkbenchBatchStatusConfirmed = "confirmed"
	AssetWorkbenchBatchStatusCancelled = "cancelled"
	AssetWorkbenchBatchStatusReversed  = "reversed"

	AssetWorkbenchPromoModeFixedPrice   = "fixed_price"
	AssetWorkbenchPromoModeMarkupAmount = "markup_amount"
	AssetWorkbenchPromoModeMarkupRate   = "markup_rate"

	AssetWorkbenchSupplementStatusDraft    = "draft"
	AssetWorkbenchSupplementStatusApproved = "approved"
	AssetWorkbenchSupplementStatusInBatch  = "in_batch"
	AssetWorkbenchSupplementStatusSettled  = "settled"
	AssetWorkbenchSupplementStatusVoided   = "voided"

	AssetWorkbenchErrorMatchStatusMatched   = "matched"
	AssetWorkbenchErrorMatchStatusUnmatched = "unmatched"
	AssetWorkbenchErrorMatchStatusAmbiguous = "ambiguous"

	AssetWorkbenchAdjustmentTypeAdjustment = "adjustment"
	AssetWorkbenchAdjustmentTypeReversal   = "reversal"
	AssetWorkbenchAdjustmentStatusApplied  = "applied"

	AssetWorkbenchEventProfileUpserted             = "profile.upserted"
	AssetWorkbenchEventPriceCreated                = "price.created"
	AssetWorkbenchEventDeductionCreated            = "deduction.created"
	AssetWorkbenchEventWelfareCreated              = "welfare.created"
	AssetWorkbenchEventPromoCreated                = "promo.created"
	AssetWorkbenchEventUploadSessionCreated        = "upload_session.created"
	AssetWorkbenchEventUploadSessionUpdated        = "upload_session.updated"
	AssetWorkbenchEventSubmissionCreated           = "submission.created"
	AssetWorkbenchEventErrorImportCreated          = "error_import.created"
	AssetWorkbenchEventSettlementGenerated         = "settlement.generated"
	AssetWorkbenchEventSettlementConfirmed         = "settlement.confirmed"
	AssetWorkbenchEventSettlementCancelled         = "settlement.cancelled"
	AssetWorkbenchEventSettlementAdjusted          = "settlement.adjusted"
	AssetWorkbenchEventSupplementCreated           = "supplement.created"
	AssetWorkbenchEventSupplementPermissionChanged = "supplement_permission.changed"
	AssetWorkbenchEventSavedViewUpserted           = "saved_view.upserted"
	AssetWorkbenchEventFileDownloaded              = "file.downloaded"
	AssetWorkbenchEventFileBatchDownloaded         = "file.batch_downloaded"
	AssetWorkbenchEventSystemAssetDownloaded       = "system_asset.downloaded"
	AssetWorkbenchEventSystemAssetBatchDownloaded  = "system_asset.batch_downloaded"
	AssetWorkbenchEventItemQCUpdated               = "item.qc_updated"
	AssetWorkbenchEventItemVoided                  = "item.voided"
	AssetWorkbenchEventItemRepriced                = "item.repriced"
	AssetWorkbenchEventGroupUpserted               = "group.upserted"
	AssetWorkbenchEventTemplateUpserted            = "template.upserted"
	AssetWorkbenchEventTemplateAssigned            = "template.assigned"
	AssetWorkbenchEventTemplateAssignmentRemoved   = "template_assignment.removed"

	AssetWorkbenchEntityProfile              = "profile"
	AssetWorkbenchEntityPriceMatrix          = "price_matrix"
	AssetWorkbenchEntityDeductionRule        = "deduction_rule"
	AssetWorkbenchEntityWelfareRule          = "welfare_rule"
	AssetWorkbenchEntityPromoCoupon          = "promo_coupon"
	AssetWorkbenchEntityUploadSession        = "upload_session"
	AssetWorkbenchEntitySubmission           = "submission"
	AssetWorkbenchEntitySubmissionItem       = "submission_item"
	AssetWorkbenchEntitySubmissionFile       = "submission_file"
	AssetWorkbenchEntitySystemAsset          = "system_asset"
	AssetWorkbenchEntityErrorImport          = "error_import"
	AssetWorkbenchEntitySettlement           = "settlement_batch"
	AssetWorkbenchEntityAdjustment           = "settlement_adjustment"
	AssetWorkbenchEntitySupplement           = "settlement_supplement"
	AssetWorkbenchEntitySupplementPermission = "supplement_permission"
	AssetWorkbenchEntitySavedView            = "saved_view"
	AssetWorkbenchEntityGroup                = "group"
	AssetWorkbenchEntityTemplate             = "template"
	AssetWorkbenchEntityTemplateAssignment   = "template_assignment"
)

type AssetWorkbenchProfile struct {
	ID            int64      `json:"id" db:"id"`
	UserID        int64      `json:"user_id" db:"user_id"`
	WorkerType    string     `json:"worker_type" db:"worker_type"`
	JobGrade      string     `json:"job_grade" db:"job_grade"`
	RealName      string     `json:"real_name" db:"real_name"`
	Phone         *string    `json:"phone,omitempty" db:"phone"`
	Province      string     `json:"province" db:"province"`
	City          string     `json:"city" db:"city"`
	IDCard        *string    `json:"id_card,omitempty" db:"id_card"`
	Gender        string     `json:"gender" db:"gender"`
	AlipayAccount string     `json:"alipay_account" db:"alipay_account"`
	OnboardedAt   *time.Time `json:"onboarded_at,omitempty" db:"onboarded_at"`
	GradeHidden   bool       `json:"grade_hidden" db:"grade_hidden"`
	Status        string     `json:"status" db:"status"`
	PIICompleted  bool       `json:"pii_completed" db:"pii_completed"`
	CreatedBy     *int64     `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy     *int64     `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchGradePeriod struct {
	ID            int64      `json:"id" db:"id"`
	ProfileID     int64      `json:"profile_id" db:"profile_id"`
	UserID        int64      `json:"user_id" db:"user_id"`
	WorkerType    string     `json:"worker_type" db:"worker_type"`
	JobGrade      string     `json:"job_grade" db:"job_grade"`
	EffectiveFrom time.Time  `json:"effective_from" db:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty" db:"effective_to"`
	ChangedBy     *int64     `json:"changed_by,omitempty" db:"changed_by"`
	Reason        string     `json:"reason" db:"reason"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

type AssetWorkbenchPriceMatrix struct {
	ID              int64      `json:"id" db:"id"`
	WorkerType      string     `json:"worker_type" db:"worker_type"`
	JobGrade        string     `json:"job_grade" db:"job_grade"`
	DifficultyClass string     `json:"difficulty_class" db:"difficulty_class"`
	UnitPrice       float64    `json:"unit_price" db:"unit_price"`
	EffectiveFrom   time.Time  `json:"effective_from" db:"effective_from"`
	EffectiveTo     *time.Time `json:"effective_to,omitempty" db:"effective_to"`
	Enabled         bool       `json:"enabled" db:"enabled"`
	RevisionNo      int        `json:"revision_no" db:"revision_no"`
	CreatedBy       int64      `json:"created_by" db:"created_by"`
	Remark          string     `json:"remark" db:"remark"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchDeductionRule struct {
	ID              int64      `json:"id" db:"id"`
	WorkerType      string     `json:"worker_type" db:"worker_type"`
	JobGrade        string     `json:"job_grade" db:"job_grade"`
	DifficultyClass string     `json:"difficulty_class" db:"difficulty_class"`
	DeductionAmount float64    `json:"deduction_amount" db:"deduction_amount"`
	EffectiveFrom   time.Time  `json:"effective_from" db:"effective_from"`
	EffectiveTo     *time.Time `json:"effective_to,omitempty" db:"effective_to"`
	Enabled         bool       `json:"enabled" db:"enabled"`
	RevisionNo      int        `json:"revision_no" db:"revision_no"`
	CreatedBy       int64      `json:"created_by" db:"created_by"`
	Remark          string     `json:"remark" db:"remark"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchWelfareRule struct {
	ID            int64           `json:"id" db:"id"`
	RuleName      string          `json:"rule_name" db:"rule_name"`
	WorkerType    string          `json:"worker_type" db:"worker_type"`
	JobGrade      string          `json:"job_grade" db:"job_grade"`
	RuleType      string          `json:"rule_type" db:"rule_type"`
	Amount        float64         `json:"amount" db:"amount"`
	Config        json.RawMessage `json:"config_json,omitempty" db:"config_json"`
	EffectiveFrom time.Time       `json:"effective_from" db:"effective_from"`
	EffectiveTo   *time.Time      `json:"effective_to,omitempty" db:"effective_to"`
	Enabled       bool            `json:"enabled" db:"enabled"`
	CreatedBy     int64           `json:"created_by" db:"created_by"`
	Remark        string          `json:"remark" db:"remark"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchPromoCoupon struct {
	ID              int64           `json:"id" db:"id"`
	CouponCode      string          `json:"coupon_code" db:"coupon_code"`
	CouponName      string          `json:"coupon_name" db:"coupon_name"`
	Mode            string          `json:"mode" db:"mode"`
	Amount          *float64        `json:"amount,omitempty" db:"amount"`
	Percent         *float64        `json:"percent,omitempty" db:"percent"`
	Priority        int             `json:"priority" db:"priority"`
	WorkerType      string          `json:"worker_type" db:"worker_type"`
	JobGrade        string          `json:"job_grade" db:"job_grade"`
	DifficultyClass string          `json:"difficulty_class" db:"difficulty_class"`
	EligibleUserIDs json.RawMessage `json:"eligible_user_ids_json,omitempty" db:"eligible_user_ids_json"`
	EligibleCodes   json.RawMessage `json:"eligible_codes_json,omitempty" db:"eligible_codes_json"`
	EffectiveFrom   time.Time       `json:"effective_from" db:"effective_from"`
	EffectiveTo     *time.Time      `json:"effective_to,omitempty" db:"effective_to"`
	Enabled         bool            `json:"enabled" db:"enabled"`
	StackPolicy     string          `json:"stack_policy" db:"stack_policy"`
	CreatedBy       int64           `json:"created_by" db:"created_by"`
	Remark          string          `json:"remark" db:"remark"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchGroup struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchGroupMember struct {
	GroupID   int64     `json:"group_id" db:"group_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type AssetWorkbenchTemplate struct {
	ID              int64     `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	Category        string    `json:"category" db:"category"`
	DifficultyClass string    `json:"difficulty_class" db:"difficulty_class"`
	WorkerType      string    `json:"worker_type" db:"worker_type"`
	Enabled         bool      `json:"enabled" db:"enabled"`
	SortOrder       int       `json:"sort_order" db:"sort_order"`
	CreatedBy       int64     `json:"created_by" db:"created_by"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchTemplateAssignment struct {
	ID         int64     `json:"id" db:"id"`
	TemplateID int64     `json:"template_id" db:"template_id"`
	TargetType string    `json:"target_type" db:"target_type"`
	TargetID   int64     `json:"target_id" db:"target_id"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	AssignedBy int64     `json:"assigned_by" db:"assigned_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchUploadSession struct {
	ID               int64           `json:"id" db:"id"`
	SessionID        string          `json:"session_id" db:"session_id"`
	OwnerUserID      int64           `json:"owner_user_id" db:"owner_user_id"`
	Status           string          `json:"status" db:"status"`
	ObjectKey        string          `json:"object_key" db:"object_key"`
	OriginalFilename string          `json:"original_filename" db:"original_filename"`
	FileSize         int64           `json:"file_size" db:"file_size"`
	MimeType         string          `json:"mime_type" db:"mime_type"`
	FileHash         string          `json:"file_hash" db:"file_hash"`
	UploadID         string          `json:"upload_id" db:"upload_id"`
	MultipartPlan    json.RawMessage `json:"multipart_plan_json,omitempty" db:"multipart_plan_json"`
	ExpiresAt        time.Time       `json:"expires_at" db:"expires_at"`
	UploadedAt       *time.Time      `json:"uploaded_at,omitempty" db:"uploaded_at"`
	CancelledAt      *time.Time      `json:"cancelled_at,omitempty" db:"cancelled_at"`
	SubmittedItemID  *int64          `json:"submitted_item_id,omitempty" db:"submitted_item_id"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSubmission struct {
	ID              int64     `json:"id" db:"id"`
	SubmissionNo    string    `json:"submission_no" db:"submission_no"`
	SubmitterUserID int64     `json:"submitter_user_id" db:"submitter_user_id"`
	BusinessMonth   string    `json:"business_month" db:"business_month"`
	SubmittedAt     time.Time `json:"submitted_at" db:"submitted_at"`
	Status          string    `json:"status" db:"status"`
	ItemCount       int       `json:"item_count" db:"item_count"`
	FileCount       int       `json:"file_count" db:"file_count"`
	PageCount       int       `json:"page_count" db:"page_count"`
	GrossTotal      float64   `json:"gross_total" db:"gross_total"`
	Notes           string    `json:"notes" db:"notes"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSubmissionItem struct {
	ID                       int64           `json:"id" db:"id"`
	SubmissionID             int64           `json:"submission_id" db:"submission_id"`
	PayeeUserID              int64           `json:"payee_user_id" db:"payee_user_id"`
	OrderNo                  string          `json:"order_no" db:"order_no"`
	TemplateID               *int64          `json:"template_id,omitempty" db:"template_id"`
	TemplateNameSnapshot     string          `json:"template_name_snapshot" db:"template_name_snapshot"`
	CategorySnapshot         string          `json:"category_snapshot" db:"category_snapshot"`
	DifficultyClass          string          `json:"difficulty_class" db:"difficulty_class"`
	Finalized                bool            `json:"finalized" db:"finalized"`
	PageCount                int             `json:"page_count" db:"page_count"`
	ItemCount                int             `json:"item_count" db:"item_count"`
	BusinessMonth            string          `json:"business_month" db:"business_month"`
	SubmittedAt              time.Time       `json:"submitted_at" db:"submitted_at"`
	WorkerTypeSnapshot       string          `json:"worker_type_snapshot" db:"worker_type_snapshot"`
	JobGradeSnapshot         string          `json:"job_grade_snapshot" db:"job_grade_snapshot"`
	BasePriceRuleID          *int64          `json:"base_price_rule_id,omitempty" db:"base_price_rule_id"`
	BaseUnitPrice            *float64        `json:"base_unit_price,omitempty" db:"base_unit_price"`
	PromoCouponID            *int64          `json:"promo_coupon_id,omitempty" db:"promo_coupon_id"`
	PromoSnapshot            json.RawMessage `json:"promo_snapshot_json,omitempty" db:"promo_snapshot_json"`
	PricingSnapshot          json.RawMessage `json:"pricing_snapshot_json,omitempty" db:"pricing_snapshot_json"`
	GrossAmount              float64         `json:"gross_amount" db:"gross_amount"`
	PricingStatus            string          `json:"pricing_status" db:"pricing_status"`
	QCStatus                 string          `json:"qc_status" db:"qc_status"`
	SettlementStatus         string          `json:"settlement_status" db:"settlement_status"`
	CurrentSettlementBatchID *int64          `json:"current_settlement_batch_id,omitempty" db:"current_settlement_batch_id"`
	VoidedAt                 *time.Time      `json:"voided_at,omitempty" db:"voided_at"`
	VoidedBy                 *int64          `json:"voided_by,omitempty" db:"voided_by"`
	VoidReason               string          `json:"void_reason" db:"void_reason"`
	CreatedAt                time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSubmissionFile struct {
	ID                    int64      `json:"id" db:"id"`
	SubmissionID          int64      `json:"submission_id" db:"submission_id"`
	SubmissionItemID      int64      `json:"submission_item_id" db:"submission_item_id"`
	UploadSessionID       *int64     `json:"upload_session_id,omitempty" db:"upload_session_id"`
	OwnerUserID           int64      `json:"owner_user_id" db:"owner_user_id"`
	ObjectKey             string     `json:"object_key" db:"object_key"`
	PreviewKey            string     `json:"preview_key" db:"preview_key"`
	PreviewStatus         string     `json:"preview_status" db:"preview_status"`
	PreviewAttempts       int        `json:"preview_attempts" db:"preview_attempts"`
	PreviewError          string     `json:"preview_error" db:"preview_error"`
	PreviewNextRetryAt    *time.Time `json:"preview_next_retry_at,omitempty" db:"preview_next_retry_at"`
	PreviewWorkerID       string     `json:"preview_worker_id" db:"preview_worker_id"`
	PreviewLeaseExpiresAt *time.Time `json:"preview_lease_expires_at,omitempty" db:"preview_lease_expires_at"`
	OriginalFilename      string     `json:"original_filename" db:"original_filename"`
	FileExt               string     `json:"file_ext" db:"file_ext"`
	FileType              string     `json:"file_type" db:"file_type"`
	MimeType              string     `json:"mime_type" db:"mime_type"`
	FileSize              int64      `json:"file_size" db:"file_size"`
	FileHash              string     `json:"file_hash" db:"file_hash"`
	SortOrder             int        `json:"sort_order" db:"sort_order"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSettlementBatch struct {
	ID               int64      `json:"id" db:"id"`
	BatchNo          string     `json:"batch_no" db:"batch_no"`
	BusinessMonth    string     `json:"business_month" db:"business_month"`
	Status           string     `json:"status" db:"status"`
	GeneratedBy      int64      `json:"generated_by" db:"generated_by"`
	ConfirmedBy      *int64     `json:"confirmed_by,omitempty" db:"confirmed_by"`
	CancelledBy      *int64     `json:"cancelled_by,omitempty" db:"cancelled_by"`
	GeneratedAt      time.Time  `json:"generated_at" db:"generated_at"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CancelReason     string     `json:"cancel_reason" db:"cancel_reason"`
	ItemCount        int        `json:"item_count" db:"item_count"`
	GrossAmount      float64    `json:"gross_amount" db:"gross_amount"`
	DeductionAmount  float64    `json:"deduction_amount" db:"deduction_amount"`
	WelfareAmount    float64    `json:"welfare_amount" db:"welfare_amount"`
	SupplementAmount float64    `json:"supplement_amount" db:"supplement_amount"`
	AdjustmentAmount float64    `json:"adjustment_amount" db:"adjustment_amount"`
	NetAmount        float64    `json:"net_amount" db:"net_amount"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}
