package service

import (
	"testing"

	"workflow/domain"
)

func TestMatchesTaskFilterPriority(t *testing.T) {
	item := &domain.TaskListItem{Priority: domain.TaskPriorityDrawing}
	filter := TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Priorities: []domain.TaskPriority{domain.TaskPriorityDrawing},
		},
	}
	if !matchesTaskFilter(item, filter) {
		t.Fatal("expected drawing task to match drawing priority filter")
	}

	filter.Priorities = []domain.TaskPriority{domain.TaskPriorityNormal}
	if matchesTaskFilter(item, filter) {
		t.Fatal("expected drawing task not to match normal priority filter")
	}
}
