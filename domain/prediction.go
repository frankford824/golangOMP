package domain

import "time"

type PredictionSuggestion struct {
	ID                  string            `json:"id"`
	SuggestionEventID   string            `json:"suggestion_event_id,omitempty"`
	SuggestionStableKey string            `json:"suggestion_stable_key,omitempty"`
	AttributionEligible bool              `json:"attribution_eligible"`
	Type                string            `json:"type"`
	Title               string            `json:"title"`
	Detail              string            `json:"detail,omitempty"`
	ActionLabel         string            `json:"action_label,omitempty"`
	ActionType          string            `json:"action_type,omitempty"`
	TargetType          string            `json:"target_type,omitempty"`
	TargetID            string            `json:"target_id,omitempty"`
	Confidence          string            `json:"confidence,omitempty"`
	Source              string            `json:"source,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

type PredictionBundle struct {
	Suggestions []PredictionSuggestion `json:"suggestions"`
	GeneratedAt time.Time              `json:"generated_at"`
}
