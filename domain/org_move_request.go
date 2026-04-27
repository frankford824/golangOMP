package domain

import "time"

type OrgMoveRequestState string

const (
	OrgMoveRequestStatePendingSuperAdminConfirm OrgMoveRequestState = "pending_super_admin_confirm"
	OrgMoveRequestStateApproved                 OrgMoveRequestState = "approved"
	OrgMoveRequestStateRejected                 OrgMoveRequestState = "rejected"
)

func (s OrgMoveRequestState) Valid() bool {
	switch s {
	case "", OrgMoveRequestStatePendingSuperAdminConfirm, OrgMoveRequestStateApproved, OrgMoveRequestStateRejected:
		return true
	default:
		return false
	}
}

type OrgMoveRequest struct {
	ID                 int64               `db:"id" json:"id"`
	UserID             int64               `db:"user_id" json:"user_id"`
	SourceDepartmentID int64               `db:"source_department_id" json:"source_department_id"`
	TargetDepartmentID *int64              `db:"target_department_id" json:"target_department_id"`
	SourceDepartment   string              `db:"source_department" json:"-"`
	TargetDepartment   string              `db:"target_department" json:"-"`
	State              OrgMoveRequestState `db:"state" json:"state"`
	RequestedByUserID  int64               `db:"requested_by" json:"-"`
	DecidedByUserID    *int64              `db:"resolved_by" json:"-"`
	Reason             string              `db:"reason" json:"reason,omitempty"`
	RejectReason       string              `db:"-" json:"reject_reason,omitempty"`
	ResolvedAt         *time.Time          `db:"resolved_at" json:"-"`
	CreatedAt          time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time           `db:"-" json:"updated_at"`
}

const (
	OrgMoveRequestEventRequested = "org_move_requested"
	OrgMoveRequestEventApproved  = "org_move_approved"
	OrgMoveRequestEventRejected  = "org_move_rejected"
)
