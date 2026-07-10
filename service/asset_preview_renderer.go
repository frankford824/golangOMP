package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AssetPreviewSourceMeta struct {
	Filename string
	MimeType string
}

type AssetPreviewRenderSpec struct {
	MaxWidth  int
	MaxHeight int
	Quality   int
}

type AssetPreviewRenderer interface {
	Render(ctx context.Context, sourcePath string, source AssetPreviewSourceMeta, spec AssetPreviewRenderSpec) ([]byte, error)
}

type ExternalAssetPreviewRenderer struct {
	Bin string
}

func NewExternalAssetPreviewRenderer() *ExternalAssetPreviewRenderer {
	return &ExternalAssetPreviewRenderer{Bin: strings.TrimSpace(os.Getenv("ASSET_PREVIEW_RENDERER_BIN"))}
}

func (r *ExternalAssetPreviewRenderer) Render(ctx context.Context, sourcePath string, source AssetPreviewSourceMeta, spec AssetPreviewRenderSpec) ([]byte, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("source path is required")
	}
	bin, mode, err := resolvePreviewRendererCommand(r.Bin)
	if err != nil {
		return nil, err
	}
	if spec.MaxWidth <= 0 {
		spec.MaxWidth = 1600
	}
	if spec.MaxHeight <= 0 {
		spec.MaxHeight = spec.MaxWidth
	}
	if spec.Quality <= 0 {
		spec.Quality = 82
	}

	outFile, err := os.CreateTemp("", "asset-preview-*.webp")
	if err != nil {
		return nil, fmt.Errorf("create preview output temp file: %w", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(outPath)

	input := sourcePath
	pdfCleanup := func() {}
	if isPDFPreviewSource(source.Filename, source.MimeType) {
		input, pdfCleanup, err = renderPDFPreviewInput(ctx, sourcePath, spec)
		if err != nil {
			return nil, err
		}
	} else if shouldReadFirstRenderableFrame(source.Filename, source.MimeType) {
		input += "[0]"
	}
	defer pdfCleanup()
	geometry := fmt.Sprintf("%dx%d>", spec.MaxWidth, spec.MaxHeight)
	args := []string{input, "-auto-orient", "-thumbnail", geometry, "-background", "white", "-alpha", "remove", "-alpha", "off", "-strip", "-quality", fmt.Sprintf("%d", spec.Quality), outPath}
	if mode == "magick" {
		args = append([]string{}, args...)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s preview render failed: %w output=%s", filepath.Base(bin), err, strings.TrimSpace(string(output)))
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read preview output: %w", err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("preview renderer produced empty output")
	}
	return content, nil
}

func renderPDFPreviewInput(ctx context.Context, sourcePath string, spec AssetPreviewRenderSpec) (string, func(), error) {
	bin, err := exec.LookPath("pdftoppm")
	if err != nil {
		return "", func() {}, fmt.Errorf("PDF preview renderer pdftoppm is not installed: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "asset-preview-pdf-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create PDF preview temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPrefix := filepath.Join(tempDir, "page")
	maxDimension := spec.MaxWidth
	if spec.MaxHeight > maxDimension {
		maxDimension = spec.MaxHeight
	}
	args := []string{
		"-f", "1",
		"-l", "1",
		"-singlefile",
		"-scale-to", fmt.Sprintf("%d", maxDimension),
		"-png",
		sourcePath,
		outputPrefix,
	}
	output, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("pdftoppm preview render failed: %w output=%s", err, strings.TrimSpace(string(output)))
	}
	outputPath := outputPrefix + ".png"
	if info, statErr := os.Stat(outputPath); statErr != nil || info.Size() == 0 {
		cleanup()
		if statErr != nil {
			return "", func() {}, fmt.Errorf("read pdftoppm preview output: %w", statErr)
		}
		return "", func() {}, fmt.Errorf("pdftoppm preview renderer produced empty output")
	}
	return outputPath, cleanup, nil
}

func resolvePreviewRendererCommand(configured string) (string, string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		if _, err := exec.LookPath(configured); err != nil {
			return "", "", fmt.Errorf("configured preview renderer %q not found: %w", configured, err)
		}
		mode := "convert"
		if strings.Contains(strings.ToLower(filepath.Base(configured)), "magick") {
			mode = "magick"
		}
		return configured, mode, nil
	}
	if bin, err := exec.LookPath("magick"); err == nil {
		return bin, "magick", nil
	}
	if bin, err := exec.LookPath("convert"); err == nil {
		if strings.EqualFold(filepath.Base(bin), "convert.exe") {
			return "", "", fmt.Errorf("Windows convert.exe is not ImageMagick; install ImageMagick magick/convert")
		}
		return bin, "convert", nil
	}
	return "", "", fmt.Errorf("asset preview renderer is not installed; install ImageMagick and set ASSET_PREVIEW_RENDERER_BIN if needed")
}

func shouldReadFirstRenderableFrame(filename, mimeType string) bool {
	ext := normalizePreviewFileExtension(filename)
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch ext {
	case ".psd", ".psb", ".pdf", ".ai", ".tiff":
		return true
	}
	return strings.Contains(mimeType, "photoshop") ||
		mimeType == "application/pdf" ||
		strings.Contains(mimeType, "illustrator") ||
		mimeType == "image/tiff" ||
		mimeType == "image/x-tiff"
}

func isPDFPreviewSource(filename, mimeType string) bool {
	if normalizePreviewFileExtension(filename) == ".pdf" {
		return true
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	return mimeType == "application/pdf"
}
