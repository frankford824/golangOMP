//go:build linux

package assetmedia

import (
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

// Hold the returned descriptor for the worker lifetime. A second worker may
// not clean or share its temporary workspace. Only known service temp names
// beneath a marked, non-source directory are recoverable after a crash.
func AcquireWorkerWorkspace(root, work string) (*os.File, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(work) {
		return nil, fmt.Errorf("absolute source and worker paths required")
	}
	if err := os.MkdirAll(work, 0700); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(work)
	if err != nil {
		return nil, err
	}
	source, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(source, resolved)
	if err != nil || resolved == "/" || (rel != ".." && !strings.HasPrefix(rel, "../")) {
		return nil, fmt.Errorf("worker workspace overlaps originals")
	}
	st, err := os.Lstat(work)
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("worker workspace must not be a symlink")
	}
	marker := filepath.Join(work, ".asset-media-worker-v1")
	if _, err = os.Stat(marker); os.IsNotExist(err) {
		entries, e := os.ReadDir(work)
		if e != nil {
			return nil, e
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("refusing unmanaged nonempty workspace")
		}
		if err = os.WriteFile(marker, []byte("asset-media-worker-v1\n"), 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	fd, err := unix.Open(filepath.Join(work, ".asset-media-worker.lock"), unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	lock := os.NewFile(uintptr(fd), "media-workspace-lock")
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("worker workspace already in use")
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		lock.Close()
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		owned := strings.HasPrefix(name, "asset-preview-")
		if len(name) > 37 && name[36] == '-' {
			if _, e := uuid.Parse(name[:36]); e == nil {
				owned = true
			}
		}
		if !owned {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		// ReadDir supplied this basename; this path cannot escape work.
		if err = os.RemoveAll(filepath.Join(work, name)); err != nil {
			lock.Close()
			return nil, err
		}
	}
	return lock, nil
}
