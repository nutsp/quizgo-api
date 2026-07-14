package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/common/pagination"
	esdomain "virtual-exam-api/internal/examset/domain"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	esqdomain "virtual-exam-api/internal/examsetquestion/domain"
	esqrepo "virtual-exam-api/internal/examsetquestion/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	qdomain "virtual-exam-api/internal/question/domain"
	questionrepo "virtual-exam-api/internal/question/repository"
	tagdomain "virtual-exam-api/internal/questiontag/domain"
)

type questionTagLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*tagdomain.QuestionTag, error)
}

type UseCase struct {
	repo        esqrepo.Repository
	questions   questionrepo.QuestionAdminRepository
	sets        examsetrepo.Repository
	setAdmin    examsetrepo.AdminRepository
	trackAdmin  trackrepo.AdminRepository
	tags        questionTagLookup
	invalidator *cache.Invalidator
}

func NewUseCase(
	repo esqrepo.Repository,
	questions questionrepo.QuestionAdminRepository,
	sets examsetrepo.Repository,
	setAdmin examsetrepo.AdminRepository,
	trackAdmin trackrepo.AdminRepository,
	tags questionTagLookup,
	invalidator *cache.Invalidator,
) *UseCase {
	return &UseCase{
		repo:        repo,
		questions:   questions,
		sets:        sets,
		setAdmin:    setAdmin,
		trackAdmin:  trackAdmin,
		tags:        tags,
		invalidator: invalidator,
	}
}

type AvailableFilterInput struct {
	Query           string
	SubjectID       string
	TagID           string
	Difficulty      string
	Status          string
	ExcludeAssigned bool
	Page            int
	Limit           int
	Sort            string
	Order           string
}

type AssignedFilterInput struct {
	Query     string
	SubjectID string
	TagID     string
	Page      int
	Limit     int
	Sort      string
	Order     string
}

type TagSummaryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Code  string `json:"code"`
	Color string `json:"color,omitempty"`
}

type AvailableQuestionResponse struct {
	ID               string          `json:"id"`
	QuestionText     string          `json:"question_text"`
	Subject          *SubjectDTO     `json:"subject,omitempty"`
	Difficulty       string          `json:"difficulty"`
	Status           string          `json:"status"`
	CorrectChoiceKey string          `json:"correct_choice_key,omitempty"`
	CreatedAt        string          `json:"created_at"`
	AlreadyAssigned  bool            `json:"already_assigned"`
	Tags             []TagSummaryDTO `json:"tags,omitempty"`
}

type SubjectDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PaginationDTO = pagination.PaginationMeta

type AvailableQuestionsResponse = pagination.PaginatedList[AvailableQuestionResponse]

type ExamSetDTO struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	Title           string `json:"title"`
	TotalQuestions  int    `json:"total_questions"`
	DurationMinutes int    `json:"duration_minutes"`
	PassingScore    int    `json:"passing_score"`
}

type AssignedQuestionResponse struct {
	QuestionID   string          `json:"question_id"`
	QuestionNo   int             `json:"question_no"`
	Score        float64         `json:"score"`
	QuestionText string          `json:"question_text"`
	Subject      *SubjectDTO     `json:"subject,omitempty"`
	Difficulty   string          `json:"difficulty"`
	Status       string          `json:"status"`
	Tags         []TagSummaryDTO `json:"tags,omitempty"`
}

type ListAssignedResponse struct {
	ExamSet            ExamSetDTO                 `json:"exam_set"`
	Items              []AssignedQuestionResponse `json:"items"`
	Pagination         pagination.PaginationMeta  `json:"pagination"`
	IsLockedByAttempts bool                       `json:"is_locked_by_attempts"`
}

type BulkAddInput struct {
	QuestionIDs []string `json:"question_ids"`
	Score       float64  `json:"score"`
	AppendToEnd bool     `json:"append_to_end"`
}

