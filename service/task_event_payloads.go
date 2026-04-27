package service

import "workflow/domain"

func taskEventBasePayload(task *domain.Task) map[string]interface{} {
	payload := map[string]interface{}{}
	if task == nil {
		return payload
	}
	payload["task_no"] = task.TaskNo
	payload["task_type"] = string(task.TaskType)
	payload["source_mode"] = string(task.SourceMode)
	payload["workflow_lane"] = string(task.WorkflowLane())
	payload["source_department"] = taskSourceDepartment(task)
	payload["sku_code"] = task.SKUCode
	payload["product_name_snapshot"] = task.ProductNameSnapshot
	payload["product_id"] = cloneInt64Ptr(task.ProductID)
	return payload
}

func taskTransitionEventPayload(task *domain.Task, fromStatus, toStatus domain.TaskStatus, fromHandlerID, toHandlerID *int64, extra map[string]interface{}) map[string]interface{} {
	payload := taskEventBasePayload(task)
	payload["from_task_status"] = string(fromStatus)
	payload["to_task_status"] = string(toStatus)
	payload["from_handler_id"] = cloneInt64Ptr(fromHandlerID)
	payload["to_handler_id"] = cloneInt64Ptr(toHandlerID)
	return mergeTaskEventPayload(payload, extra)
}

func mergeTaskEventPayload(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func warehouseReceiptStatusValue(receipt *domain.WarehouseReceipt) interface{} {
	if receipt == nil || receipt.Status == "" {
		return nil
	}
	return string(receipt.Status)
}

func procurementStatusValue(record *domain.ProcurementRecord) interface{} {
	if record == nil || record.Status == "" {
		return nil
	}
	return string(record.Status)
}
