package domain

import "time"

type OrgDepartment struct {
	ID        int64     `db:"id"         json:"id"`
	Name      string    `db:"name"       json:"name"`
	Enabled   bool      `db:"enabled"    json:"enabled"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type OrgTeam struct {
	ID           int64     `db:"id"            json:"id"`
	DepartmentID int64     `db:"department_id" json:"department_id"`
	Department   string    `db:"department"    json:"department"`
	Name         string    `db:"name"          json:"name"`
	Enabled      bool      `db:"enabled"       json:"enabled"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

type OrgTeamOption struct {
	ID      int64  `json:"id,omitempty"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled,omitempty"`
}
