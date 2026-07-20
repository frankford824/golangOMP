package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	minimumGoldenCases = 50
	exactRecallTarget  = 1.0
	semanticRecallGoal = 0.95
	semanticNDCGGoal   = 0.80
)

type goldenSet struct {
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Scope     string   `json:"scope"`
	Mode      string   `json:"mode"`
	Expected  []string `json:"expected"`
	Forbidden []string `json:"forbidden,omitempty"`
}

type searchResponse struct {
	Results struct {
		Tasks []struct {
			ID int64 `json:"id"`
		} `json:"tasks"`
		Assets []struct {
			AssetID         int64  `json:"asset_id"`
			ResourceGroupID int64  `json:"resource_group_id"`
			SourceType      string `json:"source_type"`
		} `json:"assets"`
		Products []struct {
			ERPCode string `json:"erp_code"`
			IID     string `json:"i_id"`
		} `json:"products"`
		Users []struct {
			UserID int64 `json:"user_id"`
		} `json:"users"`
	} `json:"results"`
}

type evaluationReport struct {
	Cases              int      `json:"cases"`
	ExactCases         int      `json:"exact_cases"`
	SemanticCases      int      `json:"semantic_cases"`
	ExactRecallAt1     float64  `json:"exact_recall_at_1"`
	SemanticRecallAt10 float64  `json:"semantic_recall_at_10"`
	SemanticNDCGAt10   float64  `json:"semantic_ndcg_at_10"`
	LeakageCases       []string `json:"leakage_cases"`
	FailedCases        []string `json:"failed_cases"`
	Passed             bool     `json:"passed"`
}

func main() {
	var baseURL, token, goldenPath string
	var timeout time.Duration
	flag.StringVar(&baseURL, "base-url", "http://127.0.0.1:8080", "backend base URL")
	flag.StringVar(&token, "token", os.Getenv("AI_RETRIEVAL_EVAL_TOKEN"), "Bearer token; defaults to AI_RETRIEVAL_EVAL_TOKEN")
	flag.StringVar(&goldenPath, "golden", "", "path to the reviewed 50+ case golden JSON")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "whole evaluation timeout")
	flag.Parse()

	report, err := run(context.Background(), baseURL, token, goldenPath, timeout)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(parent context.Context, baseURL, token, goldenPath string, timeout time.Duration) (evaluationReport, error) {
	if strings.TrimSpace(token) == "" {
		return evaluationReport{}, fmt.Errorf("evaluation bearer token is required")
	}
	if strings.TrimSpace(goldenPath) == "" {
		return evaluationReport{}, fmt.Errorf("--golden is required")
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		return evaluationReport{}, err
	}
	var set goldenSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return evaluationReport{}, fmt.Errorf("decode golden set: %w", err)
	}
	if err := validateGoldenSet(set); err != nil {
		return evaluationReport{}, err
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	results := make(map[string][]string, len(set.Cases))
	for _, item := range set.Cases {
		items, err := executeCase(ctx, client, baseURL, token, item)
		if err != nil {
			return evaluationReport{}, fmt.Errorf("case %s: %w", item.ID, err)
		}
		results[item.ID] = items
	}
	report := evaluate(set, results)
	if !report.Passed {
		return report, fmt.Errorf("retrieval golden gate failed")
	}
	return report, nil
}

func validateGoldenSet(set goldenSet) error {
	if len(set.Cases) < minimumGoldenCases {
		return fmt.Errorf("golden set requires at least %d cases", minimumGoldenCases)
	}
	ids := make(map[string]struct{}, len(set.Cases))
	exact, semantic := 0, 0
	for index, item := range set.Cases {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" || strings.TrimSpace(item.Query) == "" || len(item.Expected) == 0 {
			return fmt.Errorf("golden case %d is incomplete", index+1)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("duplicate golden case id %q", item.ID)
		}
		ids[item.ID] = struct{}{}
		switch strings.ToLower(strings.TrimSpace(item.Mode)) {
		case "exact":
			exact++
		case "hybrid":
			semantic++
		default:
			return fmt.Errorf("case %s mode must be exact or hybrid", item.ID)
		}
	}
	if exact == 0 || semantic == 0 {
		return fmt.Errorf("golden set must include exact and hybrid cases")
	}
	return nil
}

