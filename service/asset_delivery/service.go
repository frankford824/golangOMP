package assetdelivery

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"workflow/config"
	"workflow/domain"
	"workflow/internal/assetmedia"
	"workflow/repo"
	baseservice "workflow/service"
	assetcenter "workflow/service/asset_center"
	assetworkbench "workflow/service/asset_workbench"
	externalassets "workflow/service/external_assets"
)

type AccessReader interface {
	EffectiveAccess(context.Context, int64) (*domain.EffectiveAccess, *domain.AppError)
}

type Request struct {
	ResourceKind          string `json:"resource_kind"`
	ResourceID            string `json:"resource_id"`
	ItemID                int64  `json:"item_id,omitempty"`
	Purpose               string `json:"purpose"`
	Rendition             string `json:"rendition"`
	Delivery              string `json:"delivery"`
	ExpectedSourceVersion string `json:"expected_source_version,omitempty"`
}

type Service struct {
	Config     config.AssetMediaConfig
	Jobs       repo.AssetMediaJobRepo
	Users      repo.UserRepo
	Tasks      repo.TaskRepo
	TaskAssets repo.TaskAssetRepo
	Access     AccessReader
	TaskCenter baseservice.TaskAssetCenterService
	Assets     *assetcenter.Service
	External   *externalassets.Service
	Workbench  *assetworkbench.Service
	OSS        *baseservice.OSSDirectService
	key        ed25519.PrivateKey
	pub        ed25519.PublicKey
}

func (s *Service) Initialize() error {
	if !s.Config.NASDeliveryEnabled {
		return nil
	}
	raw, err := os.ReadFile(s.Config.SigningKeyFile)
	if err != nil {
		return fmt.Errorf("read media signing key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return fmt.Errorf("invalid media signing key encoding")
	}
	if len(key) == ed25519.SeedSize {
		key = ed25519.NewKeyFromSeed(key)
	}
	if len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid media signing key length")
	}
	u, err := url.Parse(s.Config.GatewayURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("HTTPS media gateway required")
	}
	if s.Config.GatewayToken == "" {
		return fmt.Errorf("media gateway validation credential required")
	}
	s.key = ed25519.PrivateKey(key)
	s.pub = s.key.Public().(ed25519.PublicKey)
	return nil
}

func normalizeRequest(r Request) (Request, *domain.AppError) {
	if r.ResourceKind == "" {
		r.ResourceKind = "asset"
	}
	if r.Purpose == "" {
		r.Purpose = "download"
	}
	if r.Rendition == "" {
		r.Rendition = "original"
		if r.Purpose == "preview" {
			r.Rendition = "preview"
		}
	}
	if r.Delivery == "" {
		r.Delivery = "auto"
	}
	if r.Delivery != "auto" && r.Delivery != "lan" && r.Delivery != "cloud" {
		return r, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid delivery preference", nil)
	}
	if (r.Purpose == "download" && r.Rendition != "original") || (r.Purpose == "preview" && r.Rendition != "preview" && r.Rendition != "thumbnail") || (r.Purpose != "preview" && r.Purpose != "download") {
		return r, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media purpose or rendition", nil)
	}
	return r, nil
}

func (s *Service) Resolve(ctx context.Context, r Request) (*domain.AssetDownloadInfo, *domain.AppError) {
	r, appErr := normalizeRequest(r)
	if appErr != nil {
		return nil, appErr
	}
	if r.Delivery == "lan" && !s.Config.NASDeliveryEnabled {
		r.Delivery = "cloud"
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	permission := domain.PermissionAssetDownload
	if r.Purpose == "preview" {
		permission = domain.PermissionAssetView
	}
	if !domain.ActorHasPermission(actor, permission) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "media access denied", nil)
	}
	ctx = domain.WithAssetMediaOptions(ctx, domain.AssetMediaOptions{Delivery: r.Delivery, ExpectedVersion: r.ExpectedSourceVersion, Rendition: r.Rendition,
		AccessReference: &domain.AssetMediaAccessReference{ResourceKind: r.ResourceKind, ResourceID: r.ResourceID, ItemID: r.ItemID, Purpose: r.Purpose, Rendition: r.Rendition, ExpectedSourceVersion: r.ExpectedSourceVersion}})
	info, target, appErr := s.resolveAuthorized(ctx, r, actor)
	if appErr != nil {
		return nil, appErr
	}
	if info == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "media resolution returned no result", nil)
	}
	if r.ExpectedSourceVersion != "" && info.SourceVersion != "" && r.ExpectedSourceVersion != info.SourceVersion {
		return nil, domain.NewAppError("source_version_changed", "文件已更新，请刷新后重新选择", nil)
	}
	if r.Delivery == "lan" && s.Config.NASDeliveryEnabled && info.State == "ready" && target != nil {
		now := time.Now().UTC()
		ttl := 10 * time.Minute
		if r.Purpose == "preview" {
			ttl = 2 * time.Minute
		}
		expires := now.Add(ttl)
		claim := assetmedia.Ticket{Version: 1, KeyID: s.Config.SigningKeyID, GatewayID: s.Config.GatewayID, ActorID: actor.ID,
			ResourceKind: r.ResourceKind, ResourceID: r.ResourceID, ItemID: r.ItemID, SourceVersion: info.SourceVersion, ContentID: info.ContentID,
			Purpose: r.Purpose, Rendition: r.Rendition, IssuedAt: now.Unix(), ExpiresAt: expires.Unix()}
		ticket, err := assetmedia.SignTicket(claim, s.key)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "media signing unavailable", nil)
		}
		localURL := strings.TrimRight(s.Config.GatewayURL, "/") + "/edge/v1/content/" + info.ContentID + "?ticket=" + url.QueryEscape(ticket)
		info.LANDelivery = &domain.AssetMediaDelivery{State: "ready", URL: localURL, GatewayID: s.Config.GatewayID, ExpiresAt: &expires}
		info.DownloadURL = &localURL
		info.ExpiresAt = &expires
		info.AccessHint = "nas_authorized"
	}
	return info, nil
}

