package domain

import "time"

type AnalyticsMetricSource string

const (
	AnalyticsMetricSourceTaskEvent     AnalyticsMetricSource = "task_event"
	AnalyticsMetricSourceWorkflowTrace AnalyticsMetricSource = "workflow_trace"
	AnalyticsMetricSourceDerived       AnalyticsMetricSource = "derived"
)

type AnalyticsMetricDefinition struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	Source          AnalyticsMetricSource `json:"source"`
	EventTypes      []string              `json:"event_types,omitempty"`
	Measures        []string              `json:"measures"`
	AllowedGroupBys []string              `json:"allowed_group_bys"`
	Strategy        string                `json:"-"`
}

type AnalyticsMetricQuery struct {
	MetricID string    `json:"metric_id"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	GroupBy  string    `json:"group_by"`
	Limit    int       `json:"limit"`
}

type AnalyticsMetricRow struct {
	Key              string  `json:"key"`
	Label            string  `json:"label"`
	EventCount       int64   `json:"event_count"`
	TaskCount        int64   `json:"task_count"`
	ActorCount       int64   `json:"actor_count"`
	AverageLatencyMS float64 `json:"average_latency_ms,omitempty"`
}

type AnalyticsMetricResult struct {
	MetricID   string               `json:"metric_id"`
	MetricName string               `json:"metric_name"`
	From       time.Time            `json:"from"`
	To         time.Time            `json:"to"`
	TimeZone   string               `json:"time_zone"`
	GroupBy    string               `json:"group_by"`
	Rows       []AnalyticsMetricRow `json:"rows"`
	Notes      []string             `json:"notes,omitempty"`
}

type AnalyticsTraceQuery struct {
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	Limit      int       `json:"limit"`
}
