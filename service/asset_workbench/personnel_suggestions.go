package assetworkbench

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
	"golang.org/x/text/unicode/norm"

	"workflow/domain"
)

const assetWorkbenchRecentPayeesLimit = 50

type PersonnelSuggestionsResult struct {
	Items []domain.AssetWorkbenchPersonnelSuggestion `json:"items"`
	Total int                                        `json:"total"`
}

func (s *Service) SuggestPersonnel(ctx context.Context, actor domain.RequestActor, scope, query, status string) (*PersonnelSuggestionsResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	switch scope {
	case "upload_owner":
		if s.driveOwnerFilter(actor) != nil {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only site-wide upload ledger readers can search upload owners.", nil)
		}
	case "member":
		if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleSuperAdmin) ||
			(actor.EffectiveAccess != nil && !actor.EffectiveAccess.Has(domain.PermissionAssetWorkbenchMembers)) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only workbench member readers can search members.", nil)
		}
		switch status {
		case "", domain.AppMembershipStatusActive, domain.AppMembershipStatusPending, domain.AppMembershipStatusDisabled, domain.AppMembershipStatusMerged:
		default:
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid workbench membership status.", nil)
		}
	case "main_account":
		if !actorHasAny(actor, domain.RoleSuperAdmin) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only super admins can search main accounts.", nil)
		}
	case "audit_actor":
		if !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin) ||
			(actor.EffectiveAccess != nil && !actor.EffectiveAccess.Has(domain.PermissionAssetWorkbenchAuditView)) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only site-wide workbench audit readers can search event actors.", nil)
		}
	case "payee":
		if !actorHasAny(actor, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Only HR or settlement roles can search payees.", nil)
		}
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid personnel suggestion scope.", nil)
	}

	query = strings.ToLower(strings.TrimSpace(norm.NFKC.String(query)))
	if utf8.RuneCountInString(query) > 80 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "Personnel search is limited to 80 characters.", nil)
	}
	if scope != "payee" && query == "" {
		return &PersonnelSuggestionsResult{Items: []domain.AssetWorkbenchPersonnelSuggestion{}}, nil
	}
	candidates, err := s.repo.ListPersonnelSuggestionCandidates(ctx, scope, status)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to search asset workbench personnel.", nil)
	}
	return rankPersonnelSuggestions(candidates, query), nil
}

func rankPersonnelSuggestions(candidates []domain.AssetWorkbenchPersonnelSuggestion, query string) *PersonnelSuggestionsResult {
	type match struct {
		person domain.AssetWorkbenchPersonnelSuggestion
		rank   int
	}
	var matches []match
	phoneticQuery, _ := personnelPhonetics(query)
	for _, person := range candidates {
		if rank := personnelSuggestionRank(person, query, phoneticQuery); rank >= 0 {
			matches = append(matches, match{person: person, rank: rank})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].rank < matches[j].rank })

	count := len(matches)
	if query == "" {
		count = min(assetWorkbenchRecentPayeesLimit, count)
	}
	result := &PersonnelSuggestionsResult{
		Items: make([]domain.AssetWorkbenchPersonnelSuggestion, 0, count),
		Total: len(matches),
	}
	for _, item := range matches[:count] {
		result.Items = append(result.Items, item.person)
	}
	return result
}

func personnelSuggestionRank(person domain.AssetWorkbenchPersonnelSuggestion, query, phoneticQuery string) int {
	if query == "" || query == strconv.FormatInt(person.UserID, 10) {
		return 0
	}
	fields := []string{person.Name, person.AlternateName, person.Username, person.PersonnelNo}
	best := 3
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(norm.NFKC.String(field)))
		switch {
		case field == query:
			return 0
		case strings.HasPrefix(field, query):
			best = 1
		case best > 2 && strings.Contains(field, query):
			best = 2
		}
	}
	if best < 3 {
		return best
	}
	if phoneticQuery == "" {
		return -1
	}
	for _, name := range []string{person.Name, person.AlternateName} {
		full, initials := personnelPhonetics(name)
		if strings.Contains(full, phoneticQuery) || strings.Contains(initials, phoneticQuery) {
			return 3
		}
	}
	return -1
}

func personnelPhonetics(value string) (string, string) {
	var full, initials strings.Builder
	args := pinyin.NewArgs()
	for _, char := range norm.NFD.String(strings.ToLower(value)) {
		if unicode.Is(unicode.Mn, char) {
			continue
		}
		if syllables := pinyin.SinglePinyin(char, args); len(syllables) > 0 && syllables[0] != "" {
			full.WriteString(syllables[0])
			initials.WriteByte(syllables[0][0])
		} else if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			full.WriteRune(char)
			initials.WriteRune(char)
		}
	}
	return full.String(), initials.String()
}
