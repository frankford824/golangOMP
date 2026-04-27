package service

import (
	"bytes"
	"context"
	"log"
	"reflect"
	"strings"
	"testing"

	"workflow/domain"
)

func TestListTaskOwnerTeamCompatMappingsGuardrailSet(t *testing.T) {
	gotSlice := ListTaskOwnerTeamCompatMappings()
	got := make(map[string]string, len(gotSlice))
	for _, mapping := range gotSlice {
		got[mapping.OrgTeam] = mapping.LegacyOwnerTeam
	}

	want := map[string]string{
		"淘系一组":                           "内贸运营组",
		"淘系二组":                           "内贸运营组",
		"天猫一组":                           "内贸运营组",
		"天猫二组":                           "内贸运营组",
		"拼多多南京组":                         "内贸运营组",
		"拼多多池州组":                         "内贸运营组",
		"\u8fd0\u8425\u4e00\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u4e8c\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u4e09\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u56db\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u4e94\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u516d\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u8fd0\u8425\u4e03\u7ec4":       "\u5185\u8d38\u8fd0\u8425\u7ec4",
		"\u5b9a\u5236\u7f8e\u5de5\u7ec4": "\u8bbe\u8ba1\u7ec4",
		"\u8bbe\u8ba1\u5ba1\u6838\u7ec4": "\u8bbe\u8ba1\u7ec4",
		"\u91c7\u8d2d\u7ec4":             "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
		"\u4ed3\u50a8\u7ec4":             "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
		"\u70d8\u7119\u4ed3\u50a8\u7ec4": "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTaskOwnerTeamCompatMappings() = %#v, want %#v", got, want)
	}
}

func TestConfigureTaskOrgCatalogAlignsConfiguredOrgTeamsWithTaskCreate(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{
		DepartmentTeams: map[string][]string{
			string(domain.DepartmentOperations): {"\u8fd0\u8425\u4e00\u7ec4", "\u8fd0\u8425\u4e8c\u7ec4", "\u8fd0\u8425\u4e03\u7ec4"},
			string(domain.DepartmentDesign):     {"\u5b9a\u5236\u7f8e\u5de5\u7ec4", "\u8bbe\u8ba1\u5ba1\u6838\u7ec4"},
		},
		TaskTeamMappings: map[string][]string{
			string(domain.DepartmentOperations): {"\u5185\u8d38\u8fd0\u8425\u7ec4"},
			string(domain.DepartmentDesign):     {"\u8bbe\u8ba1\u7ec4"},
		},
	})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	for _, ownerTeam := range []string{
		"\u8fd0\u8425\u4e00\u7ec4",
		"\u8fd0\u8425\u4e8c\u7ec4",
		"\u8fd0\u8425\u4e03\u7ec4",
		"\u5b9a\u5236\u7f8e\u5de5\u7ec4",
		"\u8bbe\u8ba1\u5ba1\u6838\u7ec4",
	} {
		p := normalizeCreateTaskParams(ownerTeamGuardrailBaseParams(ownerTeam))
		if appErr := validateCreateTaskEntry(context.Background(), p); appErr != nil {
			t.Fatalf("owner_team %q rejected after ConfigureTaskOrgCatalog: %+v", ownerTeam, appErr)
		}
	}
}