type AutoAssignRule struct {
	SubjectID string `json:"subject_id"`
	TagID     string `json:"tag_id,omitempty"`
	Count     int    `json:"count"`
}

type AutoAssignInput struct {
	Rules []AutoAssignRule `json:"rules"`
}

type QuestionRuleResponse struct {
	SubjectID string `json:"subject_id"`
	TagID     string `json:"tag_id,omitempty"`
	Count     int    `json:"count"`
}

type SetQuestionRulesInput struct {
	Rules []AutoAssignRule `json:"rules"`
}

type QuestionRuleCapacityResponse struct {
	TotalAvailable     int64 `json:"total_available"`
	AssignedCount      int64 `json:"assigned_count"`
	RemainingAvailable int64 `json:"remaining_available"`
}

type BulkAddResponse struct {
	ExamSetID      string `json:"exam_set_id"`
	AddedCount     int    `json:"added_count"`
	SkippedCount   int    `json:"skipped_count"`
	TotalQuestions int    `json:"total_questions"`
	AddedQuestions []struct {
		QuestionID string `json:"question_id"`
		QuestionNo int    `json:"question_no"`
	} `json:"added_questions"`
	SkippedQuestions []struct {
		QuestionID string `json:"question_id"`
		Reason     string `json:"reason"`
	} `json:"skipped_questions"`
}

type ReorderInput struct {
	Items []struct {
		QuestionID string `json:"question_id"`
		QuestionNo int    `json:"question_no"`
	} `json:"items"`
}

type RemoveResponse struct {
	Removed        bool `json:"removed"`
	TotalQuestions int  `json:"total_questions"`
}

type ClearAllInput struct {
	Confirm bool `json:"confirm"`
}

type ClearAllResponse struct {
	Cleared        bool `json:"cleared"`
	TotalQuestions int  `json:"total_questions"`
}

type AutoAssignResponse = BulkAddResponse

