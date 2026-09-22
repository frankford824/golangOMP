package mysqlrepo

import (
	"context"
	"workflow/domain"
	"workflow/repo"
)

// Network filing acknowledgements must not write an old business snapshot back
// over a newer local/ERP cost or specification update.
func (r *taskRepo) UpdateDetailFilingOnly(ctx context.Context, tx repo.Tx, d *domain.TaskDetail) error {
	_, err := Unwrap(tx).ExecContext(ctx, `UPDATE task_details SET filing_status=?,filing_error_message=?,filing_trigger_source=?,last_filing_attempt_at=?,last_filed_at=?,erp_sync_required=?,erp_sync_version=?,last_filing_payload_hash=?,last_filing_payload_json=?,filed_at=?,updated_at=UTC_TIMESTAMP() WHERE task_id=?`, d.FilingStatus, d.FilingErrorMessage, d.FilingTriggerSource, toNullTime(d.LastFilingAttemptAt), toNullTime(d.LastFiledAt), d.ERPSyncRequired, d.ERPSyncVersion, d.LastFilingPayloadHash, d.LastFilingPayloadJSON, toNullTime(d.FiledAt), d.TaskID)
	return err
}
