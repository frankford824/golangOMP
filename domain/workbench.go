package domain

type WorkbenchSortKey string

const (
	WorkbenchSortUpdatedAtDesc WorkbenchSortKey = "updated_at_desc"
)

func (k WorkbenchSortKey) Valid() bool {
	switch k {
	case "", WorkbenchSortUpdatedAtDesc:
		return true
	default:
		return false
	}
}

type WorkbenchPreferences struct {
	DefaultQueueKey string            `json:"default_queue_key,omitempty"`
	PinnedQueueKeys []string          `json:"pinned_queue_keys"`
	DefaultFilters  TaskQueryTemplate `json:"default_filters"`
	DefaultPageSize int               `json:"default_page_size"`
	DefaultSort     WorkbenchSortKey  `json:"default_sort"`
}

type WorkbenchPreferencesPatch struct {
	DefaultQueueKey *string            `json:"default_queue_key,omitempty"`
	PinnedQueueKeys *[]string          `json:"pinned_queue_keys,omitempty"`
	DefaultFilters  *TaskQueryTemplate `json:"default_filters,omitempty"`
	DefaultPageSize *int               `json:"default_page_size,omitempty"`
	DefaultSort     *WorkbenchSortKey  `json:"default_sort,omitempty"`
}

type WorkbenchQueueConfig struct {
	QueueKey         string        `json:"queue_key"`
	QueueName        string        `json:"queue_name"`
	QueueDescription string        `json:"queue_description,omitempty"`
	BoardView        TaskBoardView `json:"board_view"`
	TaskBoardQueueOwnershipHints
	Filters           TaskQueryFilterDefinition `json:"filters"`
	NormalizedFilters TaskQueryFilterDefinition `json:"normalized_filters"`
	QueryTemplate     TaskQueryTemplate         `json:"query_template"`
}

type WorkbenchConfig struct {
	FiltersSchema      TaskBoardFiltersSchema `json:"filters_schema"`
	SupportedSorts     []WorkbenchSortKey     `json:"supported_sorts"`
	SupportedPageSizes []int                  `json:"supported_page_sizes"`
	Queues             []WorkbenchQueueConfig `json:"queues"`
}

type WorkbenchPreferencesEnvelope struct {
	Actor           RequestActor         `json:"actor"`
	Preferences     WorkbenchPreferences `json:"preferences"`
	WorkbenchConfig WorkbenchConfig      `json:"workbench_config"`
}

type WorkbenchPreferenceRecord struct {
	ActorID       int64
	ActorRolesKey string
	AuthMode      AuthMode
	Preferences   WorkbenchPreferences
}
