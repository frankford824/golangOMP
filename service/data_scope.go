package service

// DataScope is the SQL projection of v8 EffectiveAccess for task lists.
// Organization visibility is ID-based; display names and workflow stages are
// never authorization inputs.
type DataScope struct {
	ViewAll       bool
	DepartmentIDs []int64
	TeamIDs       []int64
	UserIDs       []int64
}
