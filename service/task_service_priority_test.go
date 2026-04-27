package service

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
)

func TestMapTaskCreateTxErrorMapsMySQLCheckConstraintToInvalidRequest(t *testing.T) {
	svc := &taskService{}
	txErr := fmt.Errorf("insert task: %w", &mysql.MySQLError{
		Number:  3819,
		Message: "Check constraint 'chk_tasks_priority_v1' is violated.",
	})

	appErr := svc.mapTaskCreateTxError(CreateTaskParams{
		TaskType:   domain.TaskTypeOriginalProductDevelopment,
		SourceMode: domain.TaskSourceModeExistingProduct,
	}, txErr)

	if appErr == nil {
		t.Fatal("mapTaskCreateTxError() returned nil, want AppError")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if appErr.Message != "task field violates DB constraint: Check constraint 'chk_tasks_priority_v1' is violated." {
		t.Fatalf("AppError.Message = %q", appErr.Message)
	}
}
