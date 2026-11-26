package domain

const MaxSelectedCategories = 5

type Profile struct {
	UserID              string
	SelectedCategoryIDs []string
}
