package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	esdomain "virtual-exam-api/internal/examset/domain"
)

type examSetListHashPayload struct {
	Page          int    `json:"page"`
	Limit         int    `json:"limit"`
	Q             string `json:"q"`
	TrackCode     string `json:"track_code"`
	TrackID       string `json:"track_id"`
	AccessType    string `json:"access_type"`
	Difficulty    string `json:"difficulty"`
	Mode          string `json:"mode"`
	OnlyActive    bool   `json:"only_active"`
	OnlyPublished bool   `json:"only_published"`
}

func HashExamSetListFilter(filter esdomain.ListFilter) string {
	payload := examSetListHashPayload{
		Page:          filter.Page,
		Limit:         filter.Limit,
		Q:             filter.Query,
		TrackCode:     filter.TrackCode,
		AccessType:    filter.AccessType,
		Difficulty:    filter.Difficulty,
		Mode:          filter.Mode,
		OnlyActive:    filter.OnlyActive,
		OnlyPublished: filter.OnlyPublished,
	}
	if filter.TrackID != uuid.Nil {
		payload.TrackID = filter.TrackID.String()
	}
	return hashPayload(payload)
}

func hashPayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return "00000000"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:4])
}