func (s *Service) resolveAuthorized(ctx context.Context, r Request, actor domain.RequestActor) (*domain.AssetDownloadInfo, *assetmedia.ReadTarget, *domain.AppError) {
	if r.ResourceKind == "package" {
		return s.resolvePackage(ctx, r, actor)
	}
	var info *domain.AssetDownloadInfo
	var appErr *domain.AppError
	id, err := strconv.ParseInt(r.ResourceID, 10, 64)
	if externalID, ok := domain.ParseExternalAssetResourceID(r.ResourceID); ok && r.ResourceKind == "asset" {
		r.ResourceKind = "external_asset"
		id = externalID
		err = nil
	}
	if r.ResourceKind == "external_asset" {
		if externalID, ok := domain.ParseExternalAssetResourceID(r.ResourceID); ok {
			id = externalID
			err = nil
		}
	}
	if err != nil || id <= 0 {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media resource", nil)
	}
	switch r.ResourceKind {
	case "external_asset":
		info, appErr = s.External.ExternalMediaInfo(ctx, id, r.Rendition, r.Delivery)
		if appErr != nil {
			return nil, nil, appErr
		}
		if info.State != "ready" {
			return info, nil, nil
		}
		row, e := s.External.GetMediaRecord(ctx, id)
		if e != nil {
			return nil, nil, e
		}
		if r.Rendition == "original" && r.Delivery == "lan" && row.Media != nil {
			fp := row.Media
			return info, &assetmedia.ReadTarget{Source: "nas", ContentID: info.ContentID, SourceVersion: info.SourceVersion,
				RelativePath: strings.TrimPrefix(row.OriginPath, "/p3/"), Filename: row.FileName, MimeType: row.MimeType, Size: row.FileSize,
				ModifiedNS: fp.ModifiedNS, ChangedNS: fp.ChangedNS, FileIdentity: fp.FileIdentity, RootIdentity: fp.RootIdentity, SHA256: fp.SHA256}, nil
		}
		if r.Rendition != "original" && r.Delivery == "lan" && row.Media != nil {
			var artifacts domain.ExternalMediaResult
			if json.Unmarshal(row.Media.Result, &artifacts) == nil {
				object := artifacts.Preview
				if r.Rendition == "thumbnail" {
					object = artifacts.Thumbnail
				}
				if object != nil && object.SourceVersion == info.SourceVersion {
					origin := ""
					if info.DownloadURL != nil {
						origin = *info.DownloadURL
					}
					return info, &assetmedia.ReadTarget{Source: "artifact", OriginURL: origin, ContentID: info.ContentID, SourceVersion: info.SourceVersion, Filename: object.Filename, MimeType: object.MimeType, Size: object.Size, SHA256: object.SHA256, CRC64: object.CRC64}, nil
				}
			}
		}
	case "task_asset":
		if r.Purpose == "download" {
			info, appErr = s.TaskCenter.GetTaskAssetDownloadInfoByID(ctx, id)
		} else if r.Rendition == "thumbnail" {
			p, ok := s.TaskCenter.(interface {
				GetTaskAssetThumbnailInfoByID(context.Context, int64) (*domain.AssetDownloadInfo, *domain.AppError)
			})
			if !ok {
				return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "thumbnail resolver unavailable", nil)
			}
			info, appErr = p.GetTaskAssetThumbnailInfoByID(ctx, id)
		} else {
			info, appErr = s.TaskCenter.GetTaskAssetPreviewInfoByID(ctx, id)
		}
	case "asset":
		if r.Purpose == "preview" {
			if r.Rendition == "thumbnail" {
				p := s.TaskCenter.(interface {
					GetAssetThumbnailInfoByID(context.Context, int64) (*domain.AssetDownloadInfo, *domain.AppError)
				})
				info, appErr = p.GetAssetThumbnailInfoByID(ctx, id)
			} else {
				info, appErr = s.TaskCenter.GetAssetPreviewInfoByID(ctx, id)
			}
		} else {
			detail, e := s.Assets.GetDetail(ctx, id)
			if e != nil {
				return nil, nil, e
			}
			task, e2 := s.Tasks.GetByID(ctx, detail.TaskID)
			if e2 != nil || task == nil {
				return nil, nil, domain.ErrNotFound
			}
			if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetDownload, task.AccessSubject()) {
				return nil, nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset is outside current data scope", nil)
			}
			if detail.CurrentVersionID == nil {
				return nil, nil, domain.ErrNotFound
			}
			// Freeze the concrete row before signing; a concurrent root-pointer
			// update must not combine a new URL with the old version/size.
			info, appErr = s.TaskCenter.GetTaskAssetDownloadInfoByID(ctx, *detail.CurrentVersionID)
		}
	case "client_material":
		info, appErr = s.Workbench.ClientMaterialMediaInfo(ctx, actor, id, r.Purpose, r.Rendition, r.ItemID)
	default:
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported resource kind", nil)
	}
	if appErr != nil {
		return nil, nil, appErr
	}
	if info == nil {
		return nil, nil, domain.ErrNotFound
	}
	if info.Rendition == "" {
		info.Rendition = r.Rendition
	}
	if info.ContentID == "" && info.SourceVersion != "" {
		info.ContentID = domain.AssetMediaIdentity(info.SourceVersion, r.Rendition, domain.AssetMediaRecipe)
	}
	if info.LocalSource != nil && r.Delivery == "lan" && info.LocalSource.Media != nil {
		row := info.LocalSource
		fp := row.Media
		return info, &assetmedia.ReadTarget{Source: "nas", ContentID: info.ContentID, SourceVersion: info.SourceVersion, RelativePath: strings.TrimPrefix(row.OriginPath, "/p3/"), Filename: row.FileName, MimeType: row.MimeType, Size: row.FileSize, ModifiedNS: fp.ModifiedNS, ChangedNS: fp.ChangedNS, FileIdentity: fp.FileIdentity, RootIdentity: fp.RootIdentity, SHA256: fp.SHA256}, nil
	}
	if info.DownloadURL != nil && *info.DownloadURL != "" {
		info.State = "ready"
		info.CloudDelivery = &domain.AssetMediaDelivery{State: "ready", URL: *info.DownloadURL, ExpiresAt: info.ExpiresAt}
	}
	if info.State != "ready" || info.ObjectKey == "" {
		return info, nil, nil
	}
	if r.Delivery != "lan" {
		return info, nil, nil
	}
	stat, exists, e := s.OSS.StatObject(ctx, info.ObjectKey)
	if e != nil {
		return nil, nil, domain.NewAppError("media_temporarily_unavailable", "文件校验暂不可用", nil)
	}
	if !exists || stat == nil {
		return nil, nil, domain.NewAppError("source_missing", "原件已不可用", nil)
	}
	if stat.ContentLength != info.FileSize {
		return nil, nil, domain.NewAppError("source_version_changed", "文件大小与版本记录不符", nil)
	}
	signed := s.OSS.PresignDownloadURL(info.ObjectKey)
	if signed == nil {
		return nil, nil, domain.NewAppError("media_temporarily_unavailable", "文件地址暂不可用", nil)
	}
	return info, &assetmedia.ReadTarget{Source: "oss", ContentID: info.ContentID, SourceVersion: info.SourceVersion, OriginURL: signed.DownloadURL, Filename: info.Filename, MimeType: info.MimeType, Size: stat.ContentLength, CRC64: stat.CRC64ECMA}, nil
}

