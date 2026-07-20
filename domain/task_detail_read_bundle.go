package domain

// TaskDetailReadBundle is the database read projection for one task detail
// request. It keeps the public response shape out of the repository while
// allowing MySQL to return all detail surfaces in one multi-result roundtrip.
type TaskDetailReadBundle struct {
	Task                *Task
	TaskDetail          *TaskDetail
	Modules             []*TaskModule
	Events              []*TaskModuleEvent
	ReferenceFiles      []*ReferenceFileRefFlat
	SKUItems            []*TaskSKUItem
	TaskAssets          []*TaskAsset
	RetouchRequirements []*TaskRetouchRequirement
	UserNames           map[int64]string
}
