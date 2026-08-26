package http

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"virtual-exam-api/internal/apperrors"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	"virtual-exam-api/internal/middleware"
	"virtual-exam-api/internal/payment/domain"
	"virtual-exam-api/internal/payment/proofstore"
	paymentuc "virtual-exam-api/internal/payment/usecase"
	"virtual-exam-api/internal/response"
)

type Handler struct {
	payments *paymentuc.UseCase
	audit    *audituc.Logger
}

func NewHandler(payments *paymentuc.UseCase, audit *audituc.Logger) *Handler {
	return &Handler{payments: payments, audit: audit}
}

func (h *Handler) RegisterPublicRoutes(api *echo.Group) {
	api.GET("/packages", h.ListPackages)
}

func (h *Handler) RegisterUserRoutes(api *echo.Group, auth echo.MiddlewareFunc) {
	api.POST("/payments", h.CreatePayment, auth)
	api.GET("/payments/:id", h.GetPayment, auth)
	api.POST("/payments/:id/proof", h.UploadProof, auth)
	api.GET("/payments/:id/proof", h.GetProof, auth)
}

func (h *Handler) RegisterAdminRoutes(admin *echo.Group) {
	admin.GET("/payments", h.ListPayments)
	admin.GET("/payments/:id", h.GetAdminPayment)
	admin.GET("/payments/:id/proof", h.GetAdminProof)
	admin.POST("/payments/:id/approve", h.Approve)
	admin.POST("/payments/:id/reject", h.Reject)
}

func (h *Handler) ListPackages(c echo.Context) error {
	return response.JSON(c, http.StatusOK, h.payments.Packages())
}

func (h *Handler) CreatePayment(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	var body struct {
		PackageID string `json:"packageId"`
	}
	if err := c.Bind(&body); err != nil || strings.TrimSpace(body.PackageID) == "" {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	payment, err := h.payments.CreatePayment(c.Request().Context(), userID, body.PackageID)
	if err != nil {
		return response.Error(c, mapError(err))
	}
	return response.JSON(c, http.StatusCreated, payment)
}

func (h *Handler) GetPayment(c echo.Context) error {
	return h.getPayment(c, false)
}

func (h *Handler) GetAdminPayment(c echo.Context) error {
	return h.getPayment(c, true)
}

func (h *Handler) getPayment(c echo.Context, isAdmin bool) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}
	payment, err := h.payments.GetPayment(c.Request().Context(), userID, id, isAdmin)
	if err != nil {
		return response.Error(c, mapError(err))
	}
	return response.JSON(c, http.StatusOK, payment)
}

func (h *Handler) UploadProof(c echo.Context) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size <= 0 || file.Size > proofstore.MaxProofSize {
		return response.Error(c, apperrors.New("INVALID_PAYMENT_PROOF", "รองรับไฟล์ JPG, PNG, WebP หรือ PDF ขนาดไม่เกิน 10 MB", http.StatusBadRequest))
	}
	src, err := file.Open()
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidInput)
	}
	defer src.Close()
	payment, err := h.payments.UploadProof(c.Request().Context(), userID, id, file.Filename, src, file.Size)
	if err != nil {
		return response.Error(c, mapError(err))
	}
	return response.JSON(c, http.StatusOK, payment)
}

func (h *Handler) GetProof(c echo.Context) error      { return h.getProof(c, false) }
func (h *Handler) GetAdminProof(c echo.Context) error { return h.getProof(c, true) }

func (h *Handler) getProof(c echo.Context, isAdmin bool) error {
	userID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}
	r, filename, contentType, err := h.payments.OpenProof(c.Request().Context(), userID, id, isAdmin)
	if err != nil {
		return response.Error(c, mapError(err))
	}
	defer r.Close()
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("inline; filename=%s", mime.QEncoding.Encode("UTF-8", filename)))
	return c.Stream(http.StatusOK, contentType, r)
}

func (h *Handler) ListPayments(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	result, err := h.payments.List(c.Request().Context(), domain.ListParams{
		Page: page, Limit: limit, Status: c.QueryParam("status"), Query: c.QueryParam("q"),
	})
	if err != nil {
		return response.Error(c, err)
	}
	return response.JSON(c, http.StatusOK, result)
}

func (h *Handler) Approve(c echo.Context) error {
	return h.review(c, true)
}

func (h *Handler) Reject(c echo.Context) error {
	return h.review(c, false)
}

func (h *Handler) review(c echo.Context, approve bool) error {
	reviewerID, err := middleware.RequireUserID(c)
	if err != nil {
		return response.Error(c, err)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.Error(c, apperrors.ErrInvalidUUID)
	}
	var payment *domain.PaymentRequest
	if approve {
		payment, err = h.payments.Approve(c.Request().Context(), id, reviewerID)
	} else {
		var body struct {
			Reason string `json:"reason"`
		}
		if bindErr := c.Bind(&body); bindErr != nil || strings.TrimSpace(body.Reason) == "" {
			return response.Error(c, apperrors.ValidationError("กรุณาระบุเหตุผลที่ปฏิเสธ"))
		}
		payment, err = h.payments.Reject(c.Request().Context(), id, reviewerID, strings.TrimSpace(body.Reason))
	}
	if err != nil {
		return response.Error(c, mapError(err))
	}
	if h.audit != nil {
		action := "payment.reject"
		if approve {
			action = "payment.approve"
		}
		h.audit.Log(c.Request().Context(), audituc.LogInput{
			ActorUserID: &reviewerID, Action: action, ResourceType: "payment", ResourceID: &id,
			BeforeData: map[string]any{"status": domain.StatusPendingReview},
			AfterData: map[string]any{
				"status": payment.Status, "package_id": payment.PackageID, "sale_price": payment.SalePrice,
				"currency": payment.Currency, "entitlement_id": payment.EntitlementID,
				"rejection_reason": payment.RejectionReason,
			},
			IPAddress: c.RealIP(), UserAgent: c.Request().UserAgent(),
		})
	}
	return response.JSON(c, http.StatusOK, payment)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.New("PAYMENT_NOT_FOUND", "ไม่พบรายการชำระเงิน", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		return apperrors.ErrForbidden
	case errors.Is(err, domain.ErrPackageNotPurchasable):
		return apperrors.New("PACKAGE_NOT_PURCHASABLE", "แพ็กเกจนี้ยังไม่เปิดขาย", http.StatusBadRequest)
	case errors.Is(err, domain.ErrQRExpired):
		return apperrors.New("PAYMENT_QR_EXPIRED", "QR หมดอายุแล้ว กรุณาสร้างรายการใหม่", http.StatusConflict)
	case errors.Is(err, domain.ErrStateConflict):
		return apperrors.New("PAYMENT_STATE_CONFLICT", "สถานะรายการชำระเงินมีการเปลี่ยนแปลง กรุณาโหลดข้อมูลใหม่", http.StatusConflict)
	case errors.Is(err, domain.ErrInvalidProof):
		return apperrors.New("INVALID_PAYMENT_PROOF", "รองรับไฟล์ JPG, PNG, WebP หรือ PDF ขนาดไม่เกิน 10 MB", http.StatusBadRequest)
	default:
		return err
	}
}
