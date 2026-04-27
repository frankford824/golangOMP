package main

import (
	"database/sql"
	"strings"
	"time"
)

type moduleInstance struct {
	TaskID          int64
	ModuleKey       string
	State           string
	PoolTeamCode    string
	ClaimedBy       sql.NullInt64
	ClaimedTeamCode string
	ClaimedAt       sql.NullTime
	TerminalAt      sql.NullTime
	Data            string
}

func inferAssetModule(assetType, taskType string, customizationRequired bool) (string, bool) {
	switch assetType {
	case "reference":
		return "basic_info", true
	case "design_thumb", "preview":
		return "design", true
	case "source", "delivery":
		if customizationRequired {
			return "customization", true
		}
		if strings.Contains(taskType, "retouch") {
			return "retouch", true
		}
		return "design", true
	default:
		return "", false
	}
}

func poolTeam(moduleKey, taskType string, customizationRequired bool) string {
	switch moduleKey {
	case "design":
		return "design_standard"
	case "retouch":
		return "design_retouch"
	case "audit":
		if customizationRequired || taskType == "customer_customization" || taskType == "regular_customization" {
			return "audit_customization"
		}
		return "audit_standard"
	case "customization":
		return "customization_art"
	case "warehouse":
		return "warehouse_main"
	case "procurement":
		return "procurement_main"
	default:
		return ""
	}
}

func taskModules(t taskRow) []moduleInstance {
	base := []string{"basic_info"}
	switch {
	case t.CustomizationRequired || t.TaskType == "customer_customization" || t.TaskType == "regular_customization":
		base = append(base, "customization", "audit", "warehouse")
	case strings.Contains(t.TaskType, "retouch"):
		base = append(base, "retouch", "warehouse")
	case t.TaskType == "purchase_task":
		base = append(base, "procurement", "warehouse")
	default:
		base = append(base, "design", "audit", "warehouse")
	}

	reached := reachedModules(t)
	out := make([]moduleInstance, 0, len(base))
	for _, module := range base {
		if module != "basic_info" && !reached[module] {
			continue
		}
		out = append(out, moduleState(t, module))
	}
	return out
}

func reachedModules(t taskRow) map[string]bool {
	m := map[string]bool{"basic_info": true}
	status := t.TaskStatus
	custom := t.CustomizationRequired || t.TaskType == "customer_customization" || t.TaskType == "regular_customization"
	switch status {
	case "Draft":
	case "PendingAssign", "Assigned", "InProgress":
		if custom {
			m["customization"] = true
		} else if strings.Contains(t.TaskType, "retouch") {
			m["retouch"] = true
		} else if t.TaskType == "purchase_task" {
			m["procurement"] = true
		} else {
			m["design"] = true
		}
	case "PendingCustomizationProduction":
		m["customization"] = true
	case "PendingCustomizationReview":
		m["customization"] = true
		m["audit"] = true
	case "PendingAuditA", "RejectedByAuditA", "PendingAuditB", "RejectedByAuditB":
		if custom {
			m["customization"] = true
		} else {
			m["design"] = true
		}
		m["audit"] = true
	case "PendingOutsource", "Outsourcing", "PendingOutsourceReview":
		m["procurement"] = true
	case "PendingProductionTransfer", "PendingWarehouseQC", "RejectedByWarehouse", "PendingWarehouseReceive":
		if custom {
			m["customization"] = true
		} else if strings.Contains(t.TaskType, "retouch") {
			m["retouch"] = true
		} else if t.TaskType == "purchase_task" {
			m["procurement"] = true
		} else {
			m["design"] = true
			m["audit"] = true
		}
		if custom {
			m["audit"] = true
		}
		m["warehouse"] = true
	case "PendingClose", "Completed", "Archived", "Blocked", "Cancelled":
		for _, module := range []string{"design", "audit", "warehouse", "customization", "procurement", "retouch"} {
			m[module] = true
		}
	default:
		if custom {
			m["customization"] = true
		} else {
			m["design"] = true
		}
	}
	return m
}

