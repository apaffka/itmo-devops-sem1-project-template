package prices

type ImportResult struct {
	TotalCount      int64 `json:"total_count"`
	DuplicatesCount int64 `json:"duplicates_count"`
	TotalItems      int64 `json:"total_items"`
	TotalCategories int64 `json:"total_categories"`
	TotalPrice      any   `json:"total_price"`
}
