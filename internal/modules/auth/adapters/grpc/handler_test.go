package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	authv1 "github.com/gnha/golang-echo-boilerplate/gen/proto/auth/v1"
	grpcadapter "github.com/gnha/golang-echo-boilerplate/internal/modules/auth/adapters/grpc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/auth/app"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
	"github.com/golang-jwt/jwt/v5"
)


// testCfg returns a minimal config suitable for unit tests.
func testCfg() *config.Config {
	//nolint:gosec // hardcoded secret is test-only
	return &config.Config{
		AppName:      "test",
		JWTSecret:    "test-secret-32-bytes-long-padding!",
		JWTAccessTTL: 15 * time.Minute,
	}
}

// stubLookup implements auth.CredentialLookup for testing.
type stubLookup struct {
	userID  string
	hashPwd string
	role    string
	err     error
}

func (s *stubLookup) GetByEmail(_ context.Context, _ string) (string, string, string, error) {
	return s.userID, s.hashPwd, s.role, s.err
}

// buildHandler wires up a real AuthServiceHandler with the given lookup and blacklister.
func buildHandler(t *testing.T, lookup auth.CredentialLookup, bl *testutil.StubBlacklister) *grpcadapter.AuthServiceHandler {
	t.Helper()
	cfg := testCfg()
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	loginH := app.NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)
	logoutH := app.NewLogoutHandler(bl, bus)
	return grpcadapter.NewAuthServiceHandler(loginH, logoutH, cfg, bl)
}

// validToken generates a real signed JWT using the test config.
func validToken(t *testing.T, userID, jti string, ttl time.Duration) string {
	t.Helper()
	cfg := testCfg()
	claims := auth.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    cfg.AppName,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{"golang-echo-boilerplate"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
		Role:   "member",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("validToken: sign failed: %v", err)
	}
	return signed
}

// --- Login tests ---

func TestAuthHandler_Login_Success(t *testing.T) {
	//nolint:gosec // test data
	lookup := &stubLookup{
		userID:  "user-1",
		hashPwd: "$argon2id$v=19$test$hashed_correct",
		role:    "member",
	}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	resp, err := h.Login(context.Background(), connect.NewRequest(&authv1.LoginRequest{
		Email:    "user@example.com",
		Password: "correct",
	}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Msg.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.Msg.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", resp.Msg.ExpiresIn)
	}
}

func TestAuthHandler_Login_InvalidCredentials_ReturnsUnauthenticated(t *testing.T) {
	lookup := &stubLookup{err: sharederr.ErrNotFound()}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	_, err := h.Login(context.Background(), connect.NewRequest(&authv1.LoginRequest{
		Email:    "ghost@example.com",
		Password: "wrong",
	}))
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Login_WrongPassword_ReturnsUnauthenticated(t *testing.T) {
	//nolint:gosec // test data
	lookup := &stubLookup{
		userID:  "user-2",
		hashPwd: "$argon2id$v=19$test$hashed_correct",
		role:    "member",
	}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	_, err := h.Login(context.Background(), connect.NewRequest(&authv1.LoginRequest{
		Email:    "user@example.com",
		Password: "wrong",
	}))
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Login_InternalError_ReturnsInternal(t *testing.T) {
	lookup := &stubLookup{err: fmt.Errorf("db connection lost")}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	_, err := h.Login(context.Background(), connect.NewRequest(&authv1.LoginRequest{
		Email:    "user@example.com",
		Password: "password",
	}))
	assertConnectCode(t, err, connect.CodeInternal)
}

// --- Logout tests ---

func TestAuthHandler_Logout_Success(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	token := validToken(t, "user-1", "jti-logout-ok", 15*time.Minute)
	req := connect.NewRequest(&authv1.LogoutRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	resp, err := h.Logout(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestAuthHandler_Logout_MissingAuthHeader_ReturnsUnauthenticated(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	req := connect.NewRequest(&authv1.LogoutRequest{})
	// No Authorization header set.

	_, err := h.Logout(context.Background(), req)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Logout_InvalidJWT_ReturnsUnauthenticated(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	req := connect.NewRequest(&authv1.LogoutRequest{})
	req.Header().Set("Authorization", "Bearer not.a.valid.jwt")

	_, err := h.Logout(context.Background(), req)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Logout_ExpiredJWT_ReturnsUnauthenticated(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	// Generate a token that is already expired.
	token := validToken(t, "user-1", "jti-expired", -time.Second)
	req := connect.NewRequest(&authv1.LogoutRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err := h.Logout(context.Background(), req)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Logout_BlacklistedToken_ReturnsUnauthenticated(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	const jti = "jti-blacklisted"
	// Pre-populate the blacklist so the handler's defense-in-depth check fires.
	_ = bl.Blacklist(context.Background(), jti, time.Now().Add(15*time.Minute))

	token := validToken(t, "user-1", jti, 15*time.Minute)
	req := connect.NewRequest(&authv1.LogoutRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err := h.Logout(context.Background(), req)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestAuthHandler_Logout_BearerCaseInsensitive(t *testing.T) {
	lookup := &stubLookup{}
	bl := testutil.NewStubBlacklister()
	h := buildHandler(t, lookup, bl)

	token := validToken(t, "user-1", "jti-case", 15*time.Minute)
	req := connect.NewRequest(&authv1.LogoutRequest{})
	req.Header().Set("Authorization", "bearer "+token) // lowercase

	_, err := h.Logout(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success with lowercase 'bearer', got %v", err)
	}
}

// --- Constructor panic tests ---

func TestNewAuthServiceHandler_NilPanics(t *testing.T) {
	cfg := testCfg()
	bus := events.NewEventBus(&testutil.NoopPublisher{})
	bl := testutil.NewStubBlacklister()
	lookup := &stubLookup{}
	loginH := app.NewLoginHandler(lookup, &testutil.StubHasher{}, cfg, bus)
	logoutH := app.NewLogoutHandler(bl, bus)

	testutil.AssertPanics(t, "nil login", func() {
		grpcadapter.NewAuthServiceHandler(nil, logoutH, cfg, bl)
	})
	testutil.AssertPanics(t, "nil logout", func() {
		grpcadapter.NewAuthServiceHandler(loginH, nil, cfg, bl)
	})
	testutil.AssertPanics(t, "nil cfg", func() {
		grpcadapter.NewAuthServiceHandler(loginH, logoutH, nil, bl)
	})
	testutil.AssertPanics(t, "nil blacklister", func() {
		grpcadapter.NewAuthServiceHandler(loginH, logoutH, cfg, nil)
	})
}

// --- helpers ---

func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected connect error with code %v, got nil", want)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if ce.Code() != want {
		t.Errorf("connect code = %v, want %v", ce.Code(), want)
	}
}