func TestNormalizeCreateTaskOwnerTeamGuardrailCases(t *testing.T) {
	cases := []struct {
		name              string
		ownerTeam         string
		wantNormalized    string
		wantApplied       bool
		wantSource        string
		wantErrMessage    string
		wantViolationCode string
	}{
		{
			name:           "legacy ops direct",
			ownerTeam:      "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantNormalized: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantSource:     taskOwnerTeamMappingSourceLegacyDirect,
		},
		{
			name:           "legacy design direct",
			ownerTeam:      "\u8bbe\u8ba1\u7ec4",
			wantNormalized: "\u8bbe\u8ba1\u7ec4",
			wantSource:     taskOwnerTeamMappingSourceLegacyDirect,
		},
		{
			name:           "legacy procurement direct",
			ownerTeam:      "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
			wantNormalized: "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
			wantSource:     taskOwnerTeamMappingSourceLegacyDirect,
		},
		{
			name:           "ops group 1 compat",
			ownerTeam:      "\u8fd0\u8425\u4e00\u7ec4",
			wantNormalized: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "ops group 3 compat",
			ownerTeam:      "\u8fd0\u8425\u4e09\u7ec4",
			wantNormalized: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "ops group 7 compat",
			ownerTeam:      "\u8fd0\u8425\u4e03\u7ec4",
			wantNormalized: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "custom design compat",
			ownerTeam:      "\u5b9a\u5236\u7f8e\u5de5\u7ec4",
			wantNormalized: "\u8bbe\u8ba1\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "design review compat",
			ownerTeam:      "\u8bbe\u8ba1\u5ba1\u6838\u7ec4",
			wantNormalized: "\u8bbe\u8ba1\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "procurement compat",
			ownerTeam:      "\u91c7\u8d2d\u7ec4",
			wantNormalized: "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "warehouse compat",
			ownerTeam:      "\u4ed3\u50a8\u7ec4",
			wantNormalized: "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:           "bakery warehouse compat",
			ownerTeam:      "\u70d8\u7119\u4ed3\u50a8\u7ec4",
			wantNormalized: "\u91c7\u8d2d\u4ed3\u50a8\u7ec4",
			wantApplied:    true,
			wantSource:     taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:              "missing team empty string",
			ownerTeam:         "",
			wantSource:        taskOwnerTeamMappingSourceInvalid,
			wantErrMessage:    "owner_team is required",
			wantViolationCode: "missing_owner_team",
		},
		{
			name:              "missing team whitespace",
			ownerTeam:         "   ",
			wantSource:        taskOwnerTeamMappingSourceInvalid,
			wantErrMessage:    "owner_team is required",
			wantViolationCode: "missing_owner_team",
		},
		{
			name:              "invalid unknown team",
			ownerTeam:         "\u4e0d\u5b58\u5728\u7684\u7ec4",
			wantNormalized:    "\u4e0d\u5b58\u5728\u7684\u7ec4",
			wantSource:        taskOwnerTeamMappingSourceInvalid,
			wantErrMessage:    "owner_team must be a valid configured team",
			wantViolationCode: "invalid_owner_team",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := normalizeCreateTaskParams(ownerTeamGuardrailBaseParams(tc.ownerTeam))
			if p.OwnerTeam != tc.wantNormalized {
				t.Fatalf("normalized owner_team = %q, want %q", p.OwnerTeam, tc.wantNormalized)
			}
			if p.ownerTeamMappingApplied != tc.wantApplied {
				t.Fatalf("ownerTeamMappingApplied = %t, want %t", p.ownerTeamMappingApplied, tc.wantApplied)
			}
			if p.ownerTeamMappingSource != tc.wantSource {
				t.Fatalf("ownerTeamMappingSource = %q, want %q", p.ownerTeamMappingSource, tc.wantSource)
			}

			appErr := validateCreateTaskEntry(context.Background(), p)
			if tc.wantViolationCode == "" {
				if appErr != nil {
					t.Fatalf("validateCreateTaskEntry() unexpected error: %+v", appErr)
				}
				return
			}

			if appErr == nil {
				t.Fatal("validateCreateTaskEntry() expected error")
			}
			if appErr.Code != domain.ErrCodeInvalidRequest {
				t.Fatalf("error code = %q, want %q", appErr.Code, domain.ErrCodeInvalidRequest)
			}
			if appErr.Message != tc.wantErrMessage {
				t.Fatalf("error message = %q, want %q", appErr.Message, tc.wantErrMessage)
			}
			violation := firstTaskCreateViolation(t, appErr)
			if violation["field"] != "owner_team" {
				t.Fatalf("violation field = %v, want owner_team", violation["field"])
			}
			if violation["code"] != tc.wantViolationCode {
				t.Fatalf("violation code = %v, want %q", violation["code"], tc.wantViolationCode)
			}
		})
	}
}