func executeCase(ctx context.Context, client *http.Client, baseURL, token string, item goldenCase) ([]string, error) {
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/v1/search")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("q", item.Query)
	query.Set("scope", firstNonEmpty(strings.TrimSpace(item.Scope), "all"))
	query.Set("mode", strings.ToLower(strings.TrimSpace(item.Mode)))
	query.Set("limit", "50")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned HTTP %d", resp.StatusCode)
	}
	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	items := make([]string, 0, 50)
	for _, task := range payload.Results.Tasks {
		items = append(items, fmt.Sprintf("task:%d", task.ID))
	}
	for _, asset := range payload.Results.Assets {
		switch {
		case asset.ResourceGroupID > 0:
			items = append(items, fmt.Sprintf("task_resource_group:%d", asset.ResourceGroupID))
		case asset.SourceType == "external" || asset.SourceType == "external_asset":
			items = append(items, fmt.Sprintf("external_asset:%d", asset.AssetID))
		case asset.AssetID > 0:
			items = append(items, fmt.Sprintf("system_asset:%d", asset.AssetID))
		}
	}
	for _, product := range payload.Results.Products {
		key := strings.TrimSpace(product.ERPCode)
		if key == "" {
			key = strings.TrimSpace(product.IID)
		}
		if key != "" {
			items = append(items, "product:"+key)
		}
	}
	for _, user := range payload.Results.Users {
		if user.UserID > 0 {
			items = append(items, fmt.Sprintf("user:%d", user.UserID))
		}
	}
	if len(items) > 50 {
		items = items[:50]
	}
	return items, nil
}

func evaluate(set goldenSet, results map[string][]string) evaluationReport {
	report := evaluationReport{Cases: len(set.Cases), LeakageCases: []string{}, FailedCases: []string{}}
	var exactRecall, semanticRecall, semanticNDCG float64
	for _, item := range set.Cases {
		actual := results[item.ID]
		forbidden := makeStringSet(item.Forbidden)
		leaked := false
		for _, key := range actual {
			if _, exists := forbidden[key]; exists {
				leaked = true
				break
			}
		}
		if leaked {
			report.LeakageCases = append(report.LeakageCases, item.ID)
		}
		mode := strings.ToLower(strings.TrimSpace(item.Mode))
		if mode == "exact" {
			report.ExactCases++
			score := recallAtK(actual, item.Expected, 1)
			exactRecall += score
			if score < exactRecallTarget {
				report.FailedCases = append(report.FailedCases, item.ID)
			}
			continue
		}
		report.SemanticCases++
		recall := recallAtK(actual, item.Expected, 10)
		ndcg := ndcgAtK(actual, item.Expected, 10)
		semanticRecall += recall
		semanticNDCG += ndcg
		if recall < semanticRecallGoal || ndcg < semanticNDCGGoal {
			report.FailedCases = append(report.FailedCases, item.ID)
		}
	}
	if report.ExactCases > 0 {
		report.ExactRecallAt1 = exactRecall / float64(report.ExactCases)
	}
	if report.SemanticCases > 0 {
		report.SemanticRecallAt10 = semanticRecall / float64(report.SemanticCases)
		report.SemanticNDCGAt10 = semanticNDCG / float64(report.SemanticCases)
	}
	report.Passed = report.Cases >= minimumGoldenCases && report.ExactRecallAt1 >= exactRecallTarget &&
		report.SemanticRecallAt10 >= semanticRecallGoal && report.SemanticNDCGAt10 >= semanticNDCGGoal &&
		len(report.LeakageCases) == 0
	return report
}

func recallAtK(actual, expected []string, k int) float64 {
	if len(expected) == 0 {
		return 0
	}
	wanted := makeStringSet(expected)
	seen := map[string]struct{}{}
	for index, key := range actual {
		if index >= k {
			break
		}
		if _, exists := wanted[key]; exists {
			seen[key] = struct{}{}
		}
	}
	return float64(len(seen)) / float64(len(wanted))
}

func ndcgAtK(actual, expected []string, k int) float64 {
	wanted := makeStringSet(expected)
	if len(wanted) == 0 {
		return 0
	}
	dcg := 0.0
	for index, key := range actual {
		if index >= k {
			break
		}
		if _, exists := wanted[key]; exists {
			dcg += 1 / math.Log2(float64(index+2))
		}
	}
	ideal := 0.0
	for index := 0; index < min(k, len(wanted)); index++ {
		ideal += 1 / math.Log2(float64(index+2))
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func makeStringSet(items []string) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			result[item] = struct{}{}
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
