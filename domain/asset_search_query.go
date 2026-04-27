package domain

import "time"

type AssetArchiveFilter string

const (
	AssetArchiveFilterFalse AssetArchiveFilter = "false"
	AssetArchiveFilterTrue  AssetArchiveFilter = "true"
	AssetArchiveFilterAll   AssetArchiveFilter = "all"
)

type AssetTaskStatusFilter string

const (
	AssetTaskStatusFilterOpen     AssetTaskStatusFilter = "open"
	AssetTaskStatusFilterClosed   AssetTaskStatusFilter = "closed"
	AssetTaskStatusFilterArchived AssetTaskStatusFilter = "archived"
	AssetTaskStatusFilterAll      AssetTaskStatusFilter = "all"
)

type AssetSearchQuery struct {
	Keyword       string
	ModuleKey     string
	OwnerTeamCode string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	Page          int
	Size          int
	IsArchived    AssetArchiveFilter
	TaskStatus    AssetTaskStatusFilter
}

func (q AssetSearchQuery) Normalized() AssetSearchQuery {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > 100 {
		q.Size = 100
	}
	switch q.IsArchived {
	case AssetArchiveFilterTrue, AssetArchiveFilterAll:
	default:
		q.IsArchived = AssetArchiveFilterFalse
	}
	switch q.TaskStatus {
	case AssetTaskStatusFilterOpen, AssetTaskStatusFilterClosed, AssetTaskStatusFilterArchived:
	default:
		q.TaskStatus = AssetTaskStatusFilterAll
	}
	return q
}