func (s *Service) freshActor(ctx context.Context, id int64) (domain.RequestActor, *domain.AppError) {
	u, err := s.Users.GetByID(ctx, id)
	if err != nil || u == nil || u.Status != domain.UserStatusActive {
		return domain.RequestActor{}, domain.ErrUnauthorized
	}
	access, appErr := s.Access.EffectiveAccess(ctx, id)
	if appErr != nil {
		return domain.RequestActor{}, appErr
	}
	if access == nil {
		return domain.RequestActor{}, domain.ErrUnauthorized
	}
	roles, err := s.Users.ListRoles(ctx, id)
	if err != nil {
		return domain.RequestActor{}, domain.NewAppError(domain.ErrCodeInternalError, "resolve current identity", nil)
	}
	return domain.RequestActor{ID: id, Username: u.Username, Department: string(u.Department), DepartmentID: u.DepartmentID, Team: u.Team, TeamID: u.TeamID, Roles: roles,
		EffectiveAccess: access, Permissions: access.Permissions, AccessPolicyRevision: access.PolicyRevision, Source: domain.RequestActorSourceSessionToken, AuthMode: domain.AuthModeSessionTokenRoleEnforced}, nil
}

func (s *Service) ValidateTicket(ctx context.Context, token string) (*assetmedia.ReadTarget, *domain.AppError) {
	if !s.Config.NASDeliveryEnabled {
		return nil, domain.NewAppError("media_temporarily_unavailable", "NAS delivery disabled", nil)
	}
	claim, err := assetmedia.VerifyTicket(token, s.Config.GatewayID, map[string]ed25519.PublicKey{s.Config.SigningKeyID: s.pub}, time.Now())
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	actor, appErr := s.freshActor(ctx, claim.ActorID)
	if appErr != nil {
		return nil, appErr
	}
	permission := domain.PermissionAssetDownload
	if claim.Purpose == "preview" {
		permission = domain.PermissionAssetView
	}
	if !domain.ActorHasPermission(actor, permission) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "media access revoked", nil)
	}
	ctx = domain.WithRequestActor(ctx, actor)
	ctx = domain.WithAssetMediaOptions(ctx, domain.AssetMediaOptions{Delivery: "lan", ExpectedVersion: claim.SourceVersion, Rendition: claim.Rendition})
	info, target, appErr := s.resolveAuthorized(ctx, Request{ResourceKind: claim.ResourceKind, ResourceID: claim.ResourceID, ItemID: claim.ItemID, Purpose: claim.Purpose, Rendition: claim.Rendition, Delivery: "lan"}, actor)
	if appErr != nil {
		return nil, appErr
	}
	if info == nil || target == nil || info.State != "ready" || info.ContentID != claim.ContentID || info.SourceVersion != claim.SourceVersion {
		return nil, domain.NewAppError("source_version_changed", "media version no longer current or ready", nil)
	}
	return target, nil
}

