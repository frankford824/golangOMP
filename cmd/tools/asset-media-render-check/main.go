// asset-media-render-check validates the actual renderer in its deployment
// image without database access or object uploads. Original bytes are read-only.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"workflow/service"
)

func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}

func main() {
	source := flag.String("source", "", "read-only sample file")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "--source required")
		os.Exit(2)
	}
	if err := check(*source); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	before, err := digest(source)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "media-render-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	renderer := service.NewExternalAssetPreviewRenderer()
	results := []map[string]any{}
	input, filename := source, filepath.Base(source)
	for _, spec := range []service.AssetPreviewRenderSpec{{MaxWidth: 1600, MaxHeight: 1600, Quality: 82}, {MaxWidth: 480, MaxHeight: 480, Quality: 78}} {
		started := time.Now()
		body, err := renderer.Render(ctx, input, service.AssetPreviewSourceMeta{Filename: filename}, spec)
		if err != nil {
			return err
		}
		out := filepath.Join(dir, fmt.Sprintf("%d.webp", spec.MaxWidth))
		if err = os.WriteFile(out, body, 0600); err != nil {
			return err
		}
		dimensions, err := exec.CommandContext(ctx, "identify", "-format", "%w %h", out).Output()
		if err != nil {
			return err
		}
		var width, height int
		if _, err = fmt.Sscanf(string(dimensions), "%d %d", &width, &height); err != nil {
			return err
		}
		if width > spec.MaxWidth || height > spec.MaxHeight {
			return fmt.Errorf("rendition exceeds bounds")
		}
		results = append(results, map[string]any{"longest_edge": spec.MaxWidth, "width": width, "height": height, "bytes": len(body), "seconds": time.Since(started).Seconds()})
		input, filename = out, "preview.webp"
	}
	after, err := digest(source)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("sample changed during rendering")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"sample": strings.TrimSpace(filepath.Base(source)), "source_sha256": before, "original_unchanged": true, "renditions": results})
}
