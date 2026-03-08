package middleware

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/gnha/gnha-services/internal/shared/auth"
)

// newRequestWithProcedure creates a connect.Request[struct{}] with the given
// procedure set via reflection (the spec field is unexported).
func newRequestWithProcedure(procedure string) *connect.Request[struct{}] {
	req := connect.NewRequest(&struct{}{})
	// connect.Spec only has exported fields, so we can build it directly.
	spec := connect.Spec{Procedure: procedure}
	// Use reflection to set the unexported spec field on the request.
	rv := reflect.ValueOf(req).Elem()
	rf := rv.FieldByName("spec")
	// reflect.NewAt gives us a settable pointer to the same memory.
	reflect.NewAt(rf.Type(), rf.Addr().UnsafePointer()).Elem().Set(reflect.ValueOf(spec))
	return req
}

// callInterceptor exercises the RBACInterceptor with the given context and procedure.
func callInterceptor(ctx context.Context, procedure string) error {
	interceptor := RBACInterceptor()
	handler := interceptor(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	req := newRequestWithProcedure(procedure)
	_, err := handler(ctx, req)
	return err
}

func ctxWithPermissions(permissions ...string) context.Context {
	claims := &auth.TokenClaims{
		UserID:      "user-1",
		Role:        "member",
		Permissions: permissions,
	}
	return auth.WithUser(context.Background(), claims)
}

func ctxWithAdminRole() context.Context {
	claims := &auth.TokenClaims{
		UserID:      "admin-1",
		Role:        "admin",
		Permissions: []string{"admin:*"},
	}
	return auth.WithUser(context.Background(), claims)
}

// TestPermissionForProcedure_Mapping verifies the procedure-to-permission table.
func TestPermissionForProcedure_Mapping(t *testing.T) {
	cases := []struct {
		procedure string
		want      Permission
	}{
		{"/user.v1.UserService/CreateUser", PermUserWrite},
		{"/user.v1.UserService/UpdateUser", PermUserWrite},
		{"/user.v1.UserService/DeleteUser", PermUserDelete},
		{"/user.v1.UserService/GetUser", ""},   // read — no write check
		{"/user.v1.UserService/ListUsers", ""}, // read — no write check
		{"malformed", ""},
	}
	for _, tc := range cases {
		got := procedurePermissions[tc.procedure]
		if got != tc.want {
			t.Errorf("procedurePermissions[%q] = %q, want %q", tc.procedure, got, tc.want)
		}
	}
}

func TestRBACInterceptor_UnmappedProcedure_PassesThrough(t *testing.T) {
	// GetUser has no required permission — should pass even without user context.
	err := callInterceptor(context.Background(), "/user.v1.UserService/GetUser")
	if err != nil {
		t.Errorf("expected no error for unmapped procedure, got %v", err)
	}
}

func TestRBACInterceptor_CreateUser_WithPermission_Passes(t *testing.T) {
	ctx := ctxWithPermissions("user:write")
	err := callInterceptor(ctx, "/user.v1.UserService/CreateUser")
	if err != nil {
		t.Errorf("expected no error with user:write permission, got %v", err)
	}
}

func TestRBACInterceptor_CreateUser_WithoutPermission_ReturnsPermissionDenied(t *testing.T) {
	ctx := ctxWithPermissions("user:read") // missing user:write
	err := callInterceptor(ctx, "/user.v1.UserService/CreateUser")
	if err == nil {
		t.Fatal("expected error for missing user:write permission")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodePermissionDenied {
		t.Errorf("expected CodePermissionDenied, got %v", connectErr.Code())
	}
}

func TestRBACInterceptor_NoUserContext_ReturnsUnauthenticated(t *testing.T) {
	// No user injected into context — mapped procedure must return Unauthenticated.
	err := callInterceptor(context.Background(), "/user.v1.UserService/CreateUser")
	if err == nil {
		t.Fatal("expected error for missing user context")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
	}
}

func TestRBACInterceptor_DeleteUser_WithPermission_Passes(t *testing.T) {
	ctx := ctxWithPermissions("user:delete")
	err := callInterceptor(ctx, "/user.v1.UserService/DeleteUser")
	if err != nil {
		t.Errorf("expected no error with user:delete permission, got %v", err)
	}
}

func TestRBACInterceptor_AdminRole_PassesAll(t *testing.T) {
	// Admin role implicitly has all permissions via HasPermission fallback.
	ctx := ctxWithAdminRole()
	procedures := []string{
		"/user.v1.UserService/CreateUser",
		"/user.v1.UserService/UpdateUser",
		"/user.v1.UserService/DeleteUser",
	}
	for _, proc := range procedures {
		if err := callInterceptor(ctx, proc); err != nil {
			t.Errorf("expected admin role to pass %s, got %v", proc, err)
		}
	}
}

func TestRBACInterceptor_AdminWildcard_Permission_PassesAll(t *testing.T) {
	// Explicit admin:* wildcard permission also grants access to everything.
	ctx := ctxWithPermissions("admin:*")
	procedures := []string{
		"/user.v1.UserService/CreateUser",
		"/user.v1.UserService/UpdateUser",
		"/user.v1.UserService/DeleteUser",
	}
	for _, proc := range procedures {
		if err := callInterceptor(ctx, proc); err != nil {
			t.Errorf("expected admin:* to pass %s, got %v", proc, err)
		}
	}
}
