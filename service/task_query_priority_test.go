package service

import (
	"testing"

	"workflow/domain"
)

func TestMatchesTaskFilterPriority(t *testing.T) {
	item := &domain.TaskListItem{Priority: domain.TaskPriorityCritical}
	filter := TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Priorities: []domain.TaskPriority{domain.TaskPriorityCritical},
		},
	}
	if !matchesTaskFilter(item, filter) {
		t.Fatal("expected critical task to match critical priority filter")
	}

	filter.Priorities = []domain.TaskPriority{domain.TaskPriorityNormal}
	if matchesTaskFilter(item, filter) {
		t.Fatal("expected critical task not to match normal priority filter")
	}
}
