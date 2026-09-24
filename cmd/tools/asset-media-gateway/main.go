package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"workflow/internal/assetmedia"
)

func env(key, fallback string) string {
	if s := strings.TrimSpace(os.Getenv(key)); s != "" {
		return s
	}
	return fallback
}
func number(key string, fallback int64) int64 {
	n, e := strconv.ParseInt(os.Getenv(key), 10, 64)
	if e != nil {
		return fallback
	}
	return n
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	raw, err := os.ReadFile(env("MEDIA_PUBLIC_KEYS_FILE", "/run/media/public-keys.json"))
	if err != nil {
		log.Fatal("media public keys unavailable")
	}
	var encoded map[string]string
	if err = json.Unmarshal(raw, &encoded); err != nil {
		log.Fatal("invalid public key document")
	}
	keys := map[string]ed25519.PublicKey{}
	for id, value := range encoded {
		key, e := base64.StdEncoding.DecodeString(value)
		if e != nil || len(key) != ed25519.PublicKeySize {
			log.Fatal("invalid media public key")
		}
		keys[id] = ed25519.PublicKey(key)
	}
	controlClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	gateway, err := assetmedia.NewGateway(assetmedia.GatewayConfig{ID: env("ASSET_MEDIA_GATEWAY_ID", "company-nas"), Root: env("MEDIA_SOURCE_ROOT", "/data/image_lib"), CacheDir: env("MEDIA_CACHE_DIR", "/cache"), ArtifactDir: env("MEDIA_ARTIFACT_DIR", "/artifacts"), Keys: keys,
		MetricsToken:   os.Getenv("ASSET_MEDIA_GATEWAY_TOKEN"),
		AllowedOrigins: strings.Split(env("MEDIA_ALLOWED_ORIGINS", "https://yongbo.cloud,https://assets.yongbo.cloud"), ","), AllowedOSSHosts: strings.Split(env("MEDIA_OSS_HOSTS", "yongbooss.oss-cn-hangzhou.aliyuncs.com"), ","),
		QuotaBytes: number("MEDIA_CACHE_BYTES", 2<<40), TemporaryBytes: number("MEDIA_TEMP_BYTES", 20<<30), FreeFloorBytes: uint64(number("MEDIA_FREE_FLOOR_BYTES", 200<<30)), DayMbps: number("MEDIA_DAY_MBPS", 30), NightMbps: number("MEDIA_NIGHT_MBPS", 60),
		Authorize: assetmedia.HTTPAuthorizer(env("MEDIA_CONTROL_URL", "https://yongbo.cloud"), os.Getenv("ASSET_MEDIA_GATEWAY_TOKEN"), controlClient)})
	if err != nil {
		log.Fatal(err)
	}
	go gateway.RunMetricsPersistence(ctx)
	server := &http.Server{Addr: env("MEDIA_GATEWAY_LISTEN", "127.0.0.1:18089"), Handler: gateway, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("media gateway listening address=%s", server.Addr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	if ctx.Err() != nil {
		<-shutdownDone
	}
}
