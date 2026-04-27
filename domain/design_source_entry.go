package domain

import "time"

type DesignSourceEntry struct {
	ID            int64     `json:"id"`
	FileName      string    `json:"file_name"`
	OwnerTeamCode string    `json:"owner_team_code"`
	PreviewURL    *string   `json:"preview_url,omitempty"`
	VersionNo     *int      `json:"version_no,omitempty"`
	OriginTaskID  *int64    `json:"origin_task_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
