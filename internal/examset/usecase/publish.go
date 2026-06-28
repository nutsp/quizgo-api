package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"virtual-exam-api/internal/apperrors"
	"virtual-exam-api/internal/examset/domain"
	qdomain "virtual-exam-api/internal/question/domain"
)

const previewSampleLimit = 5

type ReadinessCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

type ReadinessSummary struct {
	TotalQuestions     int `json:"total_questions"`
	PublishedQuestions int `json:"published_questions"`
	DraftQuestions     int `json:"draft_questions"`
	InvalidQuestions   int `json:"invalid_questions"`
}

type ReadinessResult struct {
	ExamSetID string           `json:"exam_set_id"`
	Ready     bool             `json:"ready"`
	Status    string           `json:"status"`
	Checks    []ReadinessCheck `json:"checks"`
	Summary   ReadinessSummary `json:"summary"`
}

type PreviewChoice struct {
	ChoiceKey   string `json:"choice_key"`
	ChoiceLabel string `json:"choice_label"`
	ChoiceText  string `json:"choice_text"`
}

type PreviewQuestion struct {
	QuestionNo   int             `json:"question_no"`
	QuestionText string          `json:"question_text"`
	SubjectName  string          `json:"subject_name,omitempty"`
	Difficulty   string          `json:"difficulty,omitempty"`
	Choices      []PreviewChoice `json:"choices"`
}

type PreviewExamSet struct {
	domain.ExamSetSummary
	ExamTrack *domain.ExamTrackRef `json:"exam_track,omitempty"`
}

type PreviewResponse struct {
	ExamSet         PreviewExamSet    `json:"exam_set"`
	Readiness       ReadinessResult   `json:"readiness"`
	SampleQuestions []PreviewQuestion `json:"sample_questions"`
}

