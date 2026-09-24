package assetmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type artifactReceipt struct {
	cacheReceipt
	ExpiresAt time.Time `json:"expires_at"`
}

var artifactWriter sync.Mutex

func reserveArtifacts(dir string, incoming int64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var used int64
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".data") || !cacheIDValid(strings.TrimSuffix(name, ".data")) {
			continue
		}
		st, err := entry.Info()
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("nonregular artifact object")
		}
		id := strings.TrimSuffix(name, ".data")
		metaPath := filepath.Join(dir, id+".json")
		raw, err := os.ReadFile(metaPath)
		var receipt artifactReceipt
		if err == nil && json.Unmarshal(raw, &receipt) == nil && receipt.ContentID == id && !receipt.ExpiresAt.IsZero() && receipt.ExpiresAt.Before(time.Now()) {
			// Open streams retain their file descriptor on Linux. New streams
			// already reject this expired receipt, so removing it is safe.
			if err = os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
			if err = os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		used += st.Size()
	}
	if incoming < 0 || incoming > 100<<30 || used > (100<<30)-incoming {
		return fmt.Errorf("artifact quota reached")
	}
	return nil
}

// Artifact paths are derived exclusively from a cloud-assigned content identity.
// The gateway mounts this directory read-only; it never evicts worker artifacts.
func StoreArtifact(ctx context.Context, dir string, target ReadTarget, sourcePath string, expires time.Time) error {
	artifactWriter.Lock()
	defer artifactWriter.Unlock()
	if !filepath.IsAbs(dir) || !cacheIDValid(target.ContentID) {
		return fmt.Errorf("invalid artifact location")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid artifact root")
	}
	if err := reserveArtifacts(dir, target.Size); err != nil {
		return err
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()
	sha, crc, err := hashFile(ctx, in)
	if err != nil {
		return err
	}
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if st.Size() != target.Size || (target.SHA256 != "" && target.SHA256 != sha) || (target.CRC64 != "" && target.CRC64 != crc) {
		return fmt.Errorf("artifact integrity mismatch")
	}
	tmp, err := os.CreateTemp(dir, target.ContentID+"-*.part")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = io.Copy(tmp, io.LimitReader(in, target.Size+1)); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	data := filepath.Join(dir, target.ContentID+".data")
	if err = os.Rename(tmpPath, data); err != nil {
		return err
	}
	st, err = os.Stat(data)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(artifactReceipt{cacheReceipt: cacheReceipt{ContentID: target.ContentID, SourceVersion: target.SourceVersion, Size: target.Size, SHA256: sha, CRC64: crc, ModifiedNS: st.ModTime().UnixNano()}, ExpiresAt: expires})
	meta, err := os.CreateTemp(dir, target.ContentID+"-*.receipt")
	if err != nil {
		return err
	}
	metaPath := meta.Name()
	defer os.Remove(metaPath)
	if _, err = meta.Write(raw); err != nil {
		meta.Close()
		return err
	}
	if err = meta.Sync(); err != nil {
		meta.Close()
		return err
	}
	if err = meta.Close(); err != nil {
		return err
	}
	return os.Rename(metaPath, filepath.Join(dir, target.ContentID+".json"))
}

func OpenArtifact(dir string, t ReadTarget) (*os.File, error) {
	if dir == "" || !cacheIDValid(t.ContentID) {
		return nil, fmt.Errorf("artifact missing")
	}
	raw, err := os.ReadFile(filepath.Join(dir, t.ContentID+".json"))
	if err != nil {
		return nil, err
	}
	var receipt artifactReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.ContentID != t.ContentID || receipt.SourceVersion != t.SourceVersion || receipt.Size != t.Size || receipt.ExpiresAt.Before(time.Now()) || (t.SHA256 != "" && t.SHA256 != receipt.SHA256) || (t.CRC64 != "" && t.CRC64 != receipt.CRC64) {
		return nil, fmt.Errorf("artifact version unavailable")
	}
	f, err := OpenBeneath(dir, t.ContentID+".data")
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil || st.Size() != t.Size || st.ModTime().UnixNano() != receipt.ModifiedNS {
		f.Close()
		return nil, fmt.Errorf("artifact content changed")
	}
	return f, nil
}
