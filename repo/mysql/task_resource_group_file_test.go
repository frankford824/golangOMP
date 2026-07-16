package mysqlrepo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetResourceFileRejectsUnboundDeletedCleanedOrRevokedAssets(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if expected != "resource-file-defensive-guards" {
			return fmt.Errorf("unexpected marker %q", expected)
		}
		normalized := strings.Join(strings.Fields(actual), " ")
		for _, required := range []string{
			"ta.binding_state = 'bound'",
			"ta.deleted_at IS NULL",
			"ta.cleaned_at IS NULL",
			"ta.access_revoked_at IS NULL",
			"ta.object_deleted_at IS NULL",
		} {
			if !strings.Contains(normalized, required) {
				return fmt.Errorf("resource file query missing %q: %s", required, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("resource-file-defensive-guards").
		WithArgs(int64(501)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "mime_type", "file_size", "storage_key"}).
			AddRow(int64(501), "final.png", "image/png", int64(10), "tasks/1/final.png"))

	repository := NewTaskResourceGroupRepo(New(db))
	file, err := repository.getResourceFile(t.Context(), 501)
	if err != nil {
		t.Fatalf("getResourceFile() error = %v", err)
	}
	if file.TaskAssetID != 501 || file.StorageKey != "tasks/1/final.png" {
		t.Fatalf("file = %+v", file)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
