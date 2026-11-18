package handler

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseUploadLedgerDirectoryFilters(t *testing.T) {
	tests := []struct {
		query      string
		legacyID   int64
		ids        []int64
		unassigned bool
		invalid    bool
	}{
		{query: "dir_id=9", legacyID: 9},
		{query: "dir_id=9&unassigned=true", unassigned: true},
		{query: "dir_ids=3,1,3&unassigned=true", ids: []int64{3, 1}, unassigned: true},
		{query: "dir_ids=3,1,2", ids: []int64{3, 1, 2}},
		{query: "dir_ids=2,0", invalid: true},
		{query: "dir_ids=2,bad", invalid: true},
		{query: "dir_ids=2&dir_id=1", invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?"+tt.query, nil)
			filter, err := parseUploadLedgerFilters(c)
			if tt.invalid {
				if err == nil {
					t.Fatal("invalid category filter must be rejected")
				}
				return
			}
			var legacyID int64
			if filter.UploadDirectoryID != nil {
				legacyID = *filter.UploadDirectoryID
			}
			if err != nil || legacyID != tt.legacyID || !reflect.DeepEqual(filter.UploadDirectoryIDs, tt.ids) || filter.Unassigned != tt.unassigned {
				t.Fatalf("unexpected filter: %+v, error: %v", filter, err)
			}
		})
	}
}
