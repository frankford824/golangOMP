package service

import (
	"path/filepath"
	"strconv"
	"strings"

	"workflow/domain"
)

const (
	ossIMGPreviewWidth          = 1600
	ossIMGDefaultMaxSourceBytes = int64(20 * 1024 * 1024)
)

var ossIMGDirectSourceExtensions = map[string]struct{}{
	".jpg":  {},
	".png":  {},
	".bmp":  {},
	".gif":  {},
	".webp": {},
	".tiff": {},
	".heic": {},
	".avif": {},
}

var ossIMGAlphaPreserveExtensions = map[string]struct{}{
	".png":  {},
	".gif":  {},
	".webp": {},
}

func isOSSIMGDirectPreviewSupportedSourceVersion(version *domain.DesignAssetVersion) bool {
	if version == nil || !version.IsSourceFile {
		return false
	}
	return isOSSIMGDirectPreviewSupported(version.OriginalFilename, version.MimeType)
}

func isOSSIMGDirectPreviewSupported(filename, mimeType string) bool {
	ext := sourceAssetFormatExtension(filename, mimeType)
	_, ok := ossIMGDirectSourceExtensions[ext]
	return ok
}

func buildOSSIMGPreviewProcessForVersion(version *domain.DesignAssetVersion) (string, bool) {
	if version == nil {
		return "", false
	}
	fileSize := int64(0)
	if version.FileSize != nil {
		fileSize = *version.FileSize
	}
	return OSSIMGPreviewProcessForSize(version.OriginalFilename, version.MimeType, fileSize)
}

// OSSIMGPreviewProcess returns the bounded OSS IMG transform used by every
// browser preview of a directly processable raster image. Original downloads
// never call this helper and therefore retain their exact bytes.
func OSSIMGPreviewProcess(filename, mimeType string) (string, bool) {
	return OSSIMGPreviewProcessForSize(filename, mimeType, 0)
}

// OSSIMGPreviewProcessForSize keeps directly previewable images available when
// their known source size exceeds OSS IMG's default 20 MiB input limit. Those
// oversized objects fall back to an untransformed signed preview instead of a
// transform URL that OSS would reject.
func OSSIMGPreviewProcessForSize(filename, mimeType string, fileSize int64) (string, bool) {
	ext := sourceAssetFormatExtension(filename, mimeType)
	if _, ok := ossIMGDirectSourceExtensions[ext]; !ok {
		return "", false
	}
	if fileSize > ossIMGDefaultMaxSourceBytes {
		return "", true
	}
	return buildOSSIMGPreviewProcessByExtension(ext), true
}

func buildOSSIMGPreviewProcessByExtension(ext string) string {
	steps := []string{
		"image/auto-orient,1",
		"resize,w_" + intToString(ossIMGPreviewWidth) + ",m_lfit",
	}
	if _, keep := ossIMGAlphaPreserveExtensions[ext]; !keep {
		steps = append(steps, "quality,Q_82", "format,jpg")
	}
	return strings.Join(steps, "/")
}

func intToString(value int) string {
	return strconv.Itoa(value)
}

func sourceAssetFormatExtension(filename, mimeType string) string {
	ext := normalizePreviewFileExtension(filename)
	if ext != "" {
		return ext
	}
	return extensionByMimeType(mimeType)
}

func normalizePreviewFileExtension(filename string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(strings.TrimSpace(filename))))
	switch ext {
	case ".jpeg":
		return ".jpg"
	case ".tif":
		return ".tiff"
	case ".heif":
		return ".heic"
	default:
		return ext
	}
}

func extensionByMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/bmp", "image/x-ms-bmp":
		return ".bmp"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/tiff":
		return ".tiff"
	case "image/heic", "image/heif":
		return ".heic"
	case "image/avif":
		return ".avif"
	default:
		return ""
	}
}

func isPSDLikeAssetFile(filename, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if strings.Contains(mimeType, "photoshop") || strings.Contains(mimeType, "vnd.adobe.photoshop") {
		return true
	}
	switch normalizePreviewFileExtension(filename) {
	case ".psd", ".psb":
		return true
	default:
		return false
	}
}
