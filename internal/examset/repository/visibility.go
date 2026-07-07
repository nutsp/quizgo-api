package repository

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/examset/domain"
)

func applyPublishedVisibility(q *gorm.DB, scope domain.VisibilityScope) *gorm.DB {
	q = q.Where("exam_sets.status = ? AND exam_sets.is_active = ?", domain.StatusPublished, true)
	q = q.Joins("JOIN exam_tracks ON exam_tracks.id = exam_sets.exam_track_id AND exam_tracks.is_active = ?", true)

	if len(scope.EntitledPrivateExamSetIDs) == 0 {
		return q.Where("exam_sets.access_type <> ?", domain.AccessPrivate)
	}

	ids := make([]uuid.UUID, 0, len(scope.EntitledPrivateExamSetIDs))
	for _, raw := range scope.EntitledPrivateExamSetIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return q.Where("exam_sets.access_type <> ?", domain.AccessPrivate)
	}
	return q.Where(
		"exam_sets.access_type <> ? OR exam_sets.id IN ?",
		domain.AccessPrivate,
		ids,
	)
}

func applyListFilters(q *gorm.DB, filter domain.ListFilter) *gorm.DB {
	if len(filter.TrackCodes) > 0 {
		q = q.Where("exam_tracks.code IN ?", filter.TrackCodes)
	} else if filter.TrackCode != "" {
		q = q.Where("exam_tracks.code = ?", filter.TrackCode)
	}
	if filter.TrackID != uuid.Nil {
		q = q.Where("exam_sets.exam_track_id = ?", filter.TrackID)
	}

	accessTypes := filter.AccessTypes
	if len(accessTypes) == 0 && filter.AccessType != "" {
		accessTypes = []string{filter.AccessType}
	}
	if len(accessTypes) > 0 {
		q = q.Where("exam_sets.access_type IN ?", accessTypes)
	}

	difficulties := filter.Difficulties
	if len(difficulties) == 0 && filter.Difficulty != "" {
		difficulties = []string{filter.Difficulty}
	}
	if len(difficulties) > 0 {
		q = q.Where("exam_sets.difficulty IN ?", difficulties)
	}

	modes := filter.Modes
	if len(modes) == 0 && filter.Mode != "" {
		modes = []string{filter.Mode}
	}
	if len(modes) > 0 {
		q = q.Where("exam_sets.mode IN ?", modes)
	}

	if len(filter.SubjectCodes) > 0 {
		q = q.Where(`
EXISTS (
  SELECT 1
  FROM exam_set_questions esq
  JOIN questions q ON q.id = esq.question_id
  JOIN subjects s ON s.id = q.subject_id
  WHERE esq.exam_set_id = exam_sets.id
    AND q.is_active = true
    AND s.code IN ?
)`, filter.SubjectCodes)
	}

	if len(filter.QuestionTypes) > 0 {
		q = q.Where(buildQuestionTypeExistsClause(filter.QuestionTypes))
	}

	if len(filter.Statuses) > 0 {
		for _, status := range filter.Statuses {
			if strings.EqualFold(status, "ready") {
				q = q.Where(`
EXISTS (
  SELECT 1 FROM exam_set_questions esq
  WHERE esq.exam_set_id = exam_sets.id
)`)
				break
			}
		}
	}

	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		q = q.Where("exam_sets.title ILIKE ? OR exam_sets.description ILIKE ? OR exam_sets.code ILIKE ?", like, like, like)
	}

	return q
}

func buildQuestionTypeExistsClause(types []string) string {
	parts := make([]string, 0, len(types))
	for _, qt := range types {
		switch strings.ToLower(strings.TrimSpace(qt)) {
		case "image":
			parts = append(parts, `(q.question_image_url IS NOT NULL AND q.question_image_url <> '')`)
		case "math":
			parts = append(parts, `q.content_format = 'markdown_math'`)
		case "normal", "text":
			parts = append(parts, `((q.question_image_url IS NULL OR q.question_image_url = '') AND q.content_format <> 'markdown_math')`)
		}
	}
	if len(parts) == 0 {
		return "1 = 0"
	}
	return `
EXISTS (
  SELECT 1
  FROM exam_set_questions esq
  JOIN questions q ON q.id = esq.question_id
  WHERE esq.exam_set_id = exam_sets.id
    AND q.is_active = true
    AND (` + strings.Join(parts, " OR ") + `
  )
)`
}

func listOrderClause(sort string) string {
	switch sort {
	case "latest":
		return "exam_sets.created_at DESC"
	case "popular":
		return "exam_sets.is_featured DESC, exam_sets.created_at DESC"
	case "questions_desc":
		return "exam_sets.total_questions DESC, exam_sets.created_at DESC"
	case "duration_asc":
		return "exam_sets.duration_minutes ASC, exam_sets.created_at DESC"
	case "free_first":
		return "CASE exam_sets.access_type WHEN 'free' THEN 0 WHEN 'trial' THEN 1 ELSE 2 END, exam_sets.created_at DESC"
	default:
		return "exam_sets.is_featured DESC, exam_sets.created_at DESC"
	}
}
