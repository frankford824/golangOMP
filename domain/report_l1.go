package domain

type L1Card struct {
	Key   string   `json:"key"`
	Title string   `json:"title"`
	Value float64  `json:"value"`
	Unit  *string  `json:"unit"`
	Delta *float64 `json:"delta"`
}

type L1ThroughputPoint struct {
	Date      string `json:"date"`
	Created   int64  `json:"created"`
	Completed int64  `json:"completed"`
	Archived  int64  `json:"archived"`
}

type L1ModuleDwellPoint struct {
	ModuleKey       string  `json:"module_key"`
	AvgDwellSeconds float64 `json:"avg_dwell_seconds"`
	P95DwellSeconds float64 `json:"p95_dwell_seconds"`
	Samples         int64   `json:"samples"`
}
