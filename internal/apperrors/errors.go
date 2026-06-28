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

func NewWithDetails(code, message string, status int, details any) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status, Details: details}
}

func ValidationError(message string) *AppError {
	return New("VALIDATION_ERROR", message, http.StatusBadRequest)
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
	ErrAttemptNotEditable      = New("ATTEMPT_NOT_EDITABLE", "ไม่สามารถแก้ไขคำตอบได้", http.StatusBadRequest)
	ErrAttemptNotSubmitted     = New("ATTEMPT_NOT_SUBMITTED", "ยังไม่ได้ส่งคำตอบ ไม่สามารถดูผลสอบได้", http.StatusBadRequest)
	ErrResultNotAvailable      = New("RESULT_NOT_AVAILABLE", "ยังไม่สามารถดูผลสอบได้", http.StatusBadRequest)
	ErrUnauthorizedAttemptAccess = New("UNAUTHORIZED_ATTEMPT_ACCESS", "ไม่มีสิทธิ์เข้าถึงการสอบนี้", http.StatusForbidden)
	ErrQuestionNotFound   = New("QUESTION_NOT_FOUND", "ไม่พบข้อสอบ", http.StatusNotFound)
	ErrEmailTaken         = New("EMAIL_TAKEN", "อีเมลนี้ถูกใช้งานแล้ว", http.StatusConflict)
	ErrInvalidCredentials = New("INVALID_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง", http.StatusUnauthorized)
	ErrPremiumRequired         = New("PREMIUM_REQUIRED", "ชุดข้อสอบนี้สำหรับสมาชิก Premium เท่านั้น", http.StatusForbidden)
	ErrAccessRequired          = New("ACCESS_REQUIRED", "ชุดข้อสอบนี้ต้องปลดล็อกก่อนเริ่มทำข้อสอบ", http.StatusForbidden)
	ErrLoginRequired           = New("LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อนเริ่มทำข้อสอบ", http.StatusUnauthorized)
	ErrEntitlementNotFound     = New("ENTITLEMENT_NOT_FOUND", "ไม่พบสิทธิ์การเข้าถึง", http.StatusNotFound)
	ErrEntitlementAlreadyExists = New("ENTITLEMENT_ALREADY_EXISTS", "ผู้ใช้งานมีสิทธิ์นี้อยู่แล้ว", http.StatusConflict)
	ErrInvalidAccessType       = New("INVALID_ACCESS_TYPE", "ประเภทการเข้าถึงไม่ถูกต้อง", http.StatusBadRequest)
	ErrInvalidAccessConfig     = New("INVALID_ACCESS_CONFIG", "การตั้งค่าการเข้าถึงและราคาไม่ถูกต้อง", http.StatusBadRequest)
	ErrPrivateExamAccessRequired = New("PRIVATE_EXAM_ACCESS_REQUIRED", "ชุดข้อสอบนี้เปิดให้เฉพาะผู้ได้รับสิทธิ์เท่านั้น", http.StatusForbidden)
	ErrExamNotAvailable          = New("EXAM_NOT_AVAILABLE", "ชุดข้อสอบนี้ยังไม่พร้อมให้ใช้งาน", http.StatusBadRequest)
	ErrInvalidEntitlement      = New("INVALID_ENTITLEMENT", "ข้อมูลสิทธิ์การเข้าถึงไม่ถูกต้อง", http.StatusBadRequest)
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
	ErrTagNotFound             = New("TAG_NOT_FOUND", "ไม่พบกลุ่มคำถามหรือกลุ่มคำถามไม่เปิดใช้งาน", http.StatusBadRequest)
	ErrTagHasQuestions         = New("TAG_HAS_QUESTIONS", "ไม่สามารถลบกลุ่มคำถามที่มีคำถามอยู่ ระบบปิดใช้งานแทน", http.StatusBadRequest)
	ErrAccountSuspended        = New("ACCOUNT_SUSPENDED", "บัญชีนี้ถูกระงับการใช้งาน กรุณาติดต่อผู้ดูแลระบบ", http.StatusForbidden)
	ErrLastAdmin               = New("LAST_ADMIN", "ไม่สามารถเปลี่ยนแปลงผู้ดูแลระบบคนสุดท้ายได้", http.StatusBadRequest)
)
