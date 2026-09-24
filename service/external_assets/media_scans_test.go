package externalassets

import "testing"

func TestMediaScanCanonicalShardOwnership(t *testing.T) {
	for _, p := range []string{"/p3/定制/样本.psd", "/p3/仓库素材区/徐凯/a.jpg", "/p3/a.txt"} {
		if validScanPath(p, 0, 2) == validScanPath(p, 1, 2) {
			t.Fatalf("expected exactly one shard: %s", p)
		}
	}
	for _, p := range []string{"/p3/", "/p3/../secret", "/p3/a/../b", "/p3/a//b", "/quark/a", "/p3/a\\b", "/p3/a\x00b"} {
		if validScanPath(p, 0, 2) || validScanPath(p, 1, 2) {
			t.Fatalf("accepted invalid path %q", p)
		}
	}
}