func TestLogCreateTaskOwnerTeamNormalizationMappingSources(t *testing.T) {
	cases := []struct {
		name                string
		ownerTeam           string
		wantNormalizedOwner string
		wantMappingApplied  bool
		wantMappingSource   string
	}{
		{
			name:                "legacy direct",
			ownerTeam:           "\u8bbe\u8ba1\u7ec4",
			wantNormalizedOwner: "\u8bbe\u8ba1\u7ec4",
			wantMappingSource:   taskOwnerTeamMappingSourceLegacyDirect,
		},
		{
			name:                "compat mapping",
			ownerTeam:           "\u8fd0\u8425\u4e09\u7ec4",
			wantNormalizedOwner: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantMappingApplied:  true,
			wantMappingSource:   taskOwnerTeamMappingSourceOrgTeamCompat,
		},
		{
			name:                "invalid mapping",
			ownerTeam:           "\u4e0d\u5b58\u5728\u7684\u7ec4",
			wantNormalizedOwner: "\u4e0d\u5b58\u5728\u7684\u7ec4",
			wantMappingSource:   taskOwnerTeamMappingSourceInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := normalizeCreateTaskParams(ownerTeamGuardrailBaseParams(tc.ownerTeam))

			var buf bytes.Buffer
			originalWriter := log.Writer()
			originalFlags := log.Flags()
			log.SetFlags(0)
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(originalWriter)
				log.SetFlags(originalFlags)
			}()

			logCreateTaskOwnerTeamNormalization(context.Background(), p)
			output := buf.String()
			if !strings.Contains(output, "mapping_source="+tc.wantMappingSource) {
				t.Fatalf("log output missing mapping_source: %s", output)
			}
			if !strings.Contains(output, "owner_team_mapping_applied="+strings.ToLower(strconvFormatBool(tc.wantMappingApplied))) {
				t.Fatalf("log output missing owner_team_mapping_applied=%t: %s", tc.wantMappingApplied, output)
			}
			if !strings.Contains(output, "raw_owner_team=\""+tc.ownerTeam+"\"") {
				t.Fatalf("log output missing raw_owner_team: %s", output)
			}
			if !strings.Contains(output, "normalized_owner_team=\""+tc.wantNormalizedOwner+"\"") {
				t.Fatalf("log output missing normalized_owner_team: %s", output)
			}
		})
	}
}