func (s *Service) JobForActor(ctx context.Context, id string) (*domain.AssetMediaJob, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	if s.Jobs == nil {
		return nil, domain.ErrNotFound
	}
	owned, err := s.Jobs.HasRequest(ctx, id, actor.ID)
	if err != nil || !owned {
		return nil, domain.ErrNotFound
	}
	job, err := s.Jobs.Get(ctx, id)
	if err != nil || job == nil {
		return nil, domain.ErrNotFound
	}
	access, err := s.Jobs.GetRequestAccess(ctx, id, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "load media authorization context", nil)
	}
	if access != nil && access.ResourceKind != "package" {
		permission := domain.PermissionAssetDownload
		if access.Purpose == "preview" {
			permission = domain.PermissionAssetView
		}
		if !domain.ActorHasPermission(actor, permission) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "media access revoked", nil)
		}
		readCtx := domain.WithAssetMediaOptions(ctx, domain.AssetMediaOptions{Delivery: "lan", AuthorizeOnly: true, ExpectedVersion: access.ExpectedSourceVersion, Rendition: access.Rendition})
		info, _, appErr := s.resolveAuthorized(readCtx, Request{ResourceKind: access.ResourceKind, ResourceID: access.ResourceID, ItemID: access.ItemID, Purpose: access.Purpose, Rendition: access.Rendition, Delivery: "cloud"}, actor)
		if appErr != nil {
			return nil, appErr
		}
		if info == nil || info.SourceVersion != access.ExpectedSourceVersion {
			return nil, domain.NewAppError("source_version_changed", "文件版本已变化", nil)
		}
		return job, nil
	}
	var input domain.AssetMediaJobInput
	if json.Unmarshal(job.Payload, &input) != nil {
		return nil, domain.ErrNotFound
	}
	if input.TaskID > 0 {
		task, e := s.Tasks.GetByID(ctx, input.TaskID)
		if e != nil || task == nil || !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, task.AccessSubject()) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "media task scope denied", nil)
		}
	} else if input.ExternalID > 0 {
		if !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
			return nil, domain.ErrUnauthorized
		}
		if _, e := s.External.GetMediaRecord(ctx, input.ExternalID); e != nil {
			return nil, e
		}
	} else if len(input.Selection) > 0 {
		if !domain.ActorHasPermission(actor, domain.PermissionAssetDownload) {
			return nil, domain.ErrUnauthorized
		}
		if _, err := s.packageInputs(ctx, input.Selection); err != nil {
			return nil, domain.NewAppError("source_version_changed", "选择集中的文件已变化，请重新选择", nil)
		}
	}
	return job, nil
}
