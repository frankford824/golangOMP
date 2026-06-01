package aiagent

import "time"

type KPIAnalysis struct {
	Headline       string                     `json:"headline"`
	Overview       string                     `json:"overview"`
	Highlights     []KPIAnalysisHighlight     `json:"highlights"`
	PeopleInsights []KPIAnalysisPersonInsight `json:"people_insights"`
	TaskSamples    []KPIAnalysisTaskSample    `json:"task_samples"`
	Risks          []KPIAnalysisRisk          `json:"risks"`
	Actions        []KPIAnalysisAction        `json:"actions"`
	Evidence       []string                   `json:"evidence"`
	Confidence     string                     `json:"confidence"`
	RawText        string                     `json:"raw_text,omitempty"`
	GeneratedAt    time.Time                  `json:"generated_at"`
	Model          string                     `json:"model,omitempty"`
	Provider       string                     `json:"provider,omitempty"`
}

type KPIAnalysisHighlight struct {
	Title string `json:"title"`
	Value string `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
}

type KPIAnalysisPersonInsight struct {
	Role   string `json:"role,omitempty"`
	Name   string `json:"name"`
	Metric string `json:"metric,omitempty"`
	Signal string `json:"signal"`
	Action string `json:"action,omitempty"`
}

type KPIAnalysisTaskSample struct {
	TaskNo      string   `json:"task_no"`
	TaskName    string   `json:"task_name,omitempty"`
	TaskType    string   `json:"task_type,omitempty"`
	Timeline    []string `json:"timeline"`
	Observation string   `json:"observation,omitempty"`
}

type KPIAnalysisRisk struct {
	Level  string `json:"level,omitempty"`
	Title  string `json:"title"`
	Reason string `json:"reason,omitempty"`
}

type KPIAnalysisAction struct {
	Owner  string `json:"owner,omitempty"`
	Action string `json:"action"`
	Timing string `json:"timing,omitempty"`
}
