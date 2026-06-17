package aiagent

import "time"

type BusinessTrendAnalysis struct {
	Headline           string                      `json:"headline"`
	Overview           string                      `json:"overview"`
	InternalHotspots   []BusinessTrendHotspot      `json:"internal_hotspots"`
	ExternalMatches    []BusinessTrendMatch        `json:"external_matches"`
	BusinessDirections []BusinessTrendDirection    `json:"business_directions"`
	Risks              []BusinessTrendRisk         `json:"risks"`
	SourceStatuses     []BusinessTrendSourceStatus `json:"source_statuses"`
	EvidenceSamples    []BusinessTrendEvidence     `json:"evidence_samples"`
	Confidence         string                      `json:"confidence"`
	RawText            string                      `json:"raw_text,omitempty"`
	GeneratedAt        time.Time                   `json:"generated_at"`
	Model              string                      `json:"model,omitempty"`
	Provider           string                      `json:"provider,omitempty"`
}

type BusinessTrendHotspot struct {
	Topic       string   `json:"topic"`
	Count       int      `json:"count"`
	Signal      string   `json:"signal"`
	Keywords    []string `json:"keywords,omitempty"`
	TaskSamples []string `json:"task_samples,omitempty"`
}

type BusinessTrendMatch struct {
	Topic           string   `json:"topic"`
	Source          string   `json:"source"`
	Signal          string   `json:"signal"`
	BusinessMeaning string   `json:"business_meaning"`
	Evidence        []string `json:"evidence,omitempty"`
}

type BusinessTrendDirection struct {
	Title           string `json:"title"`
	Reason          string `json:"reason"`
	SuggestedAction string `json:"suggested_action"`
	Priority        string `json:"priority,omitempty"`
}

type BusinessTrendRisk struct {
	Level  string `json:"level,omitempty"`
	Title  string `json:"title"`
	Reason string `json:"reason,omitempty"`
}

type BusinessTrendSourceStatus struct {
	Source  string `json:"source"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Items   int    `json:"items,omitempty"`
}

type BusinessTrendEvidence struct {
	TaskNo    string `json:"task_no,omitempty"`
	TaskName  string `json:"task_name,omitempty"`
	Source    string `json:"source,omitempty"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at,omitempty"`
}
