package domain

// TaskDetailAggregate is the frontend-oriented detail view for a task.
// It keeps child fields present even when the related collection is empty.
type TaskDetailAggregate struct {
	Task       *Task       `json:"task"`
	TaskDetail *TaskDetail `json:"task_detail"`
}
