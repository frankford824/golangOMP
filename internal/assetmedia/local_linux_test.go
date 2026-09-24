//go:build linux

package assetmedia

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenBeneathRejectsSymlinkAndTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../secret", "link/secret", "/etc/passwd", "a/../secret"} {
		f, err := OpenBeneath(root, path)
		if err == nil {
			f.Close()
			t.Errorf("opened unsafe path %q", path)
		}
	}
}

func TestStableSnapshotSurvivesSourceRewriteAndRejectsStaleVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	identity, changed, _ := FileIdentity(st)
	target := ReadTarget{Size: st.Size(), ModifiedNS: st.ModTime().UnixNano(), ChangedNS: changed, FileIdentity: identity}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := CloneStableSource(root, "file", target, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	// Some test filesystems coalesce metadata timestamps within one clock tick.
	// This case exercises a changed stat version; same-stat writes are tracked
	// separately by the watcher's durable write-event sequence.
	changedAt := st.ModTime().Add(time.Millisecond)
	if err := os.Chtimes(path, changedAt, changedAt); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(snapshot)
	if err != nil || string(got) != string(original) {
		t.Fatal("snapshot changed with original")
	}
	if err = CloneStableSource(root, "file", target, filepath.Join(t.TempDir(), "stale")); err == nil {
		t.Fatal("accepted stale source fingerprint")
	}
}
