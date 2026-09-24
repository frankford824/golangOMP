package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const AssetMediaRecipe = "bounded-webp-v2"

type AssetMediaOptions struct {
	Delivery               string
	ExpectedVersion        string
	Rendition              string
	AuthorizeOnly          bool
	AccessReference        *AssetMediaAccessReference
	WorkbenchPreviewWorker string
	WorkbenchPreviewObject string
}

type AssetMediaAccessReference struct {
	ResourceKind          string `json:"resource_kind"`
	ResourceID            string `json:"resource_id"`
	ItemID                int64  `json:"item_id,omitempty"`
	Purpose               string `json:"purpose"`
	Rendition             string `json:"rendition"`
	ExpectedSourceVersion string `json:"expected_source_version"`
}
type assetMediaOptionsKey struct{}

func WithAssetMediaOptions(ctx context.Context, options AssetMediaOptions) context.Context {
	return context.WithValue(ctx, assetMediaOptionsKey{}, options)
}
func AssetMediaOptionsFromContext(ctx context.Context) AssetMediaOptions {
	options, _ := ctx.Value(assetMediaOptionsKey{}).(AssetMediaOptions)
	if options.Delivery == "" {
		options.Delivery = "auto"
	}
	return options
}

// AssetMediaJob is restricted to asset processing. Payloads contain immutable
// identifiers, never presigned URLs, account keys or user session tokens.
type AssetMediaJob struct {
	ID             string          `json:"job_id"`
	DedupeKey      string          `json:"-"`
	RefreshMissing bool            `json:"-"`
	Kind           string          `json:"kind"`
	Pool           string          `json:"pool"`
	ResourceID     string          `json:"resource_id"`
	Filename       string          `json:"filename,omitempty"`
	SourceVersion  string          `json:"source_version"`
	Recipe         string          `json:"recipe"`
	State          string          `json:"state"`
	Phase          string          `json:"phase"`
	Priority       int             `json:"priority"`
	Attempts       int             `json:"attempts"`
	NextRunAt      time.Time       `json:"next_run_at"`
	LeaseOwner     string          `json:"-"`
	LeaseEpoch     int64           `json:"lease_epoch,omitempty"`
	LeaseExpiresAt *time.Time      `json:"-"`
	ProcessedBytes int64           `json:"processed_bytes"`
	TotalBytes     int64           `json:"total_bytes"`
	Payload        json.RawMessage `json:"-"`
	Checkpoint     json.RawMessage `json:"-"`
	Result         json.RawMessage `json:"result,omitempty"`
	ErrorCode      string          `json:"error_code,omitempty"`
	Retryable      bool            `json:"retryable"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type AssetMediaRequest struct {
	ID              string                     `json:"request_id"`
	ActorID         int64                      `json:"-"`
	Job             *AssetMediaJob             `json:"job"`
	Cancelled       bool                       `json:"cancelled"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	AccessReference *AssetMediaAccessReference `json:"access_reference,omitempty"`
}

type AssetMediaJobInput struct {
	Filename     string                  `json:"filename,omitempty"`
	TaskID       int64                   `json:"task_id,omitempty"`
	AssetID      int64                   `json:"asset_id,omitempty"`
	TaskAssetID  int64                   `json:"task_asset_id,omitempty"`
	ExternalID   int64                   `json:"external_asset_id,omitempty"`
	ActorID      int64                   `json:"actor_id,omitempty"`
	Selection    []AssetMediaSelection   `json:"selection,omitempty"`
	ParentJobID  string                  `json:"parent_job_id,omitempty"`
	Verification *AssetMediaVerification `json:"verification,omitempty"`
}

type AssetMediaVerification struct {
	ObjectKey string `json:"object_key"`
	ETag      string `json:"etag"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
}

type AssetMediaSelection struct {
	ResourceID    string `json:"resource_id"`
	SourceVersion string `json:"source_version"`
	Filename      string `json:"filename"`
	Size          int64  `json:"size"`
}

type AssetMediaDelivery struct {
	State     string     `json:"state"`
	URL       string     `json:"url,omitempty"`
	GatewayID string     `json:"gateway_id,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func AssetMediaIdentity(parts ...string) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TaskAssetSourceVersion(id int64) string { return "ta:" + strconv.FormatInt(id, 10) }

func (j *AssetMediaJob) SetIdentity() {
	j.Recipe = strings.TrimSpace(j.Recipe)
	j.DedupeKey = AssetMediaIdentity(j.Kind, j.ResourceID, j.SourceVersion, j.Recipe)
}

// PublicState deliberately excludes SQL scheduling details.
func (j *AssetMediaJob) PublicState() string {
	if j == nil {
		return "queued"
	}
	switch j.State {
	case "succeeded":
		return "ready"
	case "processing":
		return "processing"
	case "failed", "stale":
		return "failed"
	default:
		return "queued"
	}
}
