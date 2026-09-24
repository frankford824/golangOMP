// One-time provisioning for narrowly scoped media identities. Private signing
// material stays on ECS; only public-keys.json and NAS role env files leave it.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func provision(out string) error {
	if !filepath.IsAbs(out) || filepath.Clean(out) == "/" || strings.ContainsAny(out, " \t\r\n\"'`$") {
		return fmt.Errorf("safe absolute output directory required")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("refusing existing credentials directory")
	}
	tmp, err := os.MkdirTemp(filepath.Dir(out), ".media-credentials-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	token := func() (string, error) {
		b := make([]byte, 32)
		_, e := rand.Read(b)
		return base64.RawURLEncoding.EncodeToString(b), e
	}
	gateway, err := token()
	if err != nil {
		return err
	}
	worker, err := token()
	if err != nil {
		return err
	}
	publicJSON, _ := json.Marshal(map[string]string{"media-v1": base64.StdEncoding.EncodeToString(pub)})
	files := map[string][]byte{
		"signing.key":      []byte(base64.StdEncoding.EncodeToString(key) + "\n"),
		"public-keys.json": publicJSON,
		"worker.env":       []byte("ASSET_MEDIA_WORKER_TOKEN=" + worker + "\nMEDIA_CONTROL_URL=https://yongbo.cloud\nMEDIA_WORKER_ID=company-nas-worker\n"),
		"gateway.env":      []byte("ASSET_MEDIA_GATEWAY_TOKEN=" + gateway + "\nMEDIA_CONTROL_URL=https://yongbo.cloud\nASSET_MEDIA_GATEWAY_ID=company-nas\n"),
		"ecs.env":          []byte("ASSET_MEDIA_JOBS_ENABLED=false\nASSET_MEDIA_ECS_WORKER_ENABLED=false\nASSET_MEDIA_STRICT_PREVIEWS=false\nASSET_MEDIA_QUARK_DISABLED=false\nASSET_MEDIA_NAS_SCAN_ENABLED=false\nASSET_MEDIA_NAS_WORKER_ENABLED=false\nASSET_MEDIA_NAS_DELIVERY_ENABLED=false\nASSET_MEDIA_VERSIONED_SOURCES_ENABLED=false\nASSET_MEDIA_EXTERNAL_ZIP_ENABLED=false\nASSET_MEDIA_GATEWAY_ID=company-nas\nASSET_MEDIA_GATEWAY_URL=https://media-cache.yongbo.cloud\nASSET_MEDIA_SIGNING_KEY_ID=media-v1\nASSET_MEDIA_SIGNING_KEY_FILE=" + filepath.Join(out, "signing.key") + "\nASSET_MEDIA_GATEWAY_TOKEN=" + gateway + "\nASSET_MEDIA_WORKER_TOKEN=" + worker + "\n"),
	}
	for name, body := range files {
		if err = os.WriteFile(filepath.Join(tmp, name), body, 0600); err != nil {
			return err
		}
	}
	if err = os.Chmod(filepath.Join(tmp, "public-keys.json"), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}

func main() {
	out := flag.String("out-dir", "", "new protected ECS credential directory")
	flag.Parse()
	if err := provision(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created media credentials; all feature flags are off. Never copy signing.key or ecs.env to NAS.")
}
