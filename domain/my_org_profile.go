package domain

type MyOrgProfile struct {
	Department         string   `json:"department,omitempty"`
	Team               string   `json:"team,omitempty"`
	ManagedDepartments []string `json:"managed_departments"`
	ManagedTeams       []string `json:"managed_teams"`
	Roles              []Role   `json:"roles"`
}
