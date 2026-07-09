package aiagent

import (
	"reflect"
	"testing"
)

func TestParseSearchSemanticTermsTextNormalizesTerms(t *testing.T) {
	terms, err := ParseSearchSemanticTermsText(`{"terms":["帆布包"," 帆布包 ","canvas bag",""]}`)
	if err != nil {
		t.Fatalf("ParseSearchSemanticTermsText() error = %v", err)
	}
	want := []string{"帆布包", "canvas bag"}
	if !reflect.DeepEqual(terms.Terms, want) {
		t.Fatalf("terms=%+v want=%+v", terms.Terms, want)
	}
}

func TestParseSearchSemanticTermsTextRejectsEmptyTerms(t *testing.T) {
	if _, err := ParseSearchSemanticTermsText(`{"terms":[]}`); err == nil {
		t.Fatalf("expected error for empty terms")
	}
}
