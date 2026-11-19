package office

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestOfficeCipherBindsRecordsAndDetectsWrongKey(t *testing.T) {
	t.Setenv("OA_OFFICE_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("A", 32))))
	ciphertext, err := sealBoundOfficeValue("example", "recipient:1")
	if err != nil {
		t.Fatal(err)
	}
	value, err := openBoundOfficeValue(ciphertext, "recipient:1")
	if err != nil || value != "example" {
		t.Fatal("cannot read original value")
	}
	if _, err = openBoundOfficeValue(ciphertext, "recipient:2"); err == nil {
		t.Fatal("ciphertext from another record accepted")
	}
	t.Setenv("OA_OFFICE_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("B", 32))))
	if _, err = openBoundOfficeValue(ciphertext, "recipient:1"); err == nil {
		t.Fatal("replacement key accepted")
	}
	t.Setenv("OA_OFFICE_ENCRYPTION_KEY", "")
	if _, err = sealOfficeValue("example"); err == nil {
		t.Fatal("missing key accepted")
	}
}
