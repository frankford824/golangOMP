package service

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAssetMediaRealRendererTransparencyAndInvalidFiles(t *testing.T) {
	if os.Getenv("ASSET_MEDIA_REAL_RENDER_TEST") != "1" {
		t.Skip("requires isolated deployed renderer image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "透明中文样本.png")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2400, 1600))
	for y := 200; y < 1400; y++ {
		for x := 300; x < 2100; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 80, B: 20, A: 128})
		}
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	renderer := NewExternalAssetPreviewRenderer()
	for _, edge := range []int{1600, 480} {
		body, err := renderer.Render(ctx, source, AssetPreviewSourceMeta{Filename: filepath.Base(source), MimeType: "image/png"}, AssetPreviewRenderSpec{MaxWidth: edge, MaxHeight: edge, Quality: 82})
		if err != nil {
			t.Fatal(err)
		}
		max := 2 << 20
		if edge == 480 {
			max = 200 << 10
		}
		if len(body) > max {
			t.Fatalf("oversized representation: %d", len(body))
		}
		out := filepath.Join(dir, "result.webp")
		if err = os.WriteFile(out, body, 0600); err != nil {
			t.Fatal(err)
		}
		channels, err := exec.CommandContext(ctx, "identify", "-format", "%[channels]", out).Output()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(channels), "a") {
			t.Fatalf("alpha lost: %s", channels)
		}
	}
	frames := []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 80, 50), color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}}), image.NewPaletted(image.Rect(0, 0, 80, 50), color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}})}
	for i := range frames[1].Pix {
		frames[1].Pix[i] = 1
	}
	animated := filepath.Join(dir, "two-frames.gif")
	animatedFile, err := os.Create(animated)
	if err != nil {
		t.Fatal(err)
	}
	if err = gif.EncodeAll(animatedFile, &gif.GIF{Image: frames, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	animatedFile.Close()
	body, err := renderer.Render(ctx, animated, AssetPreviewSourceMeta{Filename: "two-frames.gif"}, AssetPreviewRenderSpec{MaxWidth: 480, MaxHeight: 480})
	if err != nil {
		t.Fatal(err)
	}
	firstFrame := filepath.Join(dir, "first.webp")
	if err = os.WriteFile(firstFrame, body, 0600); err != nil {
		t.Fatal(err)
	}
	stats, err := exec.CommandContext(ctx, "identify", "-format", "%n %[fx:mean.r] %[fx:mean.b]", firstFrame).Output()
	if err != nil {
		t.Fatal(err)
	}
	var frameCount int
	var red, blue float64
	if _, err = fmt.Sscanf(string(stats), "%d %f %f", &frameCount, &red, &blue); err != nil || frameCount != 1 || red < 0.8 || blue > 0.2 {
		t.Fatalf("expected first red frame only: %s %v", stats, err)
	}
	if _, err = renderer.Render(ctx, firstFrame, AssetPreviewSourceMeta{Filename: "first.webp"}, AssetPreviewRenderSpec{MaxWidth: 480, MaxHeight: 480}); err != nil {
		t.Fatalf("WebP source unsupported: %v", err)
	}
	for _, name := range []string{"corrupt.jpg", "unsupported.txt"} {
		input := filepath.Join(dir, name)
		if err = os.WriteFile(input, []byte("not an image"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = renderer.Render(ctx, input, AssetPreviewSourceMeta{Filename: name}, AssetPreviewRenderSpec{MaxWidth: 480, MaxHeight: 480}); err == nil {
			t.Fatalf("invalid file rendered: %s", name)
		}
	}
}
