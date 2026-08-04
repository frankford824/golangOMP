package domain

import (
	"encoding/json"
	"time"
)

// IntegrationConnectorKey identifies the concrete ERP bridge operation recorded
// by the runtime. Placeholder connector catalogs and execution simulators were
// removed; only connectors used by the active ERP bridge remain.
type IntegrationConnectorKey string

const (
	IntegrationConnectorKeyERPBridgeProductUpsert    IntegrationConnectorKey = "erp_bridge_product_upsert"
	IntegrationConnectorKeyERPBridgeItemStyleUpdate  IntegrationConnectorKey = "erp_bridge_item_style_update"
	IntegrationConnectorKeyERPBridgeProductShelve    IntegrationConnectorKey = "erp_bridge_product_shelve_batch"
	IntegrationConnectorKeyERPBridgeProductUnshelve  IntegrationConnectorKey = "erp_bridge_product_unshelve_batch"
	IntegrationConnectorKeyERPBridgeVirtualInventory IntegrationConnectorKey = "erp_bridge_inventory_virtual_qty"
)

func (key IntegrationConnectorKey) Valid() bool {
	switch key {
	case IntegrationConnectorKeyERPBridgeProductUpsert,
		IntegrationConnectorKeyERPBridgeItemStyleUpdate,
		IntegrationConnectorKeyERPBridgeProductShelve,
		IntegrationConnectorKeyERPBridgeProductUnshelve,
		IntegrationConnectorKeyERPBridgeVirtualInventory:
		return true
	default:
		return false
	}
}

type IntegrationCallDirection string

const (
	IntegrationCallDirectionOutbound IntegrationCallDirection = "outbound"
	IntegrationCallDirectionInbound  IntegrationCallDirection = "inbound"
)

func (direction IntegrationCallDirection) Valid() bool {
	return direction == IntegrationCallDirectionOutbound || direction == IntegrationCallDirectionInbound
}

type IntegrationCallStatus string

const (
	IntegrationCallStatusQueued    IntegrationCallStatus = "queued"
	IntegrationCallStatusSent      IntegrationCallStatus = "sent"
	IntegrationCallStatusSucceeded IntegrationCallStatus = "succeeded"
	IntegrationCallStatusFailed    IntegrationCallStatus = "failed"
	IntegrationCallStatusCancelled IntegrationCallStatus = "cancelled"
)

func (status IntegrationCallStatus) Valid() bool {
	switch status {
	case IntegrationCallStatusQueued, IntegrationCallStatusSent, IntegrationCallStatusSucceeded, IntegrationCallStatusFailed, IntegrationCallStatusCancelled:
		return true
	default:
		return false
	}
}

type IntegrationCallProgressHint string

const (
	IntegrationCallProgressHintQueued    IntegrationCallProgressHint = "queued"
	IntegrationCallProgressHintInFlight  IntegrationCallProgressHint = "in_flight"
	IntegrationCallProgressHintSucceeded IntegrationCallProgressHint = "succeeded"
	IntegrationCallProgressHintFailed    IntegrationCallProgressHint = "failed"
	IntegrationCallProgressHintCancelled IntegrationCallProgressHint = "cancelled"
)

type IntegrationCallLog struct {
	CallLogID       int64                       `json:"call_log_id"`
	ConnectorKey    IntegrationConnectorKey     `json:"connector_key"`
	OperationKey    string                      `json:"operation_key"`
	Direction       IntegrationCallDirection    `json:"direction"`
	ResourceType    string                      `json:"resource_type,omitempty"`
	ResourceID      *int64                      `json:"resource_id,omitempty"`
	Status          IntegrationCallStatus       `json:"status"`
	ProgressHint    IntegrationCallProgressHint `json:"progress_hint"`
	RequestedBy     RequestActor                `json:"requested_by"`
	RequestPayload  json.RawMessage             `json:"request_payload,omitempty"`
	ResponsePayload json.RawMessage             `json:"response_payload,omitempty"`
	ErrorMessage    string                      `json:"error_message,omitempty"`
	LatestStatusAt  time.Time                   `json:"latest_status_at"`
	StartedAt       *time.Time                  `json:"started_at,omitempty"`
	FinishedAt      *time.Time                  `json:"finished_at,omitempty"`
	Remark          string                      `json:"remark,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
}

func IntegrationCallProgressForStatus(status IntegrationCallStatus) IntegrationCallProgressHint {
	switch status {
	case IntegrationCallStatusSent:
		return IntegrationCallProgressHintInFlight
	case IntegrationCallStatusSucceeded:
		return IntegrationCallProgressHintSucceeded
	case IntegrationCallStatusFailed:
		return IntegrationCallProgressHintFailed
	case IntegrationCallStatusCancelled:
		return IntegrationCallProgressHintCancelled
	default:
		return IntegrationCallProgressHintQueued
	}
}

func HydrateIntegrationCallLogDerived(log *IntegrationCallLog) {
	if log == nil {
		return
	}
	if log.LatestStatusAt.IsZero() {
		if !log.UpdatedAt.IsZero() {
			log.LatestStatusAt = log.UpdatedAt
		} else {
			log.LatestStatusAt = log.CreatedAt
		}
	}
	log.ProgressHint = IntegrationCallProgressForStatus(log.Status)
}
