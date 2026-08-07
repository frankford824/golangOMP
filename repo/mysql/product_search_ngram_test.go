package mysqlrepo

import (
	"reflect"
	"testing"
)

func TestProductSearchNgramsNormalizeChineseLatinAndBoundTerms(t *testing.T) {
	got := productSearchNgrams("医师节 KT板 / Poster Board", 0)
	want := []string{"医师", "师节", "kt", "t板", "po", "os", "st", "te", "er", "bo", "oa", "ar", "rd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("productSearchNgrams() = %#v, want %#v", got, want)
	}

	bounded := productSearchNgrams("abcdefghijklmnop", 3)
	if want := []string{"ab", "bc", "cd"}; !reflect.DeepEqual(bounded, want) {
		t.Fatalf("bounded productSearchNgrams() = %#v, want %#v", bounded, want)
	}
}

func TestProductSearchQueryNgramsSamplesLongQuery(t *testing.T) {
	got := productSearchQueryNgrams("abcdefghijklmnopqrstuvwxyz")
	if len(got) != productSearchQueryTermLimit {
		t.Fatalf("len(productSearchQueryNgrams()) = %d, want %d", len(got), productSearchQueryTermLimit)
	}
	if got[0] != "ab" || got[len(got)-1] != "yz" {
		t.Fatalf("productSearchQueryNgrams() must cover full query, got %#v", got)
	}
}