func moduleState(t taskRow, module string) moduleInstance {
	spec := moduleInstance{
		TaskID:    t.ID,
		ModuleKey: module,
		State:     "closed",
		Data:      "{}",
	}
	if module == "basic_info" {
		spec.State = "active"
		return spec
	}
	status := t.TaskStatus
	terminal := sql.NullTime{Time: time.Now(), Valid: true}
	switch module {
	case "design", "retouch":
		switch status {
		case "PendingAssign":
			spec.State = "pending_claim"
			spec.PoolTeamCode = poolTeam(module, t.TaskType, t.CustomizationRequired)
			spec.TerminalAt = sql.NullTime{}
		case "Assigned", "InProgress", "RejectedByAuditA", "RejectedByAuditB":
			spec.State = "in_progress"
			spec.ClaimedBy = firstValidInt(t.DesignerID, t.CurrentHandlerID)
			spec.TerminalAt = sql.NullTime{}
		case "Blocked", "Cancelled":
			spec.State = "forcibly_closed"
			spec.TerminalAt = terminal
		default:
			spec.State = "closed"
			spec.TerminalAt = terminal
		}
	case "customization":
		switch status {
		case "PendingCustomizationProduction", "Assigned", "InProgress", "RejectedByAuditA", "RejectedByAuditB":
			if t.LastCustomizationOperatorID.Valid {
				spec.State = "in_progress"
				spec.ClaimedBy = t.LastCustomizationOperatorID
			} else {
				spec.State = "pending_claim"
				spec.PoolTeamCode = "customization_art"
			}
			spec.TerminalAt = sql.NullTime{}
		case "PendingCustomizationReview", "PendingAuditA", "PendingAuditB":
			spec.State = "submitted"
			spec.TerminalAt = sql.NullTime{}
		case "Blocked", "Cancelled":
			spec.State = "forcibly_closed"
			spec.TerminalAt = terminal
		default:
			spec.State = "closed"
			spec.TerminalAt = terminal
		}
	case "audit":
		switch status {
		case "PendingAuditA", "PendingAuditB", "PendingCustomizationReview":
			spec.State = "pending_claim"
			spec.PoolTeamCode = poolTeam(module, t.TaskType, t.CustomizationRequired)
			spec.TerminalAt = sql.NullTime{}
		case "RejectedByAuditA", "RejectedByAuditB":
			spec.State = "rejected"
			spec.TerminalAt = terminal
		case "Blocked", "Cancelled":
			spec.State = "forcibly_closed"
			spec.TerminalAt = terminal
		default:
			spec.State = "closed"
			spec.TerminalAt = terminal
		}
	case "warehouse":
		switch status {
		case "PendingProductionTransfer", "PendingWarehouseQC", "PendingWarehouseReceive":
			spec.State = "preparing"
			spec.PoolTeamCode = "warehouse_main"
			spec.TerminalAt = sql.NullTime{}
		case "RejectedByWarehouse":
			spec.State = "rejected"
			spec.TerminalAt = terminal
		case "Blocked", "Cancelled":
			spec.State = "forcibly_closed"
			spec.TerminalAt = terminal
		default:
			spec.State = "closed"
			spec.TerminalAt = terminal
		}
	case "procurement":
		switch status {
		case "PendingOutsource", "Outsourcing":
			spec.State = "in_progress"
			spec.PoolTeamCode = "procurement_main"
			spec.TerminalAt = sql.NullTime{}
		case "PendingOutsourceReview":
			spec.State = "review"
			spec.TerminalAt = sql.NullTime{}
		case "Blocked", "Cancelled":
			spec.State = "forcibly_closed"
			spec.TerminalAt = terminal
		default:
			spec.State = "closed"
			spec.TerminalAt = terminal
		}
	}
	return spec
}

func firstValidInt(values ...sql.NullInt64) sql.NullInt64 {
	for _, value := range values {
		if value.Valid && value.Int64 > 0 {
			return value
		}
	}
	return sql.NullInt64{}
}

func mapOwnerModule(ownerType string, customizationRequired bool, skuLevel bool) (string, bool) {
	if skuLevel {
		return "basic_info", false
	}
	switch strings.TrimSpace(ownerType) {
	case "task_create_reference":
		return "basic_info", false
	case "customization_reference":
		return "customization", false
	case "audit_reference":
		return "audit", false
	default:
		if customizationRequired {
			return "customization", true
		}
		return "basic_info", true
	}
}
