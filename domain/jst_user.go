package domain

// JSTUser 聚水潭 getcompanyusers 返回的单条用户数据。
// 以真实返回字段为准；loginId 若 API version=2 可能返回，不假定必然存在。
type JSTUser struct {
	UID           int64    `json:"u_id"`              // 用户编码，主关联键
	Name          string   `json:"name"`              // 用户名称
	LoginID       string   `json:"loginId,omitempty"` // 登录账号，可能不存在
	Enabled       bool     `json:"enabled"`           // 状态
	Created       string   `json:"created,omitempty"`
	Modified      string   `json:"modified,omitempty"`
	LastLoginTime string   `json:"last_login_time,omitempty"`
	PwdModified   string   `json:"pwd_modified,omitempty"`
	Remark        string   `json:"remark,omitempty"`
	RoleIDs       string   `json:"role_ids,omitempty"`
	Roles         string   `json:"roles,omitempty"`
	UGIDs         string   `json:"ug_ids,omitempty"`
	UGNames       []string `json:"ug_names,omitempty"`
	Creator       string   `json:"creator,omitempty"`
	Modifier      string   `json:"modifier,omitempty"`
	EmpID         string   `json:"empId,omitempty"`
}

// JSTUserListFilter getcompanyusers 请求参数。
type JSTUserListFilter struct {
	CurrentPage  int    `json:"current_page"`
	PageSize     int    `json:"page_size"`
	PageAction   int    `json:"page_action,omitempty"` // 0=列表+总数, 1=仅列表, 2=仅总数
	Enabled      *bool  `json:"enabled,omitempty"`
	Version      int    `json:"version,omitempty"` // 2 可返回更多字段
	LoginID      string `json:"loginId,omitempty"`
	CreatedBegin string `json:"creatd_begin,omitempty"`
	CreatedEnd   string `json:"creatd_end,omitempty"`
}

// JSTUserListResponse getcompanyusers 响应。
type JSTUserListResponse struct {
	CurrentPage string     `json:"current_page"`
	PageSize    string     `json:"page_size"`
	Count       string     `json:"count"`
	Pages       string     `json:"pages"`
	Datas       []*JSTUser `json:"datas"`
}