type PublishStatusResponse struct {
	ID       string `json:"id"`
	Code     string `json:"code,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   string `json:"status"`
	IsActive bool   `json:"is_active,omitempty"`
}

func (uc *AdminUseCase) CheckReadiness(ctx context.Context, id uuid.UUID) (*ReadinessResult, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	assigned, err := uc.setQuestions.ListByExamSetID(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildReadiness(set, assigned), nil
}

func (uc *AdminUseCase) GetPreview(ctx context.Context, id uuid.UUID) (*PreviewResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	assigned, err := uc.setQuestions.ListByExamSetID(ctx, id)
	if err != nil {
		return nil, err
	}
	readiness := buildReadiness(set, assigned)

	summary := set.ToSummary()
	previewSet := PreviewExamSet{
		ExamSetSummary: summary,
		ExamTrack:      set.ExamTrack,
	}

	samples := make([]PreviewQuestion, 0, previewSampleLimit)
	for i, sq := range assigned {
		if i >= previewSampleLimit {
			break
		}
		pq := PreviewQuestion{QuestionNo: sq.QuestionNo}
		if sq.Question != nil {
			pq.QuestionText = sq.Question.QuestionText
			pq.Difficulty = sq.Question.Difficulty
			if sq.Question.Subject != nil {
				pq.SubjectName = sq.Question.Subject.Name
			}
			for _, ch := range sq.Question.Choices {
				pq.Choices = append(pq.Choices, PreviewChoice{
					ChoiceKey:   ch.ChoiceKey,
					ChoiceLabel: ch.ChoiceLabel,
					ChoiceText:  ch.ChoiceText,
				})
			}
		}
		samples = append(samples, pq)
	}

	return &PreviewResponse{
		ExamSet:         previewSet,
		Readiness:       *readiness,
		SampleQuestions: samples,
	}, nil
}

func (uc *AdminUseCase) Publish(ctx context.Context, id uuid.UUID) (*PublishStatusResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	assigned, err := uc.setQuestions.ListByExamSetID(ctx, id)
	if err != nil {
		return nil, err
	}
	readiness := buildReadiness(set, assigned)
	if !readiness.Ready {
		errCopy := *apperrors.ErrExamSetNotReady
		errCopy.Details = map[string]any{"checks": readiness.Checks}
		return nil, &errCopy
	}
	if err := uc.sets.UpdateStatus(ctx, id, domain.StatusPublished, true); err != nil {
		return nil, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, id.String(), set.Code)
	}
	return &PublishStatusResponse{
		ID:       id.String(),
		Code:     set.Code,
		Title:    set.Title,
		Status:   domain.StatusPublished,
		IsActive: true,
	}, nil
}

func (uc *AdminUseCase) Unpublish(ctx context.Context, id uuid.UUID) (*PublishStatusResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if err := uc.sets.UpdateStatus(ctx, id, domain.StatusDraft, set.IsActive); err != nil {
		return nil, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, id.String(), set.Code)
	}
	return &PublishStatusResponse{
		ID:     id.String(),
		Status: domain.StatusDraft,
	}, nil
}

func (uc *AdminUseCase) Archive(ctx context.Context, id uuid.UUID) (*PublishStatusResponse, error) {
	set, err := uc.reads.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, apperrors.ErrExamSetNotFound
	}
	if err := uc.sets.UpdateStatus(ctx, id, domain.StatusArchived, false); err != nil {
		return nil, err
	}
	if uc.invalidator != nil {
		uc.invalidator.OnExamSetChanged(ctx, id.String(), set.Code)
	}
	return &PublishStatusResponse{
		ID:       id.String(),
		Status:   domain.StatusArchived,
		IsActive: false,
	}, nil
}

func buildReadiness(set *domain.ExamSet, assigned []qdomain.ExamSetQuestion) *ReadinessResult {
	checks := []ReadinessCheck{}
	blockingFailed := false

	addBlocking := func(key, label, passMsg, failMsg string, passed bool) {
		msg := passMsg
		if !passed {
			msg = failMsg
			blockingFailed = true
		}
		checks = append(checks, ReadinessCheck{
			Key: key, Label: label, Passed: passed, Severity: "error", Message: msg,
		})
	}
	addWarning := func(key, label, passMsg, warnMsg string, passed bool) {
		msg := passMsg
		severity := ""
		if !passed {
			msg = warnMsg
			severity = "warning"
		}
		checks = append(checks, ReadinessCheck{
			Key: key, Label: label, Passed: passed, Severity: severity, Message: msg,
		})
	}

	addBlocking("has_title", "มีชื่อชุดข้อสอบ", "ตั้งชื่อชุดข้อสอบแล้ว", "ยังไม่ได้ตั้งชื่อชุดข้อสอบ", strings.TrimSpace(set.Title) != "")
	addBlocking("has_code", "มีรหัสชุดข้อสอบ", "ตั้งรหัสชุดข้อสอบแล้ว", "ยังไม่ได้ตั้งรหัสชุดข้อสอบ", strings.TrimSpace(set.Code) != "")
	addBlocking("has_exam_track", "มีสายการสอบ", "เลือกสายการสอบแล้ว", "ยังไม่ได้เลือกสายการสอบ", set.ExamTrackID != uuid.Nil)
	addBlocking("has_description", "มีคำอธิบาย", "มีคำอธิบายชุดข้อสอบแล้ว", "ยังไม่มีคำอธิบายชุดข้อสอบ", strings.TrimSpace(set.Description) != "")
	addBlocking("has_duration", "กำหนดเวลาสอบ", "กำหนดเวลาสอบแล้ว", "ยังไม่ได้กำหนดเวลาสอบ", set.DurationMinutes > 0)
	addBlocking("has_passing_score", "กำหนดคะแนนผ่าน", "กำหนดคะแนนผ่านแล้ว", "คะแนนผ่านต้องอยู่ระหว่าง 0–100", set.PassingScore >= 0 && set.PassingScore <= 100)

	assignedCount := len(assigned)
	addBlocking("has_questions", "มีคำถามในชุดข้อสอบ", "มีคำถามในชุดข้อสอบแล้ว", "ยังไม่มีคำถามในชุดข้อสอบ", assignedCount > 0)
	addBlocking("total_questions_match", "จำนวนข้อตรงกับที่กำหนด",
		"จำนวนข้อที่กำหนดตรงกับคำถามที่มอบหมายแล้ว",
		"จำนวนข้อที่กำหนดไม่ตรงกับคำถามที่มอบหมาย",
		assignedCount == 0 || set.TotalQuestions == assignedCount)

	publishedCount := 0
	draftCount := 0
	invalidCount := 0
	allPublished := assignedCount > 0
	allActive := assignedCount > 0
	allHave4Choices := assignedCount > 0
	allHaveCorrect := assignedCount > 0
	missingExplanation := false

	for _, sq := range assigned {
		if sq.Question == nil {
			invalidCount++
			allPublished = false
			allActive = false
			allHave4Choices = false
			allHaveCorrect = false
			continue
		}
		q := sq.Question
		switch q.Status {
		case qdomain.StatusPublished:
			if q.IsActive {
				publishedCount++
			} else {
				allActive = false
			}
		case qdomain.StatusDraft:
			draftCount++
			allPublished = false
		default:
			allPublished = false
		}
		if !q.IsActive {
			allActive = false
		}
		if len(q.Choices) != 4 {
			allHave4Choices = false
			invalidCount++
		} else {
			correct := 0
			for _, ch := range q.Choices {
				if ch.IsCorrect {
					correct++
				}
			}
			if correct != 1 {
				allHaveCorrect = false
				invalidCount++
			}
		}
		if strings.TrimSpace(q.Explanation) == "" {
			missingExplanation = true
		}
	}

	questionsReady := allPublished && allActive
	addBlocking("questions_are_published", "คำถามทั้งหมดเผยแพร่แล้ว",
		"คำถามทั้งหมดเผยแพร่และเปิดใช้งานแล้ว",
		"มีคำถาม draft หรือ archived อยู่ในชุดข้อสอบ",
		questionsReady)

	choicesMsg := "ทุกคำถามมีตัวเลือกครบ 4 ตัวเลือก"
	if !allHave4Choices {
		choicesMsg = "มีคำถามที่ตัวเลือกไม่ครบ 4 ตัวเลือก"
	}
	addBlocking("questions_have_choices", "ทุกคำถามมีตัวเลือกครบ", choicesMsg, choicesMsg, allHave4Choices)

	correctMsg := "ทุกคำถามมีเฉลยถูกต้อง"
	if !allHaveCorrect {
		correctMsg = "มีคำถามที่ไม่มีเฉลยหรือมีเฉลยมากกว่า 1 ตัวเลือก"
	}
	addBlocking("questions_have_correct_answer", "ทุกคำถามมีเฉลย", correctMsg, correctMsg, allHaveCorrect)

	coverEmpty := set.CoverImageURL == nil || strings.TrimSpace(*set.CoverImageURL) == ""
	addWarning("has_cover_image", "มีรูปปก",
		"มีรูปปกชุดข้อสอบแล้ว", "ยังไม่มีรูปปกชุดข้อสอบ (ไม่บังคับ)", !coverEmpty)

	premiumNoPrice := set.AccessType == domain.AccessPremium && set.PriceAmount <= 0
	addWarning("has_price", "กำหนดราคา",
		"กำหนดราคาแล้ว", "ชุด Premium ยังไม่ได้กำหนดราคา (ไม่บังคับ)", !premiumNoPrice)

	addWarning("has_explanations", "มีคำอธิบายเฉลย",
		"คำถามทุกข้อมีคำอธิบายเฉลย", "มีบางคำถามที่ยังไม่มีคำอธิบายเฉลย (ไม่บังคับ)", !missingExplanation)

	status := set.Status
	if status == "" {
		status = domain.StatusDraft
	}

	return &ReadinessResult{
		ExamSetID: set.ID.String(),
		Ready:     !blockingFailed,
		Status:    status,
		Checks:    checks,
		Summary: ReadinessSummary{
			TotalQuestions:     assignedCount,
			PublishedQuestions: publishedCount,
			DraftQuestions:     draftCount,
			InvalidQuestions:   invalidCount,
		},
	}
}
