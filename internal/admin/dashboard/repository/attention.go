package repository

import (
	"context"
	"log"

	"gorm.io/gorm"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	importrepo "virtual-exam-api/internal/questionimport/repository"
)

type AdminAttentionMetrics struct {
	DraftQuestions              int64 `json:"draft_questions"`
	InactiveExamSets            int64 `json:"inactive_exam_sets"`
	ExamSetsMissingQuestions    int64 `json:"exam_sets_missing_questions"`
	LatestImportErrorRows       int64 `json:"latest_import_error_rows"`
}

func (r *postgresRepository) GetAdminAttentionMetrics(ctx context.Context) AdminAttentionMetrics {
	metrics := AdminAttentionMetrics{}

	if err := r.db.WithContext(ctx).Table("questions").
		Where("status = ?", "draft").
		Count(&metrics.DraftQuestions).Error; err != nil {
		log.Printf("admin dashboard: draft questions attention query failed: %v", err)
	}

	if err := r.db.WithContext(ctx).Model(&examsetrepo.ExamSetModel{}).
		Where("is_active = ?", false).
		Count(&metrics.InactiveExamSets).Error; err != nil {
		log.Printf("admin dashboard: inactive exam sets attention query failed: %v", err)
	}

	if err := r.db.WithContext(ctx).Raw(`
SELECT COUNT(*)
FROM exam_sets es
WHERE es.total_questions > (
	SELECT COUNT(*)
	FROM exam_set_questions esq
	WHERE esq.exam_set_id = es.id
)
`).Scan(&metrics.ExamSetsMissingQuestions).Error; err != nil {
		log.Printf("admin dashboard: exam sets missing questions attention query failed: %v", err)
	}

	var latestJob importrepo.ImportJobModel
	err := r.db.WithContext(ctx).
		Model(&importrepo.ImportJobModel{}).
		Order("created_at DESC").
		Limit(1).
		First(&latestJob).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("admin dashboard: latest import job attention query failed: %v", err)
		}
	} else {
		metrics.LatestImportErrorRows = int64(latestJob.InvalidRows + latestJob.FailedRows)
	}

	return metrics
}
