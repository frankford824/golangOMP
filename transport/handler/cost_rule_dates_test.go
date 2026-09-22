package handler

import (
	"encoding/json"
	"testing"
)

func TestCostRulePatchDatePresence(t *testing.T) {
	for _, tc := range []struct {
		body     string
		from, to bool
	}{{`{}`, false, false}, {`{"effective_from":null}`, true, false}, {`{"effective_to":null}`, false, true}, {`{"effective_from":"2026-09-23T08:00:00+08:00"}`, true, false}} {
		var req patchCostRuleReq
		if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
			t.Fatal(err)
		}
		if req.EffectiveFromSet != tc.from || req.EffectiveToSet != tc.to {
			t.Fatalf("presence lost for %s", tc.body)
		}
	}
}
