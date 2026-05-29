package domain

import "testing"

func TestParseExternalAssetResourceID(t *testing.T) {
	id, ok := ParseExternalAssetResourceID("ext-42")
	if !ok || id != 42 {
		t.Fatalf("ext-42 parsed as (%d, %v), want (42, true)", id, ok)
	}
	id, ok = ParseExternalAssetResourceID("external:77")
	if !ok || id != 77 {
		t.Fatalf("external:77 parsed as (%d, %v), want (77, true)", id, ok)
	}
	if id, ok := ParseExternalAssetResourceID("42"); ok || id != 0 {
		t.Fatalf("numeric system asset id parsed as external: (%d, %v)", id, ok)
	}
	if id, ok := ParseExternalAssetResourceID("ext-0"); ok || id != 0 {
		t.Fatalf("invalid external id parsed: (%d, %v)", id, ok)
	}
}
