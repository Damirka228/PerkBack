package domain

import "strings"

func (p Profile) Validate() error {
	if strings.TrimSpace(p.UserID) == "" {
		return ErrUserIDEmpty
	}
	if len(p.SelectedCategoryIDs) > MaxSelectedCategories {
		return ErrTooManyCategories
	}
	seen := make(map[string]struct{}, len(p.SelectedCategoryIDs))
	for _, categoryID := range p.SelectedCategoryIDs {
		if strings.TrimSpace(categoryID) == "" {
			return ErrCategoryIDEmpty
		}
		if _, exists := seen[categoryID]; exists {
			return ErrDuplicateCategory
		}
		seen[categoryID] = struct{}{}
	}
	return nil
}
