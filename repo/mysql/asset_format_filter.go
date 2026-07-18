package mysqlrepo

import (
	"strings"

	"workflow/domain"
)

type assetFormatCategorySpec struct {
	extensions   []string
	mimePrefixes []string
	mimeValues   []string
}

var assetFormatCategorySpecs = map[domain.AssetFormatCategoryFilter]assetFormatCategorySpec{
	domain.AssetFormatCategoryImage: {
		extensions:   []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg", ".tif", ".tiff"},
		mimePrefixes: []string{"image/"},
	},
	domain.AssetFormatCategoryDesign: {
		extensions: []string{".psd", ".psb", ".ai", ".cdr", ".eps", ".indd"},
		mimeValues: []string{
			"application/postscript",
			"application/illustrator",
			"application/vnd.adobe.photoshop",
			"image/vnd.adobe.photoshop",
		},
	},
	domain.AssetFormatCategoryPDF: {
		extensions: []string{".pdf"},
		mimeValues: []string{"application/pdf"},
	},
	domain.AssetFormatCategoryVideo: {
		extensions:   []string{".mp4", ".mov", ".m4v", ".avi", ".mkv", ".webm"},
		mimePrefixes: []string{"video/"},
	},
	domain.AssetFormatCategoryArchive: {
		extensions: []string{".zip", ".rar", ".7z", ".tar", ".gz", ".tgz"},
		mimeValues: []string{
			"application/zip",
			"application/x-zip-compressed",
			"application/x-rar-compressed",
			"application/x-7z-compressed",
			"application/x-tar",
			"application/gzip",
		},
	},
}

func appendAssetFormatCategoryWhere(clauses []string, args []interface{}, fileExprs []string, mimeExpr string, category domain.AssetFormatCategoryFilter) ([]string, []interface{}) {
	category = normalizeAssetFormatCategoryForSQL(category)
	categories := []domain.AssetFormatCategoryFilter{category}
	if category == domain.AssetFormatCategoryAll {
		categories = []domain.AssetFormatCategoryFilter{
			domain.AssetFormatCategoryImage,
			domain.AssetFormatCategoryDesign,
			domain.AssetFormatCategoryPDF,
			domain.AssetFormatCategoryVideo,
			domain.AssetFormatCategoryArchive,
		}
	}

	var parts []string
	for _, c := range categories {
		spec, ok := assetFormatCategorySpecs[c]
		if !ok {
			continue
		}
		for _, ext := range spec.extensions {
			ext = strings.ToLower(strings.TrimSpace(ext))
			if ext == "" {
				continue
			}
			for _, expr := range fileExprs {
				parts = append(parts, expr+" LIKE ?")
				args = append(args, "%"+ext)
			}
		}
		if mimeExpr != "" {
			for _, prefix := range spec.mimePrefixes {
				prefix = strings.ToLower(strings.TrimSpace(prefix))
				if prefix == "" {
					continue
				}
				parts = append(parts, mimeExpr+" LIKE ?")
				args = append(args, prefix+"%")
			}
			for _, value := range spec.mimeValues {
				value = strings.ToLower(strings.TrimSpace(value))
				if value == "" {
					continue
				}
				parts = append(parts, mimeExpr+" = ?")
				args = append(args, value)
			}
		}
	}
	if len(parts) == 0 {
		return clauses, args
	}
	clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	return clauses, args
}

func normalizeAssetFormatCategoryForSQL(category domain.AssetFormatCategoryFilter) domain.AssetFormatCategoryFilter {
	switch category {
	case domain.AssetFormatCategoryImage,
		domain.AssetFormatCategoryDesign,
		domain.AssetFormatCategoryPDF,
		domain.AssetFormatCategoryVideo,
		domain.AssetFormatCategoryArchive:
		return category
	case "document":
		// Frontend historically labeled PDFs as "document"; map to the PDF bucket.
		return domain.AssetFormatCategoryPDF
	default:
		return domain.AssetFormatCategoryAll
	}
}
