package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func validOrganizationMapping(t *testing.T, confidence string) organizationMapping {
	t.Helper()
	confirmedAt := time.Date(2026, 7, 22, 10, 30, 0, 0, time.UTC)
	item := organizationMapping{
		SubjectType:        "task",
		SubjectID:          7,
		LegacyDepartment:   "运营部",
		LegacyTeam:         "拼多多池州组",
		TargetDepartmentID: 6,
		TargetTeamID:       30,
		Confidence:         confidence,
		ReviewPolicyIDs:    []string{reviewPolicyLegacyOrgAliasLineage},
	}
	if confidence == "confirmed_auto" {
		item.ConfirmedBy = 88
		item.ConfirmedAt = confirmedAt
		item.ConfirmationNote = "reviewed against the frozen organization lineage"
	}
	item.ManifestRowHash, _ = organizationManifestRowHash(item)
	return item
}

func validAccessDecision(t *testing.T, confidence string) accessDecisionMapping {
	t.Helper()
	confirmedAt := time.Date(2026, 7, 22, 10, 30, 0, 0, time.UTC)
	item := accessDecisionMapping{
		UserID:     31,
		LegacyRole: "Warehouse",
		Action:     "no_new_grant",
		RequiredExistingAssignments: []accessAssignmentEvidence{{
			RoleCode: "member", ScopeMode: "self", SourceType: "direct",
		}},
		Confidence:      confidence,
		ReviewPolicyIDs: []string{reviewPolicyRetiredWarehouseNoGrant},
	}
	if confidence == "confirmed_auto" {
		item.ConfirmedBy = 88
		item.ConfirmedAt = confirmedAt
		item.ConfirmationNote = "reviewed retired warehouse access without a new grant"
	}
	item.ManifestRowHash, _ = accessDecisionManifestRowHash(item)
	return item
}

func TestOrganizationAndAccessCandidatesAreReportableButCannotApply(t *testing.T) {
	tests := []struct {
		name    string
		mapping mappingFile
		want    string
	}{
		{
			name: "organization",
			mapping: mappingFile{
				Version:              workflowGroupsMappingV2,
				OrganizationMappings: []organizationMapping{validOrganizationMapping(t, "proposed_review")},
			},
			want: "confidence=proposed_review cannot be applied",
		},
		{
			name: "access",
			mapping: mappingFile{
				Version:         workflowGroupsMappingV2,
				AccessDecisions: []accessDecisionMapping{validAccessDecision(t, "proposed_review")},
			},
			want: "confidence=proposed_review cannot be applied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCandidateMapping(tt.mapping); err != nil {
				t.Fatalf("candidate validation should retain the review row: %v", err)
			}
			if err := validateMapping(tt.mapping); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("formal validation error = %v", err)
			}
		})
	}
}

