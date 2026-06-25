package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strconv"
	"strings"
	"time"
)

type AssetResourceSource string

const (
	AssetResourceSourceSystem   AssetResourceSource = "system"
	AssetResourceSourceExternal AssetResourceSource = "external"
	AssetResourceSourceAll      AssetResourceSource = "all"
)

func NormalizeAssetResourceSource(value string) AssetResourceSource {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(AssetResourceSourceSystem):
		return AssetResourceSourceSystem
	case string(AssetResourceSourceExternal):
		return AssetResourceSourceExternal
	default:
		return AssetResourceSourceAll
	}
}

type ExternalAssetKind string

const (
	ExternalAssetKindNetdisk  ExternalAssetKind = "netdisk"
	ExternalAssetKindNASLocal ExternalAssetKind = "nas_local"
)

type ExternalAssetStatus string

const (
	ExternalAssetStatusIndexed ExternalAssetStatus = "indexed"
	ExternalAssetStatusMissing ExternalAssetStatus = "missing"
)

type ExternalAssetOSSStatus string

const (
	ExternalAssetOSSStatusNone      ExternalAssetOSSStatus = "none"
	ExternalAssetOSSStatusPending   ExternalAssetOSSStatus = "pending"
	ExternalAssetOSSStatusUploading ExternalAssetOSSStatus = "uploading"
	ExternalAssetOSSStatusReady     ExternalAssetOSSStatus = "ready"
	ExternalAssetOSSStatusFailed    ExternalAssetOSSStatus = "failed"
)

type ExternalAssetPreviewStatus string

const (
	ExternalAssetPreviewStatusNone    ExternalAssetPreviewStatus = "none"
	ExternalAssetPreviewStatusPending ExternalAssetPreviewStatus = "pending"
	ExternalAssetPreviewStatusReady   ExternalAssetPreviewStatus = "ready"
	ExternalAssetPreviewStatusFailed  ExternalAssetPreviewStatus = "failed"
)

type ExternalAssetRecord struct {
	ID                int64                      `json:"id"`
	ResourceID        string                     `json:"resource_id"`
	Provider          string                     `json:"provider"`
	Kind              ExternalAssetKind          `json:"kind"`
	Driver            string                     `json:"driver"`
	MountPath         string                     `json:"mount_path"`
	OriginPath        string                     `json:"origin_path"`
	OriginPathHash    string                     `json:"origin_path_hash"`
	ParentPath        string                     `json:"parent_path"`
	FileName          string                     `json:"file_name"`
	FileExt           string                     `json:"file_ext,omitempty"`
	MimeType          string                     `json:"mime_type,omitempty"`
	FileSize          int64                      `json:"file_size"`
	IsDir             bool                       `json:"is_dir"`
	Status            ExternalAssetStatus        `json:"status"`
	RawURL            string                     `json:"-"`
	RawURLExpiresAt   *time.Time                 `json:"raw_url_expires_at,omitempty"`
	DirectURLStatus   string                     `json:"direct_url_status,omitempty"`
	OSSOriginalKey    string                     `json:"oss_original_key,omitempty"`
	OSSPreviewKey     string                     `json:"oss_preview_key,omitempty"`
	OSSThumbKey       string                     `json:"oss_thumb_key,omitempty"`
	OSSSyncStatus     ExternalAssetOSSStatus     `json:"oss_sync_status"`
	PreviewStatus     ExternalAssetPreviewStatus `json:"preview_status"`
	LastSeenAt        *time.Time                 `json:"last_seen_at,omitempty"`
	LastScannedAt     *time.Time                 `json:"last_scanned_at,omitempty"`
	LastLinkCheckedAt *time.Time                 `json:"last_link_checked_at,omitempty"`
	LastPrepareError  string                     `json:"last_prepare_error,omitempty"`
	SearchableText    string                     `json:"-"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
}

type ExternalAssetSearchQuery struct {
	Keyword        string
	Kind           ExternalAssetKind
	MountPath      string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	FormatCategory AssetFormatCategoryFilter
	Page           int
	Size           int
}

func (q ExternalAssetSearchQuery) Normalized() ExternalAssetSearchQuery {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.MountPath = strings.TrimSpace(q.MountPath)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > 100 {
		q.Size = 100
	}
	switch q.Kind {
	case ExternalAssetKindNetdisk, ExternalAssetKindNASLocal:
	default:
		q.Kind = ""
	}
	switch q.FormatCategory {
	case AssetFormatCategoryImage,
		AssetFormatCategoryDesign,
		AssetFormatCategoryPDF,
		AssetFormatCategoryVideo,
		AssetFormatCategoryArchive:
	default:
		q.FormatCategory = AssetFormatCategoryAll
	}
	return q
}

type ExternalAssetUpsert struct {
	Provider       string
	Kind           ExternalAssetKind
	Driver         string
	MountPath      string
	OriginPath     string
	ParentPath     string
	FileName       string
	FileExt        string
	MimeType       string
	FileSize       int64
	IsDir          bool
	RawURL         string
	SearchableText string
	ScannedAt      time.Time
}

func (u ExternalAssetUpsert) Normalized() ExternalAssetUpsert {
	u.Provider = strings.TrimSpace(u.Provider)
	if u.Provider == "" {
		u.Provider = "alist"
	}
	u.Driver = strings.TrimSpace(u.Driver)
	u.MountPath = cleanExternalPath(u.MountPath)
	u.OriginPath = cleanExternalPath(u.OriginPath)
	u.ParentPath = cleanExternalPath(u.ParentPath)
	u.FileName = strings.TrimSpace(u.FileName)
	u.FileExt = strings.ToLower(strings.TrimSpace(u.FileExt))
	u.MimeType = strings.TrimSpace(u.MimeType)
	u.RawURL = strings.TrimSpace(u.RawURL)
	if u.SearchableText == "" {
		u.SearchableText = strings.Join([]string{u.OriginPath, u.ParentPath, u.FileName, u.Driver, string(u.Kind)}, " ")
	}
	if u.ScannedAt.IsZero() {
		u.ScannedAt = time.Now().UTC()
	}
	switch u.Kind {
	case ExternalAssetKindNASLocal:
	default:
		u.Kind = ExternalAssetKindNetdisk
	}
	return u
}

func ExternalAssetResourceID(id int64) string {
	if id <= 0 {
		return ""
	}
	return "ext-" + strconv.FormatInt(id, 10)
}

func ParseExternalAssetResourceID(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	switch {
	case strings.HasPrefix(value, "external:"):
		value = strings.TrimPrefix(value, "external:")
	case strings.HasPrefix(value, "ext-"):
		value = strings.TrimPrefix(value, "ext-")
	default:
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func ExternalAssetOriginHash(provider, mountPath, originPath string) string {
	h := sha256.Sum256([]byte(strings.Join([]string{
		strings.ToLower(strings.TrimSpace(provider)),
		cleanExternalPath(mountPath),
		cleanExternalPath(originPath),
	}, "|")))
	return hex.EncodeToString(h[:])
}

func cleanExternalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Clean("/" + strings.TrimLeft(value, "/"))
	if value == "." {
		return "/"
	}
	return value
}
