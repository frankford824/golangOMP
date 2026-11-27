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
	PublicBaseURL         string
	SigningSecret         string
	TokenTTL              time.Duration
	PreviousSigningSecret string
	PreviousValidUntil    time.Time
}

type ERPImageProxySigner struct {
	publicBaseURL         string
	signingSecret         []byte
	tokenTTL              time.Duration
	previousSigningSecret []byte
	previousValidUntil    time.Time
	now                   func() time.Time
}

func NewERPImageProxySigner(cfg ERPImageProxyConfig) *ERPImageProxySigner {
	ttl := cfg.TokenTTL
	if ttl <= 0 {
		ttl = 365 * 24 * time.Hour
	}
	return &ERPImageProxySigner{
		publicBaseURL:         strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		signingSecret:         []byte(strings.TrimSpace(cfg.SigningSecret)),
		tokenTTL:              ttl,
		previousSigningSecret: []byte(strings.TrimSpace(cfg.PreviousSigningSecret)),
		previousValidUntil:    cfg.PreviousValidUntil,
		now:                   time.Now,
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
	return s.buildImageURL(asset.ID, storageKey, ERPImageProxyPathPrefix)
}

func (s *ERPImageProxySigner) BuildLegacyImageURL(id int64, key string) *string {
	if !s.Enabled() || id <= 0 || strings.TrimSpace(key) == "" {
		return nil
	}
	return s.buildImageURL(id, "legacy-oss:"+key, ERPImageProxyPathPrefix+"/oss")
}

func (s *ERPImageProxySigner) buildImageURL(id int64, storageKey, prefix string) *string {
	expiresUnix := s.now().Add(s.tokenTTL).Unix()
	base, err := domain.BuildAbsoluteEscapedURLPath(s.publicBaseURL, prefix, strconv.FormatInt(id, 10))
	if err != nil {
		return nil
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil
	}
	query := parsed.Query()
	query.Set("exp", strconv.FormatInt(expiresUnix, 10))
	query.Set("sig", erpImageSignature(s.signingSecret, id, storageKey, expiresUnix))
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
	return s.verifyImageURL(asset.ID, storageKey, expiresRaw, signatureRaw)
}

func (s *ERPImageProxySigner) VerifyLegacyImage(id int64, key, expiresRaw, signatureRaw string) bool {
	return s.Enabled() && id > 0 && strings.TrimSpace(key) != "" &&
		s.verifyImageURL(id, "legacy-oss:"+key, expiresRaw, signatureRaw)
}

func (s *ERPImageProxySigner) verifyImageURL(id int64, storageKey, expiresRaw, signatureRaw string) bool {
	expiresUnix, err := strconv.ParseInt(strings.TrimSpace(expiresRaw), 10, 64)
	if err != nil || expiresUnix <= 0 {
		return false
	}
	if !s.now().Before(time.Unix(expiresUnix, 0)) {
		return false
	}
	signature := []byte(strings.TrimSpace(signatureRaw))
	if hmac.Equal([]byte(erpImageSignature(s.signingSecret, id, storageKey, expiresUnix)), signature) {
		return true
	}
	return len(s.previousSigningSecret) > 0 && s.now().Before(s.previousValidUntil) &&
		hmac.Equal([]byte(erpImageSignature(s.previousSigningSecret, id, storageKey, expiresUnix)), signature)
}

func erpImageSignature(secret []byte, assetVersionID int64, storageKey string, expiresUnix int64) string {
	payload := strconv.FormatInt(assetVersionID, 10) + "\n" + strings.TrimSpace(storageKey) + "\n" + strconv.FormatInt(expiresUnix, 10)
	mac := hmac.New(sha256.New, secret)
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

func (s *ERPImageProxySigner) verifiesCurrentKey(asset *domain.TaskAsset, expiresRaw, signature string) bool {
	return s.matchesCurrent(asset.ID, erpImageProxyAssetStorageKey(asset), expiresRaw, signature)
}

func (s *ERPImageProxySigner) matchesCurrent(id int64, key, expiresRaw, signature string) bool {
	expires, err := strconv.ParseInt(expiresRaw, 10, 64)
	return err == nil && s.now().Before(time.Unix(expires, 0)) &&
		hmac.Equal([]byte(erpImageSignature(s.signingSecret, id, key, expires)), []byte(signature))
}
