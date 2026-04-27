package domain

import (
	"encoding/json"
	"time"
)

type IntegrationConnectorKey string

const (
	IntegrationConnectorKeyERPProductStub            IntegrationConnectorKey = "erp_product_stub"
	IntegrationConnectorKeyERPBridgeProductUpsert    IntegrationConnectorKey = "erp_bridge_product_upsert"
	IntegrationConnectorKeyERPBridgeItemStyleUpdate  IntegrationConnectorKey = "erp_bridge_item_style_update"
	IntegrationConnectorKeyERPBridgeProductShelve    IntegrationConnectorKey = "erp_bridge_product_shelve_batch"
	IntegrationConnectorKeyERPBridgeProductUnshelve  IntegrationConnectorKey = "erp_bridge_product_unshelve_batch"
	IntegrationConnectorKeyERPBridgeVirtualInventory IntegrationConnectorKey = "erp_bridge_inventory_virtual_qty"
	IntegrationConnectorKeyExportAdapterBridge       IntegrationConnectorKey = "export_adapter_bridge"
)

func (k IntegrationConnectorKey) Valid() bool {
	switch k {
	case IntegrationConnectorKeyERPProductStub,
		IntegrationConnectorKeyERPBridgeProductUpsert,
		IntegrationConnectorKeyERPBridgeItemStyleUpdate,
		IntegrationConnectorKeyERPBridgeProductShelve,
		IntegrationConnectorKeyERPBridgeProductUnshelve,
		IntegrationConnectorKeyERPBridgeVirtualInventory,
		IntegrationConnectorKeyExportAdapterBridge:
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

func (d IntegrationCallDirection) Valid() bool {
	switch d {
	case IntegrationCallDirectionOutbound, IntegrationCallDirectionInbound:
		return true
	default:
		return false
	}
}

type IntegrationCallStatus string

const (
	IntegrationCallStatusQueued    IntegrationCallStatus = "queued"
	IntegrationCallStatusSent      IntegrationCallStatus = "sent"
	IntegrationCallStatusSucceeded IntegrationCallStatus = "succeeded"
	IntegrationCallStatusFailed    IntegrationCallStatus = "failed"
	IntegrationCallStatusCancelled IntegrationCallStatus = "cancelled"
)

func (s IntegrationCallStatus) Valid() bool {
	switch s {
	case IntegrationCallStatusQueued, IntegrationCallStatusSent, IntegrationCallStatusSucceeded, IntegrationCallStatusFailed, IntegrationCallStatusCancelled:
		return true
	default:
		return false
	}
}

type IntegrationExecutionMode string

const (
	IntegrationExecutionModeManualPlaceholderAdapter IntegrationExecutionMode = "manual_placeholder_adapter"
)

func (m IntegrationExecutionMode) Valid() bool {
	switch m {
	case IntegrationExecutionModeManualPlaceholderAdapter:
		return true
	default:
		return false
	}
}

type IntegrationExecutionStatus string

const (
	IntegrationExecutionStatusPrepared   IntegrationExecutionStatus = "prepared"
	IntegrationExecutionStatusDispatched IntegrationExecutionStatus = "dispatched"
	IntegrationExecutionStatusReceived   IntegrationExecutionStatus = "received"
	IntegrationExecutionStatusCompleted  IntegrationExecutionStatus = "completed"
	IntegrationExecutionStatusFailed     IntegrationExecutionStatus = "failed"
	IntegrationExecutionStatusCancelled  IntegrationExecutionStatus = "cancelled"
)

func (s IntegrationExecutionStatus) Valid() bool {
	switch s {
	case IntegrationExecutionStatusPrepared, IntegrationExecutionStatusDispatched, IntegrationExecutionStatusReceived, IntegrationExecutionStatusCompleted, IntegrationExecutionStatusFailed, IntegrationExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

func IntegrationExecutionTerminal(status IntegrationExecutionStatus) bool {
	return status == IntegrationExecutionStatusCompleted || status == IntegrationExecutionStatusFailed || status == IntegrationExecutionStatusCancelled
}

type IntegrationExecutionActionType string

const (
	IntegrationExecutionActionTypeStart  IntegrationExecutionActionType = "start"
	IntegrationExecutionActionTypeRetry  IntegrationExecutionActionType = "retry"
	IntegrationExecutionActionTypeReplay IntegrationExecutionActionType = "replay"
	IntegrationExecutionActionTypeCompat IntegrationExecutionActionType = "compat"
)

func (t IntegrationExecutionActionType) Valid() bool {
	switch t {
	case IntegrationExecutionActionTypeStart, IntegrationExecutionActionTypeRetry, IntegrationExecutionActionTypeReplay, IntegrationExecutionActionTypeCompat:
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

type IntegrationConnector struct {
	Key             IntegrationConnectorKey  `json:"key"`
	Name            string                   `json:"name"`
	Description     string                   `json:"description,omitempty"`
	Direction       IntegrationCallDirection `json:"direction"`
	PlaceholderOnly bool                     `json:"placeholder_only"`
}

type IntegrationExecution struct {
	ExecutionID        string                         `json:"execution_id"`
	CallLogID          int64                          `json:"call_log_id"`
	ConnectorKey       IntegrationConnectorKey        `json:"connector_key"`
	ExecutionNo        int                            `json:"execution_no"`
	ExecutionMode      IntegrationExecutionMode       `json:"execution_mode"`
	ActionType         IntegrationExecutionActionType `json:"action_type"`
	AdapterMode        BoundaryAdapterMode            `json:"adapter_mode"`
	DispatchMode       BoundaryDispatchMode           `json:"dispatch_mode"`
	TriggerSource      string                         `json:"trigger_source"`
	Status             IntegrationExecutionStatus     `json:"status"`
	LatestStatusAt     time.Time                      `json:"latest_status_at"`
	StartedAt          time.Time                      `json:"started_at"`
	FinishedAt         *time.Time                     `json:"finished_at,omitempty"`
	ErrorMessage       string                         `json:"error_message,omitempty"`
	AdapterNote        string                         `json:"adapter_note,omitempty"`
	Retryable          bool                           `json:"retryable"`
	AdapterRefSummary  *AdapterRefSummary             `json:"adapter_ref_summary,omitempty"`
	HandoffRefSummary  *HandoffRefSummary             `json:"handoff_ref_summary,omitempty"`
	PolicyMode         PolicyMode                     `json:"policy_mode,omitempty"`
	VisibleToRoles     []Role                         `json:"visible_to_roles,omitempty"`
	ActionRoles        []ActionPolicySummary          `json:"action_roles,omitempty"`
	PolicyScopeSummary *PolicyScopeSummary            `json:"policy_scope_summary,omitempty"`
	CreatedAt          time.Time                      `json:"created_at"`
	UpdatedAt          time.Time                      `json:"updated_at"`
}

type IntegrationExecutionActionSummary struct {
	ActionType     IntegrationExecutionActionType `json:"action_type"`
	ExecutionID    string                         `json:"execution_id"`
	ExecutionNo    int                            `json:"execution_no"`
	TriggerSource  string                         `json:"trigger_source"`
	Status         IntegrationExecutionStatus     `json:"status"`
	Retryable      bool                           `json:"retryable"`
	LatestStatusAt time.Time                      `json:"latest_status_at"`
	FinishedAt     *time.Time                     `json:"finished_at,omitempty"`
}

type IntegrationCallLog struct {
	CallLogID           int64                              `json:"call_log_id"`
	ConnectorKey        IntegrationConnectorKey            `json:"connector_key"`
	OperationKey        string                             `json:"operation_key"`
	Direction           IntegrationCallDirection           `json:"direction"`
	ResourceType        string                             `json:"resource_type,omitempty"`
	ResourceID          *int64                             `json:"resource_id,omitempty"`
	Status              IntegrationCallStatus              `json:"status"`
	ProgressHint        IntegrationCallProgressHint        `json:"progress_hint"`
	AdapterMode         BoundaryAdapterMode                `json:"adapter_mode"`
	DispatchMode        BoundaryDispatchMode               `json:"dispatch_mode"`
	RequestedBy         RequestActor                       `json:"requested_by"`
	RequestPayload      json.RawMessage                    `json:"request_payload,omitempty"`
	ResponsePayload     json.RawMessage                    `json:"response_payload,omitempty"`
	ErrorMessage        string                             `json:"error_message,omitempty"`
	LatestStatusAt      time.Time                          `json:"latest_status_at"`
	StartedAt           *time.Time                         `json:"started_at,omitempty"`
	FinishedAt          *time.Time                         `json:"finished_at,omitempty"`
	ExecutionCount      int64                              `json:"execution_count"`
	LatestExecution     *IntegrationExecution              `json:"latest_execution,omitempty"`
	CanRetry            bool                               `json:"can_retry"`
	CanReplay           bool                               `json:"can_replay"`
	RetryCount          int64                              `json:"retry_count"`
	ReplayCount         int64                              `json:"replay_count"`
	LatestRetryAction   *IntegrationExecutionActionSummary `json:"latest_retry_action,omitempty"`
	LatestReplayAction  *IntegrationExecutionActionSummary `json:"latest_replay_action,omitempty"`
	RetryabilityReason  string                             `json:"retryability_reason,omitempty"`
	ReplayabilityReason string                             `json:"replayability_reason,omitempty"`
	AdapterRefSummary   *AdapterRefSummary                 `json:"adapter_ref_summary,omitempty"`
	HandoffRefSummary   *HandoffRefSummary                 `json:"handoff_ref_summary,omitempty"`
	PolicyMode          PolicyMode                         `json:"policy_mode,omitempty"`
	VisibleToRoles      []Role                             `json:"visible_to_roles,omitempty"`
	ActionRoles         []ActionPolicySummary              `json:"action_roles,omitempty"`
	PolicyScopeSummary  *PolicyScopeSummary                `json:"policy_scope_summary,omitempty"`
	Remark              string                             `json:"remark,omitempty"`
	CreatedAt           time.Time                          `json:"created_at"`
	UpdatedAt           time.Time                          `json:"updated_at"`
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

func IntegrationExecutionActionTypeFromTriggerSource(triggerSource string) IntegrationExecutionActionType {
	switch triggerSource {
	case "manual_retry":
		return IntegrationExecutionActionTypeRetry
	case "manual_replay":
		return IntegrationExecutionActionTypeReplay
	case "call_log_advance_compat":
		return IntegrationExecutionActionTypeCompat
	default:
		return IntegrationExecutionActionTypeStart
	}
}

func BuildIntegrationExecutionActionSummary(execution *IntegrationExecution) *IntegrationExecutionActionSummary {
	if execution == nil {
		return nil
	}
	return &IntegrationExecutionActionSummary{
		ActionType:     execution.ActionType,
		ExecutionID:    execution.ExecutionID,
		ExecutionNo:    execution.ExecutionNo,
		TriggerSource:  execution.TriggerSource,
		Status:         execution.Status,
		Retryable:      execution.Retryable,
		LatestStatusAt: execution.LatestStatusAt,
		FinishedAt:     execution.FinishedAt,
	}
}

func IntegrationCallRetryState(status IntegrationCallStatus, latestExecution *IntegrationExecution) (bool, string) {
	if latestExecution != nil && !IntegrationExecutionTerminal(latestExecution.Status) {
		return false, "latest_execution_in_progress"
	}
	switch status {
	case IntegrationCallStatusFailed:
		if latestExecution == nil {
			return true, "failed_call_without_execution_retry_allowed"
		}
		if latestExecution.Status != IntegrationExecutionStatusFailed {
			return false, "latest_execution_not_failed"
		}
		if latestExecution.Retryable {
			return true, "latest_failed_execution_retryable"
		}
		return false, "latest_failed_execution_not_retryable"
	case IntegrationCallStatusCancelled:
		return false, "cancelled_call_prefers_replay"
	case IntegrationCallStatusSucceeded:
		return false, "succeeded_call_prefers_replay"
	case IntegrationCallStatusSent:
		return false, "call_log_in_flight"
	default:
		if latestExecution == nil {
			return false, "awaiting_initial_execution"
		}
		return false, "call_log_already_queued"
	}
}

func IntegrationCallReplayState(status IntegrationCallStatus, latestExecution *IntegrationExecution) (bool, string) {
	if latestExecution != nil && !IntegrationExecutionTerminal(latestExecution.Status) {
		return false, "latest_execution_in_progress"
	}
	switch status {
	case IntegrationCallStatusSucceeded:
		return true, "latest_succeeded_execution_replay_allowed"
	case IntegrationCallStatusFailed:
		return true, "latest_failed_execution_replay_allowed"
	case IntegrationCallStatusCancelled:
		return true, "latest_cancelled_execution_replay_allowed"
	case IntegrationCallStatusSent:
		return false, "call_log_in_flight"
	default:
		if latestExecution == nil {
			return false, "no_execution_available_for_replay"
		}
		return false, "call_log_already_queued"
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
	log.CanRetry, log.RetryabilityReason = IntegrationCallRetryState(log.Status, log.LatestExecution)
	log.CanReplay, log.ReplayabilityReason = IntegrationCallReplayState(log.Status, log.LatestExecution)
	log.AdapterMode = BoundaryAdapterModeCallLogThenExecution
	log.DispatchMode = BoundaryDispatchModeExecutionProgress
	log.AdapterRefSummary = BuildAdapterRefSummary("integration_connector", string(log.ConnectorKey), true, "Placeholder integration connector boundary.")
	if log.LatestExecution != nil {
		HydrateIntegrationExecutionDerived(log.LatestExecution)
		log.HandoffRefSummary = log.LatestExecution.HandoffRefSummary
	} else {
		log.HandoffRefSummary = nil
	}
	HydrateIntegrationCallLogPolicy(log)
}

func HydrateIntegrationExecutionDerived(execution *IntegrationExecution) {
	if execution == nil {
		return
	}
	execution.ActionType = IntegrationExecutionActionTypeFromTriggerSource(execution.TriggerSource)
	execution.AdapterMode = BoundaryAdapterModeCallLogThenExecution
	execution.DispatchMode = BoundaryDispatchModeExecutionProgress
	execution.AdapterRefSummary = BuildAdapterRefSummary("integration_connector", string(execution.ConnectorKey), true, "Placeholder integration connector boundary.")
	execution.HandoffRefSummary = BuildHandoffRefSummary(
		"execution",
		execution.ExecutionID,
		string(execution.Status),
		&execution.StartedAt,
		nil,
		execution.FinishedAt,
		nil,
		true,
		execution.AdapterNote,
	)
	HydrateIntegrationExecutionPolicy(execution)
}
