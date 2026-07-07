package domain

type UpdateActiveStatusRequest struct {
	IsActive bool `json:"is_active"`
}

type ActiveStatusResponse struct {
	ID        string `json:"id"`
	IsActive  bool   `json:"is_active"`
	UpdatedAt string `json:"updated_at"`
}
