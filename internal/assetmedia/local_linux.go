//go:build linux

package assetmedia

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func FileIdentity(info os.FileInfo) (string, int64, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", 0, fmt.Errorf("file identity unavailable")
	}
	return strconv.FormatUint(uint64(st.Dev), 10) + ":" + strconv.FormatUint(st.Ino, 10), st.Ctim.Sec*1_000_000_000 + st.Ctim.Nsec, nil
}

// OpenBeneath refuses all symlinks, including intermediate components; checking
// EvalSymlinks and then opening would leave a path replacement race.
func OpenBeneath(root, relative string) (*os.File, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return nil, fmt.Errorf("invalid local path")
	}
	parts := strings.Split(relative, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsRune(p, 0) {
			return nil, fmt.Errorf("invalid local path")
		}
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	for i, p := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, e := unix.Openat(fd, p, flags, 0)
		_ = unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), relative)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("not a regular source file")
	}
	return f, nil
}

func CheckLocalVersion(info os.FileInfo, t ReadTarget) error {
	identity, changed, err := FileIdentity(info)
	if err != nil {
		return err
	}
	if info.Size() != t.Size || info.ModTime().UnixNano() != t.ModifiedNS || changed != t.ChangedNS || identity != t.FileIdentity {
		return fmt.Errorf("source_version_changed")
	}
	return nil
}

func CloneStableSource(root, path string, t ReadTarget, destination string) error {
	if t.RootIdentity != "" {
		st, err := os.Stat(root)
		if err != nil {
			return err
		}
		id, _, err := FileIdentity(st)
		if err != nil || id != t.RootIdentity {
			return fmt.Errorf("source root identity changed")
		}
	}
	source, err := OpenBeneath(root, path)
	if err != nil {
		return err
	}
	defer source.Close()
	before, err := source.Stat()
	if err != nil {
		return err
	}
	if err = CheckLocalVersion(before, t); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	valid := false
	defer func() {
		out.Close()
		if !valid {
			_ = os.Remove(destination)
		}
	}()
	err = unix.IoctlFileClone(int(out.Fd()), int(source.Fd()))
	if err != nil {
		if err = out.Truncate(0); err != nil {
			return err
		}
		n, e := io.Copy(out, io.LimitReader(source, t.Size+1))
		if e != nil {
			return e
		}
		if n != t.Size {
			return fmt.Errorf("source_version_changed")
		}
	}
	after, err := source.Stat()
	if err != nil {
		return err
	}
	if err = CheckLocalVersion(after, t); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	valid = true
	return nil
}

func AvailableBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
