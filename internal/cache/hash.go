package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/google/uuid"
	esdomain "virtual-exam-api/internal/examset/domain"
)

type examSetListHashPayload struct {
	Page          int      `json:"page"`
	Limit         int      `json:"limit"`
	Q             string   `json:"q"`
	TrackCode     string   `json:"track_code"`
	TrackCodes    []string `json:"track_codes"`
	TrackID       string   `json:"track_id"`
	SubjectCodes  []string `json:"subject_codes"`
	QuestionTypes []string `json:"question_types"`
	AccessType    string   `json:"access_type"`
	AccessTypes   []string `json:"access_types"`
	Difficulty    string   `json:"difficulty"`
	Difficulties  []string `json:"difficulties"`
	Mode          string   `json:"mode"`
	Modes         []string `json:"modes"`
	Statuses      []string `json:"statuses"`
	Sort          string   `json:"sort"`
	OnlyActive    bool     `json:"only_active"`
	OnlyPublished bool     `json:"only_published"`
	Visibility    string   `json:"visibility"`
}

func HashExamSetListFilter(filter esdomain.ListFilter) string {
	payload := examSetListHashPayload{
		Page:          filter.Page,
		Limit:         filter.Limit,
		Q:             filter.Query,
		TrackCode:     filter.TrackCode,
		TrackCodes:    sortedCopy(filter.TrackCodes),
		SubjectCodes:  sortedCopy(filter.SubjectCodes),
		QuestionTypes: sortedCopy(filter.QuestionTypes),
		AccessType:    filter.AccessType,
		AccessTypes:   sortedCopy(filter.AccessTypes),
		Difficulty:    filter.Difficulty,
		Difficulties:  sortedCopy(filter.Difficulties),
		Mode:          filter.Mode,
		Modes:         sortedCopy(filter.Modes),
		Statuses:      sortedCopy(filter.Statuses),
		Sort:          filter.Sort,
		OnlyActive:    filter.OnlyActive,
		OnlyPublished: filter.OnlyPublished,
		Visibility:    hashVisibilityScope(filter.Visibility),
	}
	if filter.TrackID != uuid.Nil {
		payload.TrackID = filter.TrackID.String()
	}
	return hashPayload(payload)
}

func HashFilterOptionsScope(scope esdomain.VisibilityScope) string {
	return hashVisibilityScope(scope)
}

func hashVisibilityScope(scope esdomain.VisibilityScope) string {
	if len(scope.EntitledPrivateExamSetIDs) == 0 {
		return "public"
	}
	ids := sortedCopy(scope.EntitledPrivateExamSetIDs)
	data, err := json.Marshal(ids)
	if err != nil {
		return "public"
	}
	sum := sha256.Sum256(data)
	return "user:" + hex.EncodeToString(sum[:6])
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func hashPayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return "00000000"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:4])
}

func FilterOptionsScopeKey(userID *uuid.UUID, scope esdomain.VisibilityScope) string {
	if userID == nil || *userID == uuid.Nil {
		return "public"
	}
	visibility := hashVisibilityScope(scope)
	if visibility == "public" {
		return "user:" + strings.ReplaceAll(userID.String(), "-", "")
	}
	return visibility
}
