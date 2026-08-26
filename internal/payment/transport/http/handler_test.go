package http

import (
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRegisterRoutesExposesPackageUserAndAdminPaymentEndpoints(t *testing.T) {
	e := echo.New()
	api := e.Group("/api/v1")
	auth := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	h := NewHandler(nil, nil)
	h.RegisterPublicRoutes(api)
	h.RegisterUserRoutes(api, auth)
	h.RegisterAdminRoutes(api.Group("/admin", auth))

	want := map[string]bool{
		"GET /api/v1/packages":                    false,
		"POST /api/v1/payments":                   false,
		"GET /api/v1/payments/:id":                false,
		"POST /api/v1/payments/:id/proof":         false,
		"GET /api/v1/payments/:id/proof":          false,
		"GET /api/v1/admin/payments":              false,
		"GET /api/v1/admin/payments/:id":          false,
		"GET /api/v1/admin/payments/:id/proof":    false,
		"POST /api/v1/admin/payments/:id/approve": false,
		"POST /api/v1/admin/payments/:id/reject":  false,
	}
	for _, route := range e.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
