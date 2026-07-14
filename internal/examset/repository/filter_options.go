package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"virtual-exam-api/internal/examset/domain"
)

type trackCountRow struct {
	ID    string
	Code  string
	Name  string
	Count int
}

type subjectCountRow struct {
	ID    string
	Code  string
	Name  string
	Count int
}

type valueCountRow struct {
	Value string
	Count int
}

func (r *postgresRepository) AggregateFilterOptions(ctx context.Context, scope domain.VisibilityScope) (*domain.FilterOptionsResponse, error) {
	tracks, err := r.aggregateTrackOptions(ctx, scope)
	if err != nil {
		return nil, err
	}
	subjects, err := r.aggregateSubjectOptions(ctx, scope)
	if err != nil {
		return nil, err
	}
	questionTypes, err := r.aggregateQuestionTypeOptions(ctx, scope)
	if err != nil {
		return nil, err
	}
	difficulties, err := r.aggregateValueOptions(ctx, scope, "exam_sets.difficulty", domainDifficultyLabels())
	if err != nil {
		return nil, err
	}
	accessTypes, err := r.aggregateValueOptions(ctx, scope, "exam_sets.access_type", domainAccessTypeLabels())
	if err != nil {
		return nil, err
	}
	modes, err := r.aggregateValueOptions(ctx, scope, "exam_sets.mode", domainModeLabels())
	if err != nil {
		return nil, err
	}
	statuses, err := r.aggregateReadyStatusOptions(ctx, scope)
	if err != nil {
		return nil, err
	}

	return &domain.FilterOptionsResponse{
		Tracks:           tracks,
		Subjects:         subjects,
		QuestionTypes:    questionTypes,
		DifficultyLevels: difficulties,
		AccessTypes:      accessTypes,
		Modes:            modes,
		Statuses:         statuses,
	}, nil
}

func (r *postgresRepository) baseVisibleQuery(ctx context.Context, scope domain.VisibilityScope) *gorm.DB {
	return applyPublishedVisibility(r.db.WithContext(ctx).Model(&ExamSetModel{}), scope)
}

func (r *postgresRepository) aggregateTrackOptions(ctx context.Context, scope domain.VisibilityScope) ([]domain.TrackFilterOption, error) {
	var rows []trackCountRow
	q := r.db.WithContext(ctx).Table("exam_tracks").
		Select(`
exam_tracks.id::text AS id,
exam_tracks.code AS code,
exam_tracks.name AS name,
COUNT(DISTINCT exam_sets.id)::int AS count`).
		Joins(r.visibleExamSetLeftJoin(scope, "exam_sets.exam_track_id = exam_tracks.id")).
		Where("exam_tracks.is_active = ?", true).
		Group("exam_tracks.id, exam_tracks.code, exam_tracks.name").
		Order("exam_tracks.name ASC")
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.TrackFilterOption, len(rows))
	for i, row := range rows {
		out[i] = newTrackFilterOption(row.ID, row.Code, row.Name, row.Count)
	}
	return out, nil
}

func (r *postgresRepository) aggregateSubjectOptions(ctx context.Context, scope domain.VisibilityScope) ([]domain.SubjectFilterOption, error) {
	var rows []subjectCountRow
	q := r.db.WithContext(ctx).Table("subjects").
		Select(`
subjects.id::text AS id,
subjects.code AS code,
subjects.name AS name,
COUNT(DISTINCT exam_sets.id)::int AS count`).
		Joins("LEFT JOIN questions q ON q.subject_id = subjects.id AND q.is_active = true").
		Joins("LEFT JOIN exam_set_questions esq ON esq.question_id = q.id").
		Joins(r.visibleExamSetLeftJoin(scope, "exam_sets.id = esq.exam_set_id")).
		Group("subjects.id, subjects.code, subjects.name").
		Order("subjects.name ASC")
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.SubjectFilterOption, len(rows))
	for i, row := range rows {
		out[i] = newSubjectFilterOption(row.ID, row.Code, row.Name, row.Count)
	}
	return out, nil
}

func (r *postgresRepository) aggregateQuestionTypeOptions(ctx context.Context, scope domain.VisibilityScope) ([]domain.FilterOptionCount, error) {
	labels := domainQuestionTypeLabels()
	types := domainQuestionTypeValues()
	out := make([]domain.FilterOptionCount, 0, len(types))
	for _, qt := range types {
		count, err := r.countExamSetsWithQuestionType(ctx, scope, qt)
		if err != nil {
			return nil, err
		}
		out = append(out, newFilterOptionCount(qt, labels[qt], count))
	}
	return out, nil
}