func TestTaskServiceCreateBatchOwnerTeamCompatRegression(t *testing.T) {
	cases := []struct {
		name                string
		params              CreateTaskParams
		wantOwnerTeam       string
		wantBatch           bool
		wantViolationCode   string
		wantDisallowedField string
	}{
		{
			name: "batch new product compat owner team",
			params: CreateTaskParams{
				TaskType:     domain.TaskTypeNewProductDevelopment,
				SourceMode:   domain.TaskSourceModeNewProduct,
				CreatorID:    9,
				OwnerTeam:    "\u8fd0\u8425\u4e09\u7ec4",
				DeadlineAt:   timePtr(),
				BatchSKUMode: "multiple",
				BatchItems: []CreateTaskBatchSKUItemParams{
					{ProductName: "Batch A", ProductShortName: "A", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "need design A", NewSKU: "BATCH-NEW-001"},
					{ProductName: "Batch B", ProductShortName: "B", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "need design B", NewSKU: "BATCH-NEW-002"},
				},
			},
			wantOwnerTeam: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantBatch:     true,
		},
		{
			name: "batch purchase compat owner team",
			params: CreateTaskParams{
				TaskType:     domain.TaskTypePurchaseTask,
				SourceMode:   domain.TaskSourceModeNewProduct,
				CreatorID:    9,
				OwnerTeam:    "\u8fd0\u8425\u4e09\u7ec4",
				DeadlineAt:   timePtr(),
				BatchSKUMode: "multiple",
				BatchItems: []CreateTaskBatchSKUItemParams{
					{ProductName: "Batch A", CategoryCode: "LIGHTBOX", PurchaseSKU: "BATCH-PUR-001", CostPriceMode: string(domain.CostPriceModeTemplate), Quantity: int64Ptr(10), BaseSalePrice: float64Ptr(11.5)},
					{ProductName: "Batch B", CategoryCode: "LIGHTBOX", PurchaseSKU: "BATCH-PUR-002", CostPriceMode: string(domain.CostPriceModeTemplate), Quantity: int64Ptr(12), BaseSalePrice: float64Ptr(13.5)},
				},
			},
			wantOwnerTeam: "\u5185\u8d38\u8fd0\u8425\u7ec4",
			wantBatch:     true,
		},
		{
			name: "batch original product still rejected",
			params: CreateTaskParams{
				TaskType:      domain.TaskTypeOriginalProductDevelopment,
				SourceMode:    domain.TaskSourceModeExistingProduct,
				CreatorID:     9,
				OwnerTeam:     "\u8fd0\u8425\u4e09\u7ec4",
				DeadlineAt:    timePtr(),
				ChangeRequest: "compat owner team should not be the blocker",
				ProductID:     int64Ptr(88),
				SKUCode:       "SKU-088",
				BatchSKUMode:  "multiple",
				BatchItems: []CreateTaskBatchSKUItemParams{
					{ProductName: "Batch A"},
					{ProductName: "Batch B"},
				},
			},
			wantViolationCode:   "batch_not_supported_for_task_type",
			wantDisallowedField: "batch_sku_mode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewTaskService(
				&prdTaskRepo{},
				&prdProcurementRepo{},
				&prdTaskAssetRepo{},
				&prdTaskEventRepo{},
				nil,
				&prdWarehouseRepo{},
				prdCodeRuleService{},
				step04TxRunner{},
			)

			task, appErr := svc.Create(context.Background(), tc.params)
			if tc.wantViolationCode == "" {
				if appErr != nil {
					t.Fatalf("Create() unexpected error: %+v", appErr)
				}
				if task == nil {
					t.Fatal("Create() task = nil")
				}
				if task.OwnerTeam != tc.wantOwnerTeam {
					t.Fatalf("Create() owner_team = %q, want %q", task.OwnerTeam, tc.wantOwnerTeam)
				}
				if task.IsBatchTask != tc.wantBatch {
					t.Fatalf("Create() is_batch_task = %t, want %t", task.IsBatchTask, tc.wantBatch)
				}
				return
			}

			if appErr == nil {
				t.Fatal("Create() expected error")
			}
			violation := firstTaskCreateViolation(t, appErr)
			if violation["code"] != tc.wantViolationCode {
				t.Fatalf("violation code = %v, want %q", violation["code"], tc.wantViolationCode)
			}
			if violation["field"] != tc.wantDisallowedField {
				t.Fatalf("violation field = %v, want %q", violation["field"], tc.wantDisallowedField)
			}
			if ownerTeamViolation := findTaskCreateViolation(appErr, "owner_team"); ownerTeamViolation != nil {
				t.Fatalf("unexpected owner_team violation: %#v", ownerTeamViolation)
			}
		})
	}
}

func ownerTeamGuardrailBaseParams(ownerTeam string) CreateTaskParams {
	return CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           ownerTeam,
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "AL",
		ProductNameSnapshot: "Guardrail Product",
		ProductShortName:    "Guardrail",
		DesignRequirement:   "need design",
	}
}

func firstTaskCreateViolation(t *testing.T, appErr *domain.AppError) map[string]interface{} {
	t.Helper()

	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("error details type = %#v", appErr.Details)
	}
	rawViolations, ok := details["violations"]
	if !ok {
		t.Fatalf("violations missing in details: %#v", details)
	}
	switch typed := rawViolations.(type) {
	case []map[string]interface{}:
		if len(typed) == 0 {
			t.Fatalf("violations empty: %#v", details)
		}
		return typed[0]
	case []interface{}:
		if len(typed) == 0 {
			t.Fatalf("violations empty: %#v", details)
		}
		first, ok := typed[0].(map[string]interface{})
		if !ok {
			t.Fatalf("first violation type = %#v", typed[0])
		}
		return first
	default:
		t.Fatalf("violations type = %#v", rawViolations)
		return nil
	}
}

func findTaskCreateViolation(appErr *domain.AppError, field string) map[string]interface{} {
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		return nil
	}
	rawViolations, ok := details["violations"]
	if !ok {
		return nil
	}
	switch typed := rawViolations.(type) {
	case []map[string]interface{}:
		for _, violation := range typed {
			if violation["field"] == field {
				return violation
			}
		}
	case []interface{}:
		for _, raw := range typed {
			violation, ok := raw.(map[string]interface{})
			if ok && violation["field"] == field {
				return violation
			}
		}
	}
	return nil
}

func strconvFormatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