func TestApplyOrganizationMappingsUsesReviewedCASAndIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	task := validOrganizationMapping(t, "confirmed_auto")
	user := task
	user.SubjectType = "user"
	user.SubjectID = 9
	user.LegacyDepartment = "设计研发部"
	user.LegacyTeam = "默认组"
	user.TargetDepartmentID = 14
	user.TargetTeamID = 31
	user.ManifestRowHash, _ = organizationManifestRowHash(user)
	snapshotAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("UPDATE tasks").
		WithArgs(int64(6), int64(30), int64(7), nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE users").
		WithArgs(int64(14), int64(31), int64(9), nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := applyOrganizationMappings(context.Background(), tx, []organizationMapping{task, user}); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	mock.ExpectExec("UPDATE tasks").
		WithArgs(int64(6), int64(30), int64(7), nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COALESCE\\(owner_department").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"owner_department", "owner_team", "owner_department_id", "owner_team_id", "updated_at",
		}).AddRow("运营部", "拼多多池州组", 6, 30, snapshotAt))
	mock.ExpectExec("UPDATE users").
		WithArgs(int64(14), int64(31), int64(9), nil, nil).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COALESCE\\(department").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"department", "team", "department_id", "team_id", "updated_at",
		}).AddRow("设计研发部", "默认组", 14, 31, snapshotAt))
	if err := applyOrganizationMappings(context.Background(), tx, []organizationMapping{task, user}); err != nil {
		t.Fatalf("idempotent apply: %v", err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizationAndAccessPreflightRejectEvidenceDrift(t *testing.T) {
	t.Run("organization target is valid", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		item := validOrganizationMapping(t, "confirmed_auto")
		snapshotAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
		mock.ExpectQuery("SELECT COALESCE\\(owner_department").
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{
				"owner_department", "owner_team", "owner_department_id", "owner_team_id", "updated_at",
			}).AddRow("运营部", "拼多多池州组", nil, nil, snapshotAt))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
			WithArgs(int64(6), int64(30)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		if err := validateOrganizationMappingPreflight(context.Background(), db, item); err != nil {
			t.Fatalf("preflight: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("access assignment drift", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		item := validAccessDecision(t, "confirmed_auto")
		mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
			WithArgs(int64(31), "Warehouse").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT r.code,a.scope_mode,a.source_type,a.source_ref_id").
			WithArgs(int64(31)).
			WillReturnRows(sqlmock.NewRows([]string{
				"code", "scope_mode", "source_type", "source_ref_id",
			}).AddRow("access_admin", "global", "direct", 0))
		err = validateAccessDecisionPreflight(context.Background(), db, item)
		if err == nil || !strings.Contains(err.Error(), "evidence drifted") {
			t.Fatalf("preflight error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSnapshotStateMatchesIncludesOrganizationAndExactAccessEvidence(t *testing.T) {
	tests := []struct {
		name         string
		after        bool
		departmentID interface{}
		teamID       interface{}
	}{
		{name: "before", departmentID: nil, teamID: nil},
		{name: "after", after: true, departmentID: int64(6), teamID: int64(30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			snapshotAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

			before := organizationStateSnapshot{
				SubjectType: "task", SubjectID: 7,
				LegacyDepartment: "运营部", LegacyTeam: "拼多多池州组",
				UpdatedAt: snapshotAt,
			}
			departmentID, teamID := int64(6), int64(30)
			after := before
			after.DepartmentID = &departmentID
			after.TeamID = &teamID
			access := []accessStateSnapshot{{
				UserID: 31,
				Assignments: []accessAssignmentEvidence{{
					RoleCode: "member", ScopeMode: "self", SourceType: "direct",
				}},
			}}
			state := snapshot{
				ResourceGroups:      []resourceGroupSnapshot{},
				AfterResourceGroups: []resourceGroupSnapshot{},
				AssetBindings:       []assetBindingSnapshot{},
				AfterAssetBindings:  []assetBindingSnapshot{},
				SKUOrigins:          []skuOriginSnapshot{},
				AfterSKUOrigins:     []skuOriginSnapshot{},
				PlanningBefore:      []planningStateSnapshot{},
				PlanningAfter:       []planningStateSnapshot{},
				OrganizationBefore:  []organizationStateSnapshot{before},
				OrganizationAfter:   []organizationStateSnapshot{after},
				AccessBefore:        access,
				AccessAfter:         access,
			}

			mock.ExpectQuery("SELECT COALESCE\\(owner_department").
				WithArgs(int64(7)).
				WillReturnRows(sqlmock.NewRows([]string{
					"owner_department", "owner_team", "owner_department_id", "owner_team_id", "updated_at",
				}).AddRow("运营部", "拼多多池州组", tt.departmentID, tt.teamID, snapshotAt))
			mock.ExpectQuery("SELECT r.code,a.scope_mode,a.source_type,a.source_ref_id").
				WithArgs(int64(31)).
				WillReturnRows(sqlmock.NewRows([]string{
					"code", "scope_mode", "source_type", "source_ref_id",
				}).AddRow("member", "self", "direct", 0))
			matches, err := snapshotStateMatches(context.Background(), db, state, tt.after)
			if err != nil {
				t.Fatal(err)
			}
			if !matches {
				t.Fatal("expected snapshot evidence to match exactly")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