func (r *postgresRepository) countExamSetsWithQuestionType(ctx context.Context, scope domain.VisibilityScope, questionType string) (int, error) {
	q := r.baseVisibleQuery(ctx, scope)
	q = q.Where(buildQuestionTypeExistsClause([]string{questionType}))
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *postgresRepository) aggregateValueOptions(
	ctx context.Context,
	scope domain.VisibilityScope,
	column string,
	labels map[string]string,
) ([]domain.FilterOptionCount, error) {
	counts := map[string]int{}
	var rows []valueCountRow
	err := r.baseVisibleQuery(ctx, scope).
		Select(column + " AS value, COUNT(DISTINCT exam_sets.id)::int AS count").
		Group(column).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.Value] = row.Count
	}

	values := orderedValuesForColumn(column)
	out := make([]domain.FilterOptionCount, 0, len(values))
	for _, value := range values {
		label, ok := labels[value]
		if !ok {
			label = value
		}
		out = append(out, newFilterOptionCount(value, label, counts[value]))
	}
	return out, nil
}

func (r *postgresRepository) aggregateReadyStatusOptions(ctx context.Context, scope domain.VisibilityScope) ([]domain.FilterOptionCount, error) {
	q := r.baseVisibleQuery(ctx, scope)
	q = q.Where(`
EXISTS (
  SELECT 1 FROM exam_set_questions esq
  WHERE esq.exam_set_id = exam_sets.id
)`)
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	return []domain.FilterOptionCount{{
		Value:    "ready",
		Label:    "พร้อมทำข้อสอบ",
		Count:    int(count),
		Disabled: count == 0,
	}}, nil
}

func (r *postgresRepository) visibleExamSetLeftJoin(scope domain.VisibilityScope, joinOn string) string {
	conditions := []string{
		joinOn,
		"exam_sets.status = '" + domain.StatusPublished + "'",
		"exam_sets.is_active = true",
		`EXISTS (
  SELECT 1 FROM exam_tracks active_filter_tracks
  WHERE active_filter_tracks.id = exam_sets.exam_track_id
    AND active_filter_tracks.is_active = true
)`,
	}
	if privateVisibilitySQL := privateVisibilityCondition(scope); privateVisibilitySQL != "" {
		conditions = append(conditions, privateVisibilitySQL)
	}
	return `
LEFT JOIN exam_sets ON ` + strings.Join(conditions, " AND ")
}

func privateVisibilityCondition(scope domain.VisibilityScope) string {
	ids := sanitizedUUIDStrings(scope.EntitledPrivateExamSetIDs)
	if len(ids) == 0 {
		return "exam_sets.access_type <> '" + domain.AccessPrivate + "'"
	}
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = "'" + id + "'"
	}
	return "(exam_sets.access_type <> '" + domain.AccessPrivate + "' OR exam_sets.id IN (" + strings.Join(quoted, ",") + "))"
}

func sanitizedUUIDStrings(rawIDs []string) []string {
	out := make([]string, 0, len(rawIDs))
	for _, raw := range rawIDs {
		value := strings.TrimSpace(raw)
		if len(value) != 36 {
			continue
		}
		valid := true
		for _, ch := range value {
			if (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			valid = false
			break
		}
		if valid {
			out = append(out, value)
		}
	}
	return out
}

func newTrackFilterOption(id, code, name string, count int) domain.TrackFilterOption {
	return domain.TrackFilterOption{
		ID:       id,
		Code:     code,
		Name:     name,
		Count:    count,
		Disabled: count == 0,
	}
}

func newSubjectFilterOption(id, code, name string, count int) domain.SubjectFilterOption {
	return domain.SubjectFilterOption{
		ID:       id,
		Code:     code,
		Name:     name,
		Count:    count,
		Disabled: count == 0,
	}
}

func newFilterOptionCount(value, label string, count int) domain.FilterOptionCount {
	return domain.FilterOptionCount{
		Value:    value,
		Label:    label,
		Count:    count,
		Disabled: count == 0,
	}
}

func orderedValuesForColumn(column string) []string {
	switch column {
	case "exam_sets.difficulty":
		return []string{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard}
	case "exam_sets.access_type":
		return []string{domain.AccessFree, domain.AccessPremium}
	case "exam_sets.mode":
		return []string{domain.ModeMockExam, domain.ModePractice}
	default:
		return []string{}
	}
}

func domainAccessTypeLabels() map[string]string {
	return map[string]string{
		domain.AccessFree:    "ฟรี",
		domain.AccessPremium: "สำหรับสมาชิก",
	}
}

func domainDifficultyLabels() map[string]string {
	return map[string]string{
		domain.DifficultyEasy:   "ง่าย",
		domain.DifficultyMedium: "ปานกลาง",
		domain.DifficultyHard:   "ยาก",
	}
}

func domainModeLabels() map[string]string {
	return map[string]string{
		domain.ModePractice: "ฝึกทำ",
		domain.ModeMockExam: "จำลองสอบ",
	}
}

func domainQuestionTypeLabels() map[string]string {
	return map[string]string{
		"text":  "ข้อความทั่วไป",
		"math":  "สูตรคณิต",
		"image": "รูปภาพ",
	}
}

func domainQuestionTypeValues() []string {
	return []string{"text", "math", "image"}
}