func (uc *UseCase) GetQuestionRules(ctx context.Context, examSetID uuid.UUID) ([]QuestionRuleResponse, error) {
	if _, err := uc.requireExamSet(ctx, examSetID); err != nil {
		return nil, err
	}
	rules, err := uc.repo.GetQuestionRules(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	return toQuestionRuleResponses(rules), nil
}

func (uc *UseCase) SetQuestionRules(ctx context.Context, examSetID uuid.UUID, input SetQuestionRulesInput) ([]QuestionRuleResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	rules, err := parseQuestionRules(input.Rules, set.TotalQuestions)
	if err != nil {
		return nil, err
	}
	if err := uc.validateRuleTags(ctx, rules); err != nil {
		return nil, err
	}
	rules = applyExamSetDifficulty(rules, set.Difficulty)
	if _, err := uc.questionRuleCapacities(ctx, examSetID, rules, true); err != nil {
		return nil, err
	}
	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	assignedCounts, err := uc.countAssignedByRules(ctx, assigned, rules)
	if err != nil {
		return nil, err
	}
	for index, count := range assignedCounts {
		if count > rules[index].Count {
			return nil, apperrors.ErrExamSetQuestionLimitExceeded
		}
	}
	if err := uc.repo.ReplaceQuestionRules(ctx, examSetID, rules); err != nil {
		return nil, err
	}
	if err := uc.regroupQuestionsByRules(ctx, examSetID, rules); err != nil {
		return nil, err
	}
	return toQuestionRuleResponses(rules), nil
}

func (uc *UseCase) GetQuestionRuleCapacities(ctx context.Context, examSetID uuid.UUID, input SetQuestionRulesInput) ([]QuestionRuleCapacityResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	rules := make([]esqdomain.QuestionRule, 0, len(input.Rules))
	for _, raw := range input.Rules {
		rule, err := parseQuestionRule(raw)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := uc.validateRuleTags(ctx, rules); err != nil {
		return nil, err
	}
	rules = applyExamSetDifficulty(rules, set.Difficulty)
	return uc.questionRuleCapacities(ctx, examSetID, rules, false)
}

func (uc *UseCase) validateRuleTags(ctx context.Context, rules []esqdomain.QuestionRule) error {
	for _, rule := range rules {
		if rule.TagID == nil {
			continue
		}
		if uc.tags == nil {
			return apperrors.ErrTagNotFound
		}
		tag, err := uc.tags.FindByID(ctx, *rule.TagID)
		if err != nil {
			return err
		}
		if err := validateRuleTagSubject(rule.SubjectID, tag); err != nil {
			return err
		}
	}
	return nil
}

func validateRuleTagSubject(subjectID uuid.UUID, tag *tagdomain.QuestionTag) error {
	if tag == nil || !tag.IsActive {
		return apperrors.ErrTagNotFound
	}
	if tag.SubjectID == nil || *tag.SubjectID != subjectID {
		return apperrors.ValidationError("กลุ่มคำถามไม่ตรงกับวิชาที่เลือก")
	}
	return nil
}

func (uc *UseCase) questionRuleCapacities(ctx context.Context, examSetID uuid.UUID, rules []esqdomain.QuestionRule, enforceMaximum bool) ([]QuestionRuleCapacityResponse, error) {
	result := make([]QuestionRuleCapacityResponse, len(rules))
	for index, rule := range rules {
		total, assigned, err := uc.repo.QuestionRuleCapacity(ctx, examSetID, esqrepo.AutoAssignSelection{SubjectID: rule.SubjectID, TagID: uuidValue(rule.TagID), Difficulty: rule.Difficulty})
		if err != nil {
			return nil, err
		}
		if enforceMaximum {
			if err := validateRuleCapacity(index, rule.Count, total); err != nil {
				return nil, err
			}
		}
		result[index] = QuestionRuleCapacityResponse{TotalAvailable: total, AssignedCount: assigned, RemainingAvailable: total - assigned}
	}
	return result, nil
}

func validateRuleCapacity(index, requested int, total int64) error {
	if int64(requested) <= total {
		return nil
	}
	return apperrors.ValidationError(fmt.Sprintf("ส่วนที่ %d ของโครงสร้างมีคำถามพร้อมใช้สูงสุด %d ข้อ", index+1, total))
}

func (uc *UseCase) ListAvailable(ctx context.Context, examSetID uuid.UUID, input AvailableFilterInput) (*AvailableQuestionsResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	filter := esqdomain.AvailableFilter{
		Query:           input.Query,
		Difficulty:      set.Difficulty,
		Status:          input.Status,
		ExcludeAssigned: input.ExcludeAssigned,
		Page:            input.Page,
		Limit:           input.Limit,
		Sort:            input.Sort,
		Order:           input.Order,
	}
	if input.SubjectID != "" {
		sid, err := uuid.Parse(input.SubjectID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.SubjectID = sid
	}
	if input.TagID != "" {
		tid, err := uuid.Parse(input.TagID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.TagID = tid
	}

	items, total, err := uc.repo.ListAvailable(ctx, examSetID, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]AvailableQuestionResponse, len(items))
	for i, item := range items {
		resp[i] = toAvailableResponse(item)
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	result := pagination.NewList(resp, page, limit, total)
	return &result, nil
}

func (uc *UseCase) ListAssigned(ctx context.Context, examSetID uuid.UUID, input AssignedFilterInput) (*ListAssignedResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}

	filter := esqdomain.AssignedFilter{
		Query: input.Query,
		Page:  input.Page,
		Limit: input.Limit,
		Sort:  input.Sort,
		Order: input.Order,
	}
	if input.SubjectID != "" {
		sid, err := uuid.Parse(input.SubjectID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.SubjectID = sid
	}
	if input.TagID != "" {
		tid, err := uuid.Parse(input.TagID)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		filter.TagID = tid
	}

	items, total, err := uc.repo.ListAssigned(ctx, examSetID, filter)
	if err != nil {
		return nil, err
	}
	locked, err := uc.repo.HasSubmittedAttempts(ctx, examSetID)
	if err != nil {
		return nil, err
	}

	resp := make([]AssignedQuestionResponse, len(items))
	for i, item := range items {
		resp[i] = toAssignedResponse(item)
		question, questionErr := uc.questions.FindByID(ctx, item.QuestionID)
		if questionErr != nil {
			return nil, questionErr
		}
		if question != nil {
			for _, tag := range question.Tags {
				resp[i].Tags = append(resp[i].Tags, TagSummaryDTO{ID: tag.ID.String(), Name: tag.Name, Code: tag.Code, Color: tag.Color})
			}
		}
	}
	page, limit := pagination.Sanitize(filter.Page, filter.Limit)
	return &ListAssignedResponse{
		ExamSet: ExamSetDTO{
			ID:              set.ID.String(),
			Code:            set.Code,
			Title:           set.Title,
			TotalQuestions:  set.TotalQuestions,
			DurationMinutes: set.DurationMinutes,
			PassingScore:    set.PassingScore,
		},
		Items:              resp,
		Pagination:         pagination.NewPaginationMeta(page, limit, total),
		IsLockedByAttempts: locked,
	}, nil
}

func (uc *UseCase) BulkAdd(ctx context.Context, examSetID uuid.UUID, input BulkAddInput) (*BulkAddResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	if len(input.QuestionIDs) == 0 {
		return nil, apperrors.ErrInvalidInput
	}
	rules, err := uc.repo.GetQuestionRules(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, apperrors.ErrExamSetQuestionRulesRequired
	}
	rules = applyExamSetDifficulty(rules, set.Difficulty)
	if err := uc.validateQuestionIDsAgainstRules(ctx, examSetID, input.QuestionIDs, rules); err != nil {
		return nil, err
	}
	assignedIDs, err := uc.repo.AssignedQuestionIDs(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	currentCount, err := uc.repo.CountByExamSetID(ctx, examSetID)
	if err != nil {
		return nil, err
	}

	questionIDs := make([]uuid.UUID, 0, len(input.QuestionIDs))
	seenIDs := make(map[uuid.UUID]bool, len(input.QuestionIDs))
	for _, idStr := range input.QuestionIDs {
		qID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, apperrors.ErrInvalidUUID
		}
		q, err := uc.questions.FindByID(ctx, qID)
		if err != nil {
			return nil, err
		}
		if q == nil {
			return nil, apperrors.ErrQuestionNotFound
		}
		if q.Status != qdomain.StatusPublished || !q.IsActive {
			return nil, apperrors.ErrQuestionNotPublished
		}
		if seenIDs[qID] {
			continue
		}
		seenIDs[qID] = true
		questionIDs = append(questionIDs, qID)
	}
	newCount := 0
	for _, qID := range questionIDs {
		if !assignedIDs[qID] {
			newCount++
		}
	}
	if int(currentCount)+newCount > set.TotalQuestions {
		return nil, apperrors.ErrExamSetQuestionLimitExceeded
	}

	score := input.Score
	if score <= 0 {
		score = 1
	}

	result, err := uc.repo.BulkAdd(ctx, examSetID, questionIDs, score)
	if err != nil {
		return nil, err
	}
	if err := uc.regroupQuestionsByRules(ctx, examSetID, rules); err != nil {
		return nil, err
	}
	if err := uc.refreshExamSetAfterQuestionChange(ctx, set); err != nil {
		return nil, err
	}

	return toBulkAddResponse(result), nil
}

func (uc *UseCase) AutoAssign(ctx context.Context, examSetID uuid.UUID, input AutoAssignInput) (*AutoAssignResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	rules, err := uc.repo.GetQuestionRules(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, apperrors.ErrExamSetQuestionRulesRequired
	}
	rules = applyExamSetDifficulty(rules, set.Difficulty)

	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if len(assigned) >= set.TotalQuestions {
		return &AutoAssignResponse{ExamSetID: examSetID.String(), TotalQuestions: len(assigned)}, nil
	}

	assignedCounts, err := uc.countAssignedByRules(ctx, assigned, rules)
	if err != nil {
		return nil, err
	}
	neededBySubject := make([]struct {
		selection esqrepo.AutoAssignSelection
		needed    int
	}, 0, len(rules))
	for index, rule := range rules {
		needed := rule.Count - assignedCounts[index]
		if needed > 0 {
			neededBySubject = append(neededBySubject, struct {
				selection esqrepo.AutoAssignSelection
				needed    int
			}{selection: esqrepo.AutoAssignSelection{SubjectID: rule.SubjectID, TagID: uuidValue(rule.TagID), Difficulty: rule.Difficulty}, needed: needed})
		}
	}

	availableSlots := set.TotalQuestions - len(assigned)
	ids := make([]uuid.UUID, 0, availableSlots)
	for _, request := range neededBySubject {
		if request.needed > availableSlots-len(ids) {
			return nil, apperrors.ErrExamSetQuestionLimitExceeded
		}
		selected, selectErr := uc.repo.RandomAvailableQuestionIDs(ctx, examSetID, request.selection, request.needed)
		if selectErr != nil {
			return nil, selectErr
		}
		if len(selected) != request.needed {
			return nil, apperrors.ErrAutoAssignQuestionsUnavailable
		}
		ids = append(ids, selected...)
	}

	if len(ids) == 0 {
		return &AutoAssignResponse{ExamSetID: examSetID.String(), TotalQuestions: len(assigned)}, nil
	}
	result, err := uc.BulkAdd(ctx, examSetID, BulkAddInput{QuestionIDs: uuidStrings(ids), Score: 1, AppendToEnd: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func uuidStrings(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}

func parseQuestionRules(inputs []AutoAssignRule, totalQuestions int) ([]esqdomain.QuestionRule, error) {
	if len(inputs) == 0 {
		return nil, apperrors.ErrInvalidInput
	}
	rules := make([]esqdomain.QuestionRule, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	total := 0
	for _, input := range inputs {
		rule, err := parseQuestionRule(input)
		if err != nil {
			return nil, err
		}
		subjectID, tagID := rule.SubjectID, rule.TagID
		key := subjectID.String() + ":" + input.TagID
		if seen[key] {
			return nil, apperrors.ErrInvalidInput
		}
		for _, existing := range rules {
			if existing.SubjectID == subjectID &&
				(existing.TagID == nil || tagID == nil || *existing.TagID == *tagID) {
				return nil, apperrors.ErrInvalidInput
			}
		}
		seen[key] = true
		total += input.Count
		rules = append(rules, rule)
	}
	if total != totalQuestions {
		return nil, apperrors.ErrInvalidInput
	}
	return rules, nil
}

func parseQuestionRule(input AutoAssignRule) (esqdomain.QuestionRule, error) {
	subjectID, err := uuid.Parse(input.SubjectID)
	if err != nil || input.Count <= 0 {
		return esqdomain.QuestionRule{}, apperrors.ErrInvalidInput
	}
	var tagID *uuid.UUID
	if input.TagID != "" {
		parsed, err := uuid.Parse(input.TagID)
		if err != nil {
			return esqdomain.QuestionRule{}, apperrors.ErrInvalidUUID
		}
		tagID = &parsed
	}
	return esqdomain.QuestionRule{SubjectID: subjectID, TagID: tagID, Count: input.Count}, nil
}

func toQuestionRuleResponses(rules []esqdomain.QuestionRule) []QuestionRuleResponse {
	result := make([]QuestionRuleResponse, len(rules))
	for i, rule := range rules {
		result[i] = QuestionRuleResponse{SubjectID: rule.SubjectID.String(), TagID: uuidPtrString(rule.TagID), Count: rule.Count}
	}
	return result
}

func applyExamSetDifficulty(rules []esqdomain.QuestionRule, difficulty string) []esqdomain.QuestionRule {
	result := append([]esqdomain.QuestionRule(nil), rules...)
	for index := range result {
		result[index].Difficulty = difficulty
	}
	return result
}

func uuidValue(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func (uc *UseCase) countAssignedByRules(ctx context.Context, assigned []esqdomain.AssignedQuestion, rules []esqdomain.QuestionRule) ([]int, error) {
	counts := make([]int, len(rules))
	for _, item := range assigned {
		q, err := uc.questions.FindByID(ctx, item.QuestionID)
		if err != nil {
			return nil, err
		}
		if q == nil {
			continue
		}
		index := matchingRuleIndex(q, rules)
		if index < 0 {
			return nil, apperrors.ErrQuestionOutsideExamSetRules
		}
		counts[index]++
	}
	return counts, nil
}

func (uc *UseCase) validateQuestionIDsAgainstRules(ctx context.Context, examSetID uuid.UUID, rawIDs []string, rules []esqdomain.QuestionRule) error {
	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return err
	}
	counts, err := uc.countAssignedByRules(ctx, assigned, rules)
	if err != nil {
		return err
	}
	seen := make(map[uuid.UUID]bool, len(rawIDs))
	for _, rawID := range rawIDs {
		id, err := uuid.Parse(rawID)
		if err != nil {
			return apperrors.ErrInvalidUUID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		q, err := uc.questions.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if q == nil {
			return apperrors.ErrQuestionNotFound
		}
		index := matchingRuleIndex(q, rules)
		if index < 0 {
			return apperrors.ErrQuestionOutsideExamSetRules
		}
		if !containsAssigned(assigned, id) {
			counts[index]++
			if counts[index] > rules[index].Count {
				return apperrors.ErrExamSetQuestionLimitExceeded
			}
		}
	}
	return nil
}

func matchingRuleIndex(q *qdomain.Question, rules []esqdomain.QuestionRule) int {
	for index, rule := range rules {
		if q.SubjectID != rule.SubjectID || (rule.Difficulty != "" && q.Difficulty != rule.Difficulty) {
			continue
		}
		if rule.TagID != nil {
			found := false
			for _, tag := range q.Tags {
				if tag.ID == *rule.TagID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		return index
	}
	return -1
}

func containsAssigned(items []esqdomain.AssignedQuestion, id uuid.UUID) bool {
	for _, item := range items {
		if item.QuestionID == id {
			return true
		}
	}
	return false
}

func uuidPtrString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

type ruleGroupedQuestion struct {
	questionID uuid.UUID
	questionNo int
	ruleIndex  int
}

func stableRuleReorderItems(grouped []ruleGroupedQuestion) []esqdomain.ReorderItem {
	ordered := append([]ruleGroupedQuestion(nil), grouped...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].ruleIndex != ordered[j].ruleIndex {
			return ordered[i].ruleIndex < ordered[j].ruleIndex
		}
		return ordered[i].questionNo < ordered[j].questionNo
	})
	items := make([]esqdomain.ReorderItem, len(ordered))
	for index, item := range ordered {
		items[index] = esqdomain.ReorderItem{QuestionID: item.questionID, QuestionNo: index + 1}
	}
	return items
}

func (uc *UseCase) regroupQuestionsByRules(ctx context.Context, examSetID uuid.UUID, rules []esqdomain.QuestionRule) error {
	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return err
	}
	grouped := make([]ruleGroupedQuestion, 0, len(assigned))
	for _, item := range assigned {
		q, err := uc.questions.FindByID(ctx, item.QuestionID)
		if err != nil {
			return err
		}
		if q == nil {
			return apperrors.ErrQuestionNotFound
		}
		ruleIndex := matchingRuleIndex(q, rules)
		if ruleIndex < 0 {
			return apperrors.ErrQuestionOutsideExamSetRules
		}
		grouped = append(grouped, ruleGroupedQuestion{questionID: item.QuestionID, questionNo: item.QuestionNo, ruleIndex: ruleIndex})
	}
	items := stableRuleReorderItems(grouped)
	if len(items) == 0 {
		return nil
	}
	return uc.repo.Reorder(ctx, examSetID, items)
}

func (uc *UseCase) Reorder(ctx context.Context, examSetID uuid.UUID, input ReorderInput) error {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return err
	}
	if len(input.Items) == 0 {
		return apperrors.ErrInvalidInput
	}
	rules, err := uc.repo.GetQuestionRules(ctx, examSetID)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return apperrors.ErrExamSetQuestionRulesRequired
	}
	rules = applyExamSetDifficulty(rules, set.Difficulty)

	assigned, err := uc.repo.ListAllAssigned(ctx, examSetID)
	if err != nil {
		return err
	}
	assignedMap := make(map[uuid.UUID]bool, len(assigned))
	for _, a := range assigned {
		assignedMap[a.QuestionID] = true
	}

	seenNos := make(map[int]bool)
	items := make([]esqdomain.ReorderItem, len(input.Items))
	for i, item := range input.Items {
		qID, err := uuid.Parse(item.QuestionID)
		if err != nil {
			return apperrors.ErrInvalidUUID
		}
		if !assignedMap[qID] {
			return apperrors.ErrQuestionNotFound
		}
		if item.QuestionNo <= 0 {
			return apperrors.ErrInvalidInput
		}
		if seenNos[item.QuestionNo] {
			return apperrors.ErrInvalidInput
		}
		seenNos[item.QuestionNo] = true
		items[i] = esqdomain.ReorderItem{QuestionID: qID, QuestionNo: item.QuestionNo}
	}
	if len(seenNos) != len(assigned) {
		return apperrors.ErrInvalidInput
	}
	orderedItems := append([]esqdomain.ReorderItem(nil), items...)
	sort.Slice(orderedItems, func(i, j int) bool { return orderedItems[i].QuestionNo < orderedItems[j].QuestionNo })
	previousRuleIndex := -1
	for _, item := range orderedItems {
		question, err := uc.questions.FindByID(ctx, item.QuestionID)
		if err != nil {
			return err
		}
		if question == nil {
			return apperrors.ErrQuestionNotFound
		}
		ruleIndex := matchingRuleIndex(question, rules)
		if ruleIndex < previousRuleIndex {
			return apperrors.ErrQuestionOutsideExamSetRules
		}
		previousRuleIndex = ruleIndex
	}

	if err := uc.repo.Reorder(ctx, examSetID, items); err != nil {
		return err
	}
	uc.invalidateExamSetCache(ctx, set)
	return nil
}

func (uc *UseCase) Remove(ctx context.Context, examSetID, questionID uuid.UUID) (*RemoveResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	if err := uc.repo.Remove(ctx, examSetID, questionID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrQuestionNotFound
		}
		return nil, err
	}
	if err := uc.refreshExamSetAfterQuestionChange(ctx, set); err != nil {
		return nil, err
	}
	count, err := uc.repo.CountByExamSetID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	return &RemoveResponse{Removed: true, TotalQuestions: int(count)}, nil
}

func (uc *UseCase) ClearAll(ctx context.Context, examSetID uuid.UUID, input ClearAllInput) (*ClearAllResponse, error) {
	set, err := uc.requireExamSet(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if !input.Confirm {
		return nil, apperrors.ErrInvalidInput
	}
	if err := uc.ensureNotLocked(ctx, examSetID); err != nil {
		return nil, err
	}
	hasAttempts, err := uc.repo.HasAnyAttempts(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if hasAttempts {
		return nil, apperrors.ErrExamSetHasAttempts
	}
	if err := uc.repo.ClearAll(ctx, examSetID); err != nil {
		return nil, err
	}
	if err := uc.refreshExamSetAfterQuestionChange(ctx, set); err != nil {
		return nil, err
	}
	return &ClearAllResponse{Cleared: true, TotalQuestions: 0}, nil
}

func (uc *UseCase) ensureNotLocked(ctx context.Context, examSetID uuid.UUID) error {
	locked, err := uc.repo.HasSubmittedAttempts(ctx, examSetID)
	if err != nil {
		return err
	}
	if locked {
		return apperrors.ErrExamSetLockedByAttempts
	}
	return nil
}

func (uc *UseCase) requireExamSet(ctx context.Context, examSetID uuid.UUID) (*esdomain.ExamSet, error) {
	set, err := uc.sets.FindByID(ctx, examSetID)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	return set, nil
}

func (uc *UseCase) refreshExamSetAfterQuestionChange(ctx context.Context, set *esdomain.ExamSet) error {
	if err := uc.trackAdmin.RefreshCounters(ctx, set.ExamTrackID); err != nil {
		return err
	}
	uc.invalidateExamSetCache(ctx, set)
	return nil
}

func (uc *UseCase) invalidateExamSetCache(ctx context.Context, set *esdomain.ExamSet) {
	if uc.invalidator == nil || set == nil {
		return
	}
	uc.invalidator.OnExamSetChanged(ctx, set.ID.String(), set.Code)
}

func toAvailableResponse(item esqdomain.AvailableQuestion) AvailableQuestionResponse {
	resp := AvailableQuestionResponse{
		ID:               item.ID.String(),
		QuestionText:     item.QuestionText,
		Difficulty:       item.Difficulty,
		Status:           item.Status,
		CorrectChoiceKey: item.CorrectChoiceKey,
		CreatedAt:        item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		AlreadyAssigned:  item.AlreadyAssigned,
	}
	if item.Subject != nil {
		resp.Subject = &SubjectDTO{ID: item.Subject.ID, Name: item.Subject.Name}
	}
	for _, t := range item.Tags {
		resp.Tags = append(resp.Tags, TagSummaryDTO{
			ID:    t.ID,
			Name:  t.Name,
			Code:  t.Code,
			Color: t.Color,
		})
	}
	return resp
}

func toAssignedResponse(item esqdomain.AssignedQuestion) AssignedQuestionResponse {
	resp := AssignedQuestionResponse{
		QuestionID:   item.QuestionID.String(),
		QuestionNo:   item.QuestionNo,
		Score:        item.Score,
		QuestionText: item.QuestionText,
		Difficulty:   item.Difficulty,
		Status:       item.Status,
	}
	if item.Subject != nil {
		resp.Subject = &SubjectDTO{ID: item.Subject.ID, Name: item.Subject.Name}
	}
	return resp
}

func toBulkAddResponse(result esqdomain.BulkAddResult) *BulkAddResponse {
	resp := &BulkAddResponse{
		ExamSetID:      result.ExamSetID.String(),
		AddedCount:     result.AddedCount,
		SkippedCount:   result.SkippedCount,
		TotalQuestions: result.TotalQuestions,
		AddedQuestions: []struct {
			QuestionID string `json:"question_id"`
			QuestionNo int    `json:"question_no"`
		}{},
		SkippedQuestions: []struct {
			QuestionID string `json:"question_id"`
			Reason     string `json:"reason"`
		}{},
	}
	for _, a := range result.AddedQuestions {
		resp.AddedQuestions = append(resp.AddedQuestions, struct {
			QuestionID string `json:"question_id"`
			QuestionNo int    `json:"question_no"`
		}{QuestionID: a.QuestionID.String(), QuestionNo: a.QuestionNo})
	}
	for _, s := range result.SkippedQuestions {
		resp.SkippedQuestions = append(resp.SkippedQuestions, struct {
			QuestionID string `json:"question_id"`
			Reason     string `json:"reason"`
		}{QuestionID: s.QuestionID.String(), Reason: s.Reason})
	}
	return resp
}
