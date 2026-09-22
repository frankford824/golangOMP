package domain

import (
	"math"
	"time"
)

func EqualCost(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Round(*a*10000) == math.Round(*b*10000)
}
func ValidObservedCost(p *float64) bool {
	return p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) && *p >= 0 && *p <= 1e9
}

// Observation never treats an external edit as permission to replace a local
// manual price or an unacknowledged local write. Historical conflicts stay held.
func DecideCostObservation(s *CostSyncState, erp *float64) string {
	if s.LocalCost != nil && !s.LocalConfirmed {
		return "conflict"
	}
	if (erp != nil && !ValidObservedCost(erp)) || (s.LocalCost != nil && !ValidObservedCost(s.LocalCost)) {
		return "conflict"
	}
	if EqualCost(s.LocalCost, erp) {
		return "agree"
	}
	if erp != nil && *erp == 0 {
		return "conflict"
	}
	if s.Status == "conflict" {
		return "conflict"
	}
	if EqualCost(s.ERPCost, erp) {
		return "unchanged"
	}
	if !ValidObservedCost(erp) || (s.ManualLock && s.ManualOrigin != "erp") || s.Revision > s.AckRevision {
		return "conflict"
	}
	return "accept_erp"
}

type CostSyncState struct {
	ProjectedRevision int64      `json:"projected_revision"`
	LocalConfirmed    bool       `json:"local_confirmed"`
	SKUCode           string     `json:"sku_code"`
	LocalCost         *float64   `json:"local_cost"`
	ERPCost           *float64   `json:"erp_cost"`
	Revision          int64      `json:"revision"`
	AckRevision       int64      `json:"ack_revision"`
	ERPRevision       int64      `json:"erp_revision"`
	ManualLock        bool       `json:"manual_lock"`
	ManualOrigin      string     `json:"manual_origin"`
	Status            string     `json:"status"`
	NeedsCheck        bool       `json:"needs_check"`
	Reason            string     `json:"reason"`
	CheckedAt         *time.Time `json:"checked_at"`
}
type CostSyncResolution struct {
	SKUCode     string `json:"-"`
	Revision    int64  `json:"revision"`
	ERPRevision int64  `json:"erp_revision"`
	Choice      string `json:"choice"`
	Reason      string `json:"reason"`
	ActorID     int64  `json:"-"`
}
type CostSyncCoverage struct {
	Total     int64 `json:"total"`
	Tracked   int64 `json:"tracked"`
	Verified  int64 `json:"verified"`
	Conflicts int64 `json:"conflicts"`
	Unpriced  int64 `json:"unpriced"`
}
type CostSyncListResult struct {
	Data       []*CostSyncState  `json:"data"`
	Pagination PaginationMeta    `json:"pagination"`
	Coverage   *CostSyncCoverage `json:"coverage"`
}
type CostSyncDecisionResult struct {
	Accepted bool   `json:"accepted"`
	SKUCode  string `json:"sku_code"`
}
