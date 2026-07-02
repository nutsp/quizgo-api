package repository

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	userrepo "virtual-exam-api/internal/user/repository"
)

const defaultTrendDays = 30

type AdminTrendPoint struct {
	Date  string `json:"date"`
	Label string `json:"label"`
	Total int64  `json:"total"`
}

type AdminStatusChartItem struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Total  int64  `json:"total"`
}

type QuestionsBySubjectItem struct {
	SubjectID       string `json:"subject_id"`
	SubjectName     string `json:"subject_name"`
	TotalQuestions  int64  `json:"total_questions"`
}

type AdminCharts struct {
	AttemptsTrend      []AdminTrendPoint        `json:"attempts_trend"`
	NewUsersTrend      []AdminTrendPoint        `json:"new_users_trend"`
	QuestionStatus     []AdminStatusChartItem   `json:"question_status"`
	QuestionsBySubject []QuestionsBySubjectItem `json:"questions_by_subject"`
}

var questionStatusLabels = map[string]string{
	"published": "เผยแพร่แล้ว",
	"draft":     "ฉบับร่าง",
	"archived":  "เก็บถาวร",
}

type trendQueryRow struct {
	Day   time.Time
	Total int64
}

func (r *postgresRepository) GetAttemptTrend(ctx context.Context, days int) ([]AdminTrendPoint, error) {
	if days <= 0 {
		days = defaultTrendDays
	}
	loc := time.Local
	start := trendStartDate(days, loc)

	var rows []trendQueryRow
	if err := r.db.WithContext(ctx).Raw(`
SELECT date_trunc('day', created_at) AS day, COUNT(*) AS total
FROM exam_attempts
WHERE created_at >= ?
GROUP BY day
ORDER BY day
`, start).Scan(&rows).Error; err != nil {
		return nil, err
	}

	points := make([]AdminTrendPoint, 0, len(rows))
	for _, row := range rows {
		day := row.Day.In(loc)
		dateStr := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
		points = append(points, AdminTrendPoint{
			Date:  dateStr,
			Total: row.Total,
		})
	}

	return FillDailyTrend(days, points, loc), nil
}

func (r *postgresRepository) GetNewUserTrend(ctx context.Context, days int) ([]AdminTrendPoint, error) {
	if days <= 0 {
		days = defaultTrendDays
	}
	loc := time.Local
	start := trendStartDate(days, loc)

	var rows []trendQueryRow
	if err := r.db.WithContext(ctx).Model(&userrepo.UserModel{}).
		Select("date_trunc('day', created_at) AS day, COUNT(*) AS total").
		Where("created_at >= ?", start).
		Group("day").
		Order("day").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	points := make([]AdminTrendPoint, 0, len(rows))
	for _, row := range rows {
		day := row.Day.In(loc)
		dateStr := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
		points = append(points, AdminTrendPoint{
			Date:  dateStr,
			Total: row.Total,
		})
	}

	return FillDailyTrend(days, points, loc), nil
}

func (r *postgresRepository) GetQuestionStatusChart(ctx context.Context) ([]AdminStatusChartItem, error) {
	type statusRow struct {
		Status string
		Total  int64
	}

	var rows []statusRow
	if err := r.db.WithContext(ctx).Raw(`
SELECT status, COUNT(*) AS total
FROM questions
GROUP BY status
`).Scan(&rows).Error; err != nil {
		return nil, err
	}

	totals := make(map[string]int64, len(rows))
	for _, row := range rows {
		totals[row.Status] = row.Total
	}

	order := []string{"published", "draft", "archived"}
	result := make([]AdminStatusChartItem, 0, len(order))
	for _, status := range order {
		result = append(result, AdminStatusChartItem{
			Status: status,
			Label:  questionStatusLabels[status],
			Total:  totals[status],
		})
	}
	return result, nil
}

func (r *postgresRepository) GetQuestionsBySubjectChart(ctx context.Context, limit int) ([]QuestionsBySubjectItem, error) {
	if limit <= 0 {
		limit = 8
	}

	type subjectRow struct {
		ID              uuid.UUID
		Name            string
		TotalQuestions  int64
	}

	var rows []subjectRow
	if err := r.db.WithContext(ctx).Raw(`
SELECT subjects.id, subjects.name, COUNT(questions.id) AS total_questions
FROM subjects
LEFT JOIN questions ON questions.subject_id = subjects.id
GROUP BY subjects.id, subjects.name
ORDER BY total_questions DESC, subjects.name ASC
LIMIT ?
`, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]QuestionsBySubjectItem, len(rows))
	for i, row := range rows {
		result[i] = QuestionsBySubjectItem{
			SubjectID:      row.ID.String(),
			SubjectName:    row.Name,
			TotalQuestions: row.TotalQuestions,
		}
	}
	return result, nil
}

func emptyCharts() AdminCharts {
	return AdminCharts{
		AttemptsTrend:      []AdminTrendPoint{},
		NewUsersTrend:      []AdminTrendPoint{},
		QuestionStatus:     []AdminStatusChartItem{},
		QuestionsBySubject: []QuestionsBySubjectItem{},
	}
}

func (r *postgresRepository) GetAdminCharts(ctx context.Context) AdminCharts {
	charts := emptyCharts()

	attemptsTrend, err := r.GetAttemptTrend(ctx, defaultTrendDays)
	if err != nil {
		log.Printf("admin dashboard: attempts trend query failed: %v", err)
	} else {
		charts.AttemptsTrend = attemptsTrend
	}

	newUsersTrend, err := r.GetNewUserTrend(ctx, defaultTrendDays)
	if err != nil {
		log.Printf("admin dashboard: new users trend query failed: %v", err)
	} else {
		charts.NewUsersTrend = newUsersTrend
	}

	questionStatus, err := r.GetQuestionStatusChart(ctx)
	if err != nil {
		log.Printf("admin dashboard: question status chart query failed: %v", err)
	} else {
		charts.QuestionStatus = questionStatus
	}

	questionsBySubject, err := r.GetQuestionsBySubjectChart(ctx, 8)
	if err != nil {
		log.Printf("admin dashboard: questions by subject chart query failed: %v", err)
	} else {
		charts.QuestionsBySubject = questionsBySubject
	}

	return charts
}
