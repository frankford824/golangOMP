package assetworkbench

import (
	"context"
	"fmt"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type conflictingProfileRepo struct{ profileListRepo }

func (r *conflictingProfileRepo) UpsertProfile(context.Context, repo.Tx, *domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, error) {
	return nil, fmt.Errorf("save profile: %w", domain.NewAppError(domain.ErrCodeConflict, "手机号已被占用", map[string]interface{}{"deny_code": "profile_identity_conflict"}))
}

func TestSelfAndManagedProfilePreserveIdentityConflict(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(fmt.Sprintf("managed=%t", managed), func(t *testing.T) {
			r := &conflictingProfileRepo{}
			s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
			params := UpsertProfileParams{RealName: "测试", Phone: "13800000991", Province: "浙江", City: "杭州", IDCard: "330100199001010078", Gender: "female", AlipayAccount: "test@example.com"}
			actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}
			var appErr *domain.AppError
			if managed {
				actor = domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleSuperAdmin}}
				_, appErr = s.HRUpsertProfile(context.Background(), actor, 77, params)
			} else {
				_, appErr = s.UpsertMyProfile(context.Background(), actor, params)
			}
			if appErr == nil || appErr.Code != domain.ErrCodeConflict {
				t.Fatalf("error=%v, want profile CONFLICT instead of 500", appErr)
			}
		})
	}
}

type employmentWriterStub struct {
	repo.UserRepo
	employment domain.EmploymentType
}

func TestFulltimeRegistrationHasCompleteProfileAndActiveMembership(t *testing.T) {
	r := &registerProfileRepo{}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}), WithIdentityRegistrar(&registerIdentityStub{}))
	result, appErr := s.Register(context.Background(), RegisterParams{
		Account: "full-worker", Name: "测试", Phone: "13800000991", Password: "Pass1234", WorkerType: "fulltime",
		Province: "浙江", City: "杭州", IDCard: "33010019900101007x", Gender: "female", AlipayAccount: "test@example.com",
	})
	if appErr != nil {
		t.Fatal(appErr)
	}
	if result.Profile.WorkerType != "fulltime" || result.Profile.JobGrade != "" || !result.Profile.PIICompleted {
		t.Fatal("registration profile is inconsistent")
	}
	if r.membership == nil || r.membership.Status != domain.AppMembershipStatusActive {
		t.Fatal("registration did not activate membership")
	}
	if result.Auth.User.EmploymentType != domain.EmploymentTypeFullTime {
		t.Fatal("base user type is inconsistent")
	}
}

func (w *employmentWriterStub) UpdateEmploymentType(_ context.Context, _ repo.Tx, _ int64, employment domain.EmploymentType, _ time.Time) error {
	w.employment = employment
	return nil
}

func TestHRProfileSynchronizesEmploymentTypeAndNormalizesIDCard(t *testing.T) {
	r := &profileListRepo{}
	w := &employmentWriterStub{}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}), WithUserRepository(w))
	profile, appErr := s.HRUpsertProfile(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleSuperAdmin}}, 77, UpsertProfileParams{
		WorkerType: "fulltime", RealName: "测试", IDCard: "33010019900101007x",
	})
	if appErr != nil {
		t.Fatal(appErr)
	}
	if w.employment != domain.EmploymentTypeFullTime || profile.WorkerType != "fulltime" {
		t.Fatal("HR worker types diverged")
	}
	if profile.IDCard == nil || *profile.IDCard != "33010019900101007X" {
		t.Fatal("ID suffix was not normalized")
	}
}

func TestSelfProfileCannotChangePricingIdentity(t *testing.T) {
	existing := &domain.AssetWorkbenchProfile{WorkerType: "parttime", JobGrade: "J1", Status: "active"}
	p := preserveSelfManagedProfileFields(UpsertProfileParams{WorkerType: "fulltime", JobGrade: "P1", Status: "suspended"}, existing, time.Now())
	if p.WorkerType != "parttime" || p.JobGrade != "J1" || p.Status != "active" {
		t.Fatal("self profile changed managed pricing fields")
	}
}
