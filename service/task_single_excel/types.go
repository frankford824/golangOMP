package task_single_excel

import (
	"context"
	"io"

	"workflow/domain"
	"workflow/service"
)

const (
	AssistModeSingle = "single"
)

type TemplateService interface {
	Generate(ctx context.Context, taskType domain.TaskType, mode string) ([]byte, *domain.AppError)
}

type ParseService interface {
	Parse(ctx context.Context, taskType domain.TaskType, mode string, file io.Reader, opts ...ParseOption) (*ParseResult, *domain.AppError)
}

type ParseOptions struct {
	ActorID           int64
	ReferenceUploader ReferenceUploader
	IIDLookup         ERPIIDLookup
}

type ParseOption func(*ParseOptions)

type ReferenceUploader interface {
	UploadFile(ctx context.Context, params service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError)
}

type ERPIIDLookup interface {
	ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError)
}

func WithActorID(actorID int64) ParseOption {
	return func(o *ParseOptions) {
		o.ActorID = actorID
	}
}

func WithIIDLookup(lookup ERPIIDLookup) ParseOption {
	return func(o *ParseOptions) {
		o.IIDLookup = lookup
	}
}

type SingleTaskDraft struct {
	ProductIID        string `json:"product_i_id"`
	ProductName       string `json:"product_name"`
	DesignRequirement string `json:"design_requirement"`
	SpecText          string `json:"spec_text,omitempty"`
	Material          string `json:"material,omitempty"`
	MaterialOther     string `json:"material_other,omitempty"`
	Remark            string `json:"remark,omitempty"`
}

type ParseResult struct {
	TaskType   domain.TaskType  `json:"task_type"`
	Mode       string             `json:"mode"`
	Draft      *SingleTaskDraft   `json:"draft,omitempty"`
	Violations []ParseViolation   `json:"violations"`
}

type ParseViolation struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewTemplateService() TemplateService {
	return &templateService{}
}

func NewParseService() ParseService {
	return &parseService{}
}

func NewParseServiceWithDependencies(iidLookup ERPIIDLookup) ParseService {
	return &parseService{iidLookup: iidLookup}
}
