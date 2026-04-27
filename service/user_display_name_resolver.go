package service

import (
	"context"
	"strings"

	"workflow/repo"
)

type userRepoDisplayNameResolver struct {
	userRepo repo.UserRepo
}

// NewUserRepoDisplayNameResolver creates a UserDisplayNameResolver from UserRepo.
func NewUserRepoDisplayNameResolver(userRepo repo.UserRepo) UserDisplayNameResolver {
	return &userRepoDisplayNameResolver{userRepo: userRepo}
}

func (r *userRepoDisplayNameResolver) GetDisplayName(ctx context.Context, userID int64) string {
	user, err := r.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return ""
	}
	if s := strings.TrimSpace(user.DisplayName); s != "" {
		return s
	}
	return strings.TrimSpace(user.Username)
}
