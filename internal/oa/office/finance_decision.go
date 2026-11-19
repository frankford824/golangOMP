package office

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
)

type financeDecision struct {
	Decision         string `json:"decision"`
	Comment          string `json:"comment"`
	Current          *int   `json:"current"`
	Version          *int   `json:"version"`
	ApprovedAmount   *int64 `json:"approved_amount_cents"`
	AdjustmentReason string `json:"adjustment_reason"`
}

func (doc document) decisionKey(input financeDecision, actor int64) string {
	if len(doc.ApprovalSteps) == 0 || input.Version == nil || input.Current == nil {
		return ""
	}
	digest := sha256.Sum256([]byte(jsonData(struct {
		Actor int64
		Input financeDecision
	}{actor, input})))
	return hex.EncodeToString(digest[:])
}

func (doc document) validDecisionPosition(input financeDecision) bool {
	if input.Current == nil || *input.Current != doc.Current || doc.Current < 0 || doc.Current >= len(doc.Route) {
		return false
	}
	if len(doc.ApprovalSteps) == 0 {
		return input.Version == nil || *input.Version == doc.Version
	}
	if input.Version == nil || *input.Version > doc.Version || *input.Version < doc.StepVersion {
		return false
	}
	// Only independent positive votes at one AND step may use its earlier
	// version. A stale amount adjustment, return, or rejection cannot overwrite
	// another approver's action.
	return *input.Version == doc.Version || (doc.ApprovalSteps[doc.Current].Mode == "AND" &&
		input.Decision == "approve" && input.ApprovedAmount == nil && input.AdjustmentReason == "")
}

func (doc *document) approveCurrentStep(actor int64) {
	if len(doc.ApprovalSteps) > 0 && doc.ApprovalSteps[doc.Current].Mode == "AND" {
		index := slices.Index(doc.PendingApprovers, actor)
		doc.PendingApprovers = slices.Delete(doc.PendingApprovers, index, index+1)
		if len(doc.PendingApprovers) > 0 {
			return
		}
	}
	doc.Current++
	if len(doc.ApprovalSteps) > 0 {
		doc.setCurrentApprovalStep()
	}
}
