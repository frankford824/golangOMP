// Package assetmedia contains the narrow protocol shared by MAIN and the NAS
// media gateway. It does not import business services or database access.
package assetmedia

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Ticket struct {
	Version       int    `json:"v"`
	KeyID         string `json:"kid"`
	GatewayID     string `json:"aud"`
	ActorID       int64  `json:"actor_id"`
	ResourceKind  string `json:"resource_kind"`
	ResourceID    string `json:"resource_id"`
	ItemID        int64  `json:"item_id,omitempty"`
	SourceVersion string `json:"source_version"`
	ContentID     string `json:"content_id"`
	Purpose       string `json:"purpose"`
	Rendition     string `json:"rendition"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
}

type ReadTarget struct {
	Source        string `json:"source"`
	ContentID     string `json:"content_id"`
	SourceVersion string `json:"source_version"`
	RelativePath  string `json:"relative_path,omitempty"`
	OriginURL     string `json:"origin_url,omitempty"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256,omitempty"`
	CRC64         string `json:"crc64,omitempty"`
	ModifiedNS    int64  `json:"modified_ns,omitempty"`
	ChangedNS     int64  `json:"changed_ns,omitempty"`
	FileIdentity  string `json:"file_identity,omitempty"`
	RootIdentity  string `json:"root_identity,omitempty"`
}

func SignTicket(t Ticket, key ed25519.PrivateKey) (string, error) {
	if len(key) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid signing key")
	}
	if err := validateTicket(t, t.GatewayID, time.Unix(t.IssuedAt, 0)); err != nil {
		return "", err
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := ed25519.Sign(key, []byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyTicket(token, gateway string, keys map[string]ed25519.PublicKey, now time.Time) (Ticket, error) {
	var t Ticket
	if len(token) > 8192 {
		return t, fmt.Errorf("invalid ticket")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return t, fmt.Errorf("invalid ticket")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return t, fmt.Errorf("invalid ticket")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&t); err != nil {
		return t, fmt.Errorf("invalid ticket")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return t, fmt.Errorf("invalid ticket")
	}
	key := keys[t.KeyID]
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, []byte(parts[0]), sig) {
		return t, fmt.Errorf("invalid ticket signature")
	}
	return t, validateTicket(t, gateway, now)
}

func validateTicket(t Ticket, gateway string, now time.Time) error {
	switch t.ResourceKind {
	case "asset", "task_asset", "external_asset", "client_material", "package":
	default:
		return fmt.Errorf("invalid resource kind")
	}
	if t.Version != 1 || t.KeyID == "" || gateway == "" || t.GatewayID != gateway || t.ActorID <= 0 || t.ResourceID == "" || t.SourceVersion == "" || len(t.ContentID) != 64 {
		return fmt.Errorf("invalid ticket scope")
	}
	for _, c := range t.ContentID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return fmt.Errorf("invalid content identity")
		}
	}
	maxTTL := int64(600)
	if t.Purpose == "preview" {
		maxTTL = 120
		if t.Rendition != "thumbnail" && t.Rendition != "preview" {
			return fmt.Errorf("invalid preview rendition")
		}
	} else if t.Purpose != "download" || t.Rendition != "original" {
		return fmt.Errorf("invalid ticket purpose")
	}
	if t.ExpiresAt <= now.Unix() || t.IssuedAt > now.Unix()+30 || t.ExpiresAt <= t.IssuedAt || t.ExpiresAt-t.IssuedAt > maxTTL {
		return fmt.Errorf("ticket expired or invalid lifetime")
	}
	return nil
}
