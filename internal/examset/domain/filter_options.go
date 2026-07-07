package domain

type FilterOptionCount struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
	Disabled bool   `json:"disabled"`
}

type TrackFilterOption struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Disabled bool   `json:"disabled"`
}

type SubjectFilterOption struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Disabled bool   `json:"disabled"`
}

type FilterOptionsResponse struct {
	Tracks           []TrackFilterOption   `json:"tracks"`
	Subjects         []SubjectFilterOption `json:"subjects"`
	QuestionTypes    []FilterOptionCount   `json:"question_types"`
	DifficultyLevels []FilterOptionCount   `json:"difficulty_levels"`
	AccessTypes      []FilterOptionCount   `json:"access_types"`
	Modes            []FilterOptionCount   `json:"modes"`
	Statuses         []FilterOptionCount   `json:"statuses"`
}

type VisibilityScope struct {
	EntitledPrivateExamSetIDs []string
}
