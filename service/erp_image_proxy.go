package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

const ERPImageProxyPathPrefix = "/v1/public/erp-product-images"

type ERPImageProxyConfig struct {
	PublicBaseURL string
	SigningSecret string
	TokenTTL      time.Duration
}

type ERPImageProxySigner struct {
	publicBaseURL string
	signingSecret []byte
	tokenTTL      time.Duration
	now           func() time.Time
}

func NewERPImageProxySigner(cfg ERPImageProxyConfig) *ERPImageProxySigner {
	ttl := cfg.TokenTTL
	if ttl <= 0 {
		ttl = 365 * 24 * time.Hour
	}
	return &ERPImageProxySigner{
		publicBaseURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		signingSecret: []byte(strings.TrimSpace(cfg.SigningSecret)),
		tokenTTL:      ttl,
		now:           time.Now,
	}
}

func (s *ERPImageProxySigner) Enabled() bool {
	return s != nil &&
		strings.HasPrefix(strings.TrimSpace(s.publicBaseURL), "https://") &&
		len(s.signingSecret) > 0
}

func (s *ERPImageProxySigner) BuildImageURL(asset *domain.TaskAsset) *string {
	if !s.Enabled() || asset == nil || asset.ID <= 0 {
		return nil
	}
	storageKey := erpImageProxyAssetStorageKey(asset)
	if storageKey == "" {
		return nil
	}
	expiresUnix := s.now().Add(s.tokenTTL).Unix()
	base, err := domain.BuildAbsoluteEscapedURLPath(s.publicBaseURL, ERPImageProxyPathPrefix, strconv.FormatInt(asset.ID, 10))
	if err != nil {
		return nil
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil
	}
	query := parsed.Query()
	query.Set("exp", strconv.FormatInt(expiresUnix, 10))
	query.Set("sig", s.signature(asset.ID, storageKey, expiresUnix))
	parsed.RawQuery = query.Encode()
	value := parsed.String()
	return &value
}

func (s *ERPImageProxySigner) Verify(asset *domain.TaskAsset, expiresRaw, signatureRaw string) bool {
	if !s.Enabled() || asset == nil || asset.ID <= 0 {
		return false
	}
	storageKey := erpImageProxyAssetStorageKey(asset)
	if storageKey == "" {
		return false
	}
	expiresUnix, err := strconv.ParseInt(strings.TrimSpace(expiresRaw), 10, 64)
	if err != nil || expiresUnix <= 0 {
		return false
	}
	if !s.now().Before(time.Unix(expiresUnix, 0)) {
		return false
	}
	expected := s.signature(asset.ID, storageKey, expiresUnix)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signatureRaw)))
}

func (s *ERPImageProxySigner) signature(assetVersionID int64, storageKey string, expiresUnix int64) string {
	payload := strconv.FormatInt(assetVersionID, 10) + "\n" + strings.TrimSpace(storageKey) + "\n" + strconv.FormatInt(expiresUnix, 10)
	mac := hmac.New(sha256.New, s.signingSecret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func erpImageProxyAssetStorageKey(asset *domain.TaskAsset) string {
	if asset == nil {
		return ""
	}
	if asset.StorageKey != nil && strings.TrimSpace(*asset.StorageKey) != "" {
		return strings.TrimSpace(*asset.StorageKey)
	}
	if asset.StorageRef != nil {
		return strings.TrimSpace(asset.StorageRef.RefKey)
	}
	return ""
}
