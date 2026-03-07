package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/labstack/echo/v4"
)

// injectClaims returns middleware that puts a fake authenticated user into context.
func injectClaims(userID, role string, perms []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := &auth.TokenClaims{}
			claims.UserID = userID
			claims.Role = role
			claims.Permissions = perms
			ctx := auth.WithUser(c.Request().Context(), claims)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func okHandler(c echo.Context) error { return c.String(http.StatusOK, "ok") }

// newRBACEcho builds an Echo instance with the given middleware chain before the handler.
func newRBACEcho(middlewares ...echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/", okHandler, middlewares...)
	return e
}

func doGet(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// --- RequirePermission tests ---

func TestRequirePermission_HasPermission_Passes(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "member", []string{"user:read"}),
		RequirePermission(PermUserRead),
	)
	rec := doGet(e)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePermission_MissingPermission_Returns403(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "member", []string{"user:read"}),
		RequirePermission(PermUserWrite),
	)
	rec := doGet(e)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequirePermission_AdminWildcard_Passes(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "member", []string{string(PermAdminAll)}),
		RequirePermission(PermUserDelete),
	)
	rec := doGet(e)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for admin:* wildcard, got %d", rec.Code)
	}
}

func TestRequirePermission_AdminRole_Passes(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "admin", nil),
		RequirePermission(PermUserDelete),
	)
	rec := doGet(e)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for admin role, got %d", rec.Code)
	}
}

func TestRequirePermission_NoUser_Returns401(t *testing.T) {
	e := newRBACEcho(
		RequirePermission(PermUserRead),
	)
	rec := doGet(e)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no user in context, got %d", rec.Code)
	}
}

// --- RequireRole tests ---

func TestRequireRole_MatchingRole_Passes(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "admin", nil),
		RequireRole("admin"),
	)
	rec := doGet(e)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_WrongRole_Returns403(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "member", nil),
		RequireRole("admin"),
	)
	rec := doGet(e)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_OneOfMultipleRoles_Passes(t *testing.T) {
	e := newRBACEcho(
		injectClaims("u1", "viewer", nil),
		RequireRole("admin", "member", "viewer"),
	)
	rec := doGet(e)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for one of multiple roles, got %d", rec.Code)
	}
}

func TestRequireRole_NoUser_Returns401(t *testing.T) {
	e := newRBACEcho(
		RequireRole("admin"),
	)
	rec := doGet(e)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no user in context, got %d", rec.Code)
	}
}
