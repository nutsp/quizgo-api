package apperrors

import "net/http"

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Details    any    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status}
}

var (
	ErrUnauthorized       = New("UNAUTHORIZED", "กรุณาเข้าสู่ระบบ", http.StatusUnauthorized)
	ErrForbidden          = New("FORBIDDEN", "ไม่มีสิทธิ์เข้าถึง", http.StatusForbidden)
	ErrNotFound           = New("NOT_FOUND", "ไม่พบข้อมูล", http.StatusNotFound)
	ErrInvalidInput       = New("INVALID_INPUT", "ข้อมูลไม่ถูกต้อง", http.StatusBadRequest)
	ErrInvalidUUID        = New("INVALID_UUID", "รหัสไม่ถูกต้อง", http.StatusBadRequest)
	ErrInvalidChoiceKey   = New("INVALID_CHOICE_KEY", "ตัวเลือกคำตอบไม่ถูกต้อง", http.StatusBadRequest)
	ErrExamSetNotFound    = New("EXAM_SET_NOT_FOUND", "ไม่พบชุดข้อสอบ", http.StatusNotFound)
	ErrExamTrackNotFound  = New("EXAM_TRACK_NOT_FOUND", "ไม่พบสายข้อสอบ", http.StatusNotFound)
	ErrAttemptNotFound    = New("ATTEMPT_NOT_FOUND", "ไม่พบข้อมูลการสอบ", http.StatusNotFound)
	ErrAttemptExpired     = New("ATTEMPT_EXPIRED", "หมดเวลาทำข้อสอบแล้ว", http.StatusBadRequest)
	ErrAttemptSubmitted   = New("ATTEMPT_SUBMITTED", "ส่งคำตอบแล้ว ไม่สามารถแก้ไขได้", http.StatusBadRequest)
	ErrAttemptNotEditable = New("ATTEMPT_NOT_EDITABLE", "ไม่สามารถแก้ไขคำตอบได้", http.StatusBadRequest)
	ErrQuestionNotFound   = New("QUESTION_NOT_FOUND", "ไม่พบข้อสอบ", http.StatusNotFound)
	ErrEmailTaken         = New("EMAIL_TAKEN", "อีเมลนี้ถูกใช้งานแล้ว", http.StatusConflict)
	ErrInvalidCredentials = New("INVALID_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง", http.StatusUnauthorized)
	ErrPremiumRequired    = New("PREMIUM_REQUIRED", "ชุดข้อสอบนี้ต้องใช้สิทธิ์ Premium", http.StatusForbidden)
	ErrExamSetInactive    = New("EXAM_SET_INACTIVE", "ชุดข้อสอบนี้ไม่เปิดให้ทำ", http.StatusBadRequest)
	ErrCodeTaken          = New("CODE_TAKEN", "รหัสนี้ถูกใช้งานแล้ว", http.StatusConflict)
	ErrSubjectHasQuestions = New("SUBJECT_HAS_QUESTIONS", "ไม่สามารถลบหมวดวิชาที่มีคำถามอยู่", http.StatusBadRequest)
	ErrDuplicateQuestion  = New("DUPLICATE_QUESTION", "คำถามนี้อยู่ในชุดข้อสอบแล้ว", http.StatusConflict)
	ErrInvalidChoices     = New("INVALID_CHOICES", "ตัวเลือกคำตอบไม่ถูกต้อง", http.StatusBadRequest)
	ErrQuestionNotPublished    = New("QUESTION_NOT_PUBLISHED", "เฉพาะคำถามที่เผยแพร่แล้วเท่านั้นที่เพิ่มได้", http.StatusBadRequest)
	ErrExamSetLockedByAttempts = New("EXAM_SET_LOCKED_BY_ATTEMPTS", "ชุดข้อสอบนี้มีผลสอบแล้ว ไม่สามารถแก้ไขคำถามในชุดได้", http.StatusConflict)
	ErrExamSetHasAttempts      = New("EXAM_SET_HAS_ATTEMPTS", "ชุดข้อสอบนี้มีประวัติการทำข้อสอบแล้ว ไม่สามารถลบคำถามทั้งหมดได้", http.StatusConflict)
	ErrExamSetHasNoQuestions   = New("EXAM_SET_HAS_NO_QUESTIONS", "ชุดข้อสอบนี้ยังไม่มีคำถาม", http.StatusBadRequest)
	ErrExamSetNotPublished     = New("EXAM_SET_NOT_PUBLISHED", "ชุดข้อสอบนี้ยังไม่เปิดให้ทำข้อสอบ", http.StatusBadRequest)
	ErrExamSetNotReady         = New("EXAM_SET_NOT_READY", "ชุดข้อสอบยังไม่พร้อมเผยแพร่", http.StatusBadRequest)
)
