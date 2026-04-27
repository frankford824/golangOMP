package domain

type Task struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type ReferenceFileRef struct {
	RefID string `json:"ref_id"`
}

type TaskReadModel struct {
	Task
	ReferenceFileRefs []ReferenceFileRef `json:"reference_file_refs"`
}
