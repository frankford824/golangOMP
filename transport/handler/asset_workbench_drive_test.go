package handler

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseUploadLedgerDirectoryFilters(t *testing.T) {
	tests := []struct {
		query          string
		legacyID       int64
		ownerID        int64
		workerType     string
		supplementDate string
		ids            []int64
		unassigned     bool
		invalid        bool
	}{
		{query: "dir_id=9", legacyID: 9},
		{query: "dir_id=9&unassigned=true", unassigned: true},
		{query: "dir_ids=3,1,3&unassigned=true", ids: []int64{3, 1}, unassigned: true},
		{query: "dir_ids=3,1,2", ids: []int64{3, 1, 2}},
		{query: "dir_ids=2,0", invalid: true},
		{query: "dir_ids=2,bad", invalid: true},
		{query: "dir_ids=2&dir_id=1", invalid: true},
		{query: "dir_ids=3&owner_user_id=42", ids: []int64{3}, ownerID: 42},
		{query: "owner_user_id=0", invalid: true},
		{query: "owner_user_id=bad", invalid: true},
		{query: "worker_type=fulltime", workerType: "fulltime"},
		{query: "worker_type=parttime", workerType: "parttime"},
		{query: "worker_type=unknown", invalid: true},
		{query: "supplement_date=2026-11-27", supplementDate: "2026-11-27"},
		{query: "supplement_date=2026-02-30", invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?"+tt.query, nil)
			filter, err := parseUploadLedgerFilters(c)
			if tt.invalid {
				if err == nil {
					t.Fatal("invalid ledger filter must be rejected")
				}
				return
			}
			var legacyID int64
			if filter.UploadDirectoryID != nil {
				legacyID = *filter.UploadDirectoryID
			}
			var ownerID int64
			if filter.OwnerUserID != nil {
				ownerID = *filter.OwnerUserID
			}
			if err != nil || filter.SupplementDate != tt.supplementDate || filter.WorkerType != tt.workerType || legacyID != tt.legacyID || ownerID != tt.ownerID || !reflect.DeepEqual(filter.UploadDirectoryIDs, tt.ids) || filter.Unassigned != tt.unassigned {
				t.Fatalf("unexpected filter: %+v, error: %v", filter, err)
			}
		})
	}
}
