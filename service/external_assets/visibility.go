package externalassets

import (
	"context"
	pathpkg "path"
	"sort"
	"strings"

	"workflow/domain"
)

func (s *Service) visibleOriginPrefixes(queryMount string) ([]string, bool) {
	if s == nil {
		return nil, false
	}
	if strings.TrimSpace(queryMount) == "" {
		queryMount = ""
	} else {
		queryMount = cleanAListPath(queryMount)
	}
	prefixes := []string{}
	matched := false
	for _, mount := range s.cfg.Mounts {
		mountPath := cleanAListPath(mount.Path)
		if queryMount != "" && queryMount != mountPath {
			continue
		}
		matched = true
		roots, narrowed := s.visibleRootsForMount(mountPath)
		if narrowed {
			prefixes = append(prefixes, roots...)
		} else {
			prefixes = append(prefixes, mountPath)
		}
	}
	return dedupeFullSyncRoots(prefixes), matched
}

func (s *Service) visibleRootsForMount(mountPath string) ([]string, bool) {
	mountPath = cleanAListPath(mountPath)
	roots := []string{}
	for _, raw := range s.cfg.VisibleRoots {
		root := cleanAListPath(raw)
		if root == mountPath {
			return []string{mountPath}, false
		}
		if strings.HasPrefix(root, mountPath+"/") {
			roots = append(roots, root)
		}
	}
	roots = dedupeFullSyncRoots(roots)
	return roots, len(roots) > 0
}

func (s *Service) isOriginVisible(mountPath, originPath string) bool {
	roots, narrowed := s.visibleRootsForMount(mountPath)
	if !narrowed {
		return true
	}
	originPath = cleanAListPath(originPath)
	for _, root := range roots {
		if originPath == root || strings.HasPrefix(originPath, root+"/") {
			return true
		}
	}
	return false
}

func (s *Service) searchScopesForMount(mount MountConfig) []string {
	if roots, narrowed := s.visibleRootsForMount(mount.Path); narrowed {
		return roots
	}
	return []string{cleanAListPath(mount.Path)}
}

func (s *Service) browsePathVisible(mountPath, candidate string) bool {
	roots, narrowed := s.visibleRootsForMount(mountPath)
	if !narrowed {
		return true
	}
	candidate = cleanAListPath(candidate)
	if candidate == mountPath {
		return true
	}
	for _, root := range roots {
		if candidate == root || strings.HasPrefix(candidate, root+"/") || strings.HasPrefix(root, candidate+"/") {
			return true
		}
	}
	return false
}

func (s *Service) browsePathInsideVisibleRoot(mountPath, candidate string) bool {
	roots, narrowed := s.visibleRootsForMount(mountPath)
	if !narrowed {
		return true
	}
	candidate = cleanAListPath(candidate)
	for _, root := range roots {
		if candidate == root || strings.HasPrefix(candidate, root+"/") {
			return true
		}
	}
	return false
}

func (s *Service) filterVisibleDirectoryEntries(parentPath, mountPath string, entries []domain.ExternalAssetDirectoryEntry) []domain.ExternalAssetDirectoryEntry {
	roots, narrowed := s.visibleRootsForMount(mountPath)
	if !narrowed || s.browsePathInsideVisibleRoot(mountPath, parentPath) {
		return entries
	}
	out := make([]domain.ExternalAssetDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := cleanAListPath(entry.Path)
		for _, root := range roots {
			if entryPath == root || strings.HasPrefix(root, entryPath+"/") {
				out = append(out, entry)
				break
			}
		}
	}
	return out
}

func (s *Service) visibleRootDirectoryEntries(ctx context.Context, browser directoryBrowserRepo, mountPath string, formatCategory domain.AssetFormatCategoryFilter) ([]domain.ExternalAssetDirectoryEntry, error) {
	roots, narrowed := s.visibleRootsForMount(mountPath)
	if !narrowed {
		return nil, nil
	}
	parents := map[string]struct{}{}
	for _, root := range roots {
		parents[cleanAListPath(pathpkg.Dir(root))] = struct{}{}
	}
	byPath := map[string]domain.ExternalAssetDirectoryEntry{}
	for parent := range parents {
		entries, err := browser.ListDirectoryChildren(ctx, parent, []string{mountPath}, 2000, formatCategory)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			byPath[cleanAListPath(entry.Path)] = entry
		}
	}
	out := make([]domain.ExternalAssetDirectoryEntry, 0, len(roots))
	for _, root := range roots {
		entry, ok := byPath[root]
		if !ok {
			entry = domain.ExternalAssetDirectoryEntry{Path: root, Name: pathpkg.Base(root)}
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
