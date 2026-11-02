package assetworkbench

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type incomeDriveRepo struct {
	repo.AssetWorkbenchRepo
	file       *domain.AssetWorkbenchDriveFile
	visible    *bool
	profileErr error
}

func (r *incomeDriveRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return &domain.AssetWorkbenchProfile{SettlementVisible: r.visible}, r.profileErr
}
func (r *incomeDriveRepo) DriveListFiles(context.Context, repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	return []*domain.AssetWorkbenchDriveFile{r.file}, 1, nil
}
func (r *incomeDriveRepo) DriveSearchFiles(ctx context.Context, f repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	return r.DriveListFiles(ctx, f)
}
func (r *incomeDriveRepo) DriveLocateFile(context.Context, repo.AssetWorkbenchDriveFilter, int64) (*domain.AssetWorkbenchDriveFile, error) {
	return r.file, nil
}

func TestDriveIncomeVisibilityAcrossReadPaths(t *testing.T) {
	for _, visible := range []bool{false, true} {
		for _, admin := range []bool{false, true} {
			for _, path := range []string{"list", "search", "folder", "locate"} {
				t.Run(path+"/"+map[bool]string{true: "visible", false: "hidden"}[visible]+"/"+map[bool]string{true: "admin", false: "client"}[admin], func(t *testing.T) {
					r := &incomeDriveRepo{visible: &visible, file: &domain.AssetWorkbenchDriveFile{ID: 1, OriginalFilename: "file.png", GrossAmount: 123.45, PageCount: 1}}
					s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
					actor := domain.RequestActor{ID: 22, Roles: []domain.Role{domain.RoleAssetSubmitter}}
					if admin {
						actor.Roles = []domain.Role{domain.RoleAssetManager}
					}
					var file *domain.AssetWorkbenchDriveFile
					switch path {
					case "list":
						result, err := s.ListDriveFiles(context.Background(), actor, nil, false, "", "", "", "", nil, nil, "", "", 1, 50)
						if err != nil {
							t.Fatal(err)
						}
						file = result.Items[0]
					case "search":
						result, err := s.SearchDriveFiles(context.Background(), actor, "file", 1, 50)
						if err != nil {
							t.Fatal(err)
						}
						file = result.Items[0]
					case "folder":
						result, err := s.BrowseDriveFolder(context.Background(), actor, nil, false, "", 1, 50)
						if err != nil {
							t.Fatal(err)
						}
						file = result.Files[0]
					case "locate":
						var err *domain.AppError
						file, err = s.LocateDriveFile(context.Background(), actor, 1)
						if err != nil {
							t.Fatal(err)
						}
					}
					expected := 123.45
					if !visible && !admin {
						expected = 0
					}
					if file.GrossAmount != expected || file.PageCount != 1 {
						t.Fatalf("response=%+v", file)
					}
					if r.file.GrossAmount != 123.45 {
						t.Fatal("persisted wage projection was mutated")
					}
				})
			}
		}
	}
}
func TestDriveIncomeVisibilityFailsClosedOnProfileFailure(t *testing.T) {
	r := &incomeDriveRepo{profileErr: errors.New("offline")}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	if _, err := s.redactDriveIncome(context.Background(), domain.RequestActor{ID: 22}, []*domain.AssetWorkbenchDriveFile{{GrossAmount: 55}}); err == nil {
		t.Fatal("profile failure leaked income")
	}
	r.profileErr = sql.ErrNoRows
	if _, err := s.redactDriveIncome(context.Background(), domain.RequestActor{ID: 22}, []*domain.AssetWorkbenchDriveFile{{GrossAmount: 55}}); err != nil {
		t.Fatal(err)
	}
}
