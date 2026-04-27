package domain

type SearchResultGroup struct {
	Tasks    []SearchTask    `json:"tasks"`
	Assets   []SearchAsset   `json:"assets"`
	Products []SearchProduct `json:"products"`
	Users    []SearchUser    `json:"users"`
}

type SearchTask struct {
	ID         int64   `json:"id"`
	TaskNo     string  `json:"task_no"`
	Title      *string `json:"title"`
	TaskStatus *string `json:"task_status"`
	Priority   *string `json:"priority"`
	Highlight  *string `json:"highlight"`
}

type SearchAsset struct {
	AssetID         int64   `json:"asset_id"`
	FileName        string  `json:"file_name"`
	SourceModuleKey *string `json:"source_module_key"`
	TaskID          *int64  `json:"task_id"`
}

type SearchProduct struct {
	ERPCode     string  `json:"erp_code"`
	ProductName string  `json:"product_name"`
	Category    *string `json:"category"`
}

type SearchUser struct {
	UserID         int64   `json:"user_id"`
	Username       string  `json:"username"`
	DepartmentName *string `json:"department_name"`
}
