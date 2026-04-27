package domain

import (
	"testing"
	"time"
)

func TestDeriveLifecycleState(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		asset TaskAsset
		task  Task
		want  AssetLifecycleState
	}{
		{name: "deleted wins", asset: TaskAsset{DeletedAt: &now, CleanedAt: &now, IsArchived: true}, task: Task{TaskStatus: TaskStatusCompleted}, want: AssetLifecycleStateDeleted},
		{name: "auto cleaned wins archive", asset: TaskAsset{CleanedAt: &now, IsArchived: true}, task: Task{TaskStatus: TaskStatusCompleted}, want: AssetLifecycleStateAutoCleaned},
		{name: "archived", asset: TaskAsset{IsArchived: true}, task: Task{TaskStatus: TaskStatusInProgress}, want: AssetLifecycleStateArchived},
		{name: "closed retained completed", asset: TaskAsset{}, task: Task{TaskStatus: TaskStatusCompleted}, want: AssetLifecycleStateClosedRetained},
		{name: "closed retained cancelled", asset: TaskAsset{}, task: Task{TaskStatus: TaskStatusCancelled}, want: AssetLifecycleStateClosedRetained},
		{name: "closed retained archived task", asset: TaskAsset{}, task: Task{TaskStatus: TaskStatusArchived}, want: AssetLifecycleStateClosedRetained},
		{name: "active", asset: TaskAsset{}, task: Task{TaskStatus: TaskStatusInProgress}, want: AssetLifecycleStateActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveLifecycleState(tt.asset, tt.task); got != tt.want {
				t.Fatalf("DeriveLifecycleState() = %q, want %q", got, tt.want)
			}
		})
	}
}
