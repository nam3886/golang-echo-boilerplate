package connectutil_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/connectutil"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

func connectCode(err error) connect.Code {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr.Code()
	}
	return 0
}

func TestDomainErrorToConnect_KnownCodes(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{"not found", sharederr.ErrNotFound(), connect.CodeNotFound},
		{"already exists", sharederr.ErrAlreadyExists(), connect.CodeAlreadyExists},
		{"forbidden", sharederr.ErrForbidden(), connect.CodePermissionDenied},
		{"unauthorized", sharederr.ErrUnauthorized(), connect.CodeUnauthenticated},
		{"internal", sharederr.ErrInternal(), connect.CodeInternal},
		{"no change", sharederr.ErrNoChange(), connect.CodeFailedPrecondition},
		{"invalid argument", sharederr.New(sharederr.CodeInvalidArgument, "", "bad input"), connect.CodeInvalidArgument},
		{"unavailable", sharederr.New(sharederr.CodeUnavailable, "", "down"), connect.CodeUnavailable},
		{"resource exhausted", sharederr.New(sharederr.CodeResourceExhausted, "", "limit"), connect.CodeResourceExhausted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectutil.DomainErrorToConnect(ctx, tc.err)
			if got == nil {
				t.Fatal("expected non-nil error")
			}
			if code := connectCode(got); code != tc.wantCode {
				t.Errorf("expected code %v, got %v", tc.wantCode, code)
			}
		})
	}
}

func TestDomainErrorToConnect_UnknownCodeDefaultsToInternal(t *testing.T) {
	unknown := &sharederr.DomainError{Code: "TOTALLY_UNKNOWN", Message: "oops"}
	got := connectutil.DomainErrorToConnect(context.Background(), unknown)
	if got == nil {
		t.Fatal("expected non-nil error")
	}
	if code := connectCode(got); code != connect.CodeInternal {
		t.Errorf("expected CodeInternal for unknown code, got %v", code)
	}
}

func TestDomainErrorToConnect_NonDomainError(t *testing.T) {
	plain := errors.New("something went wrong")
	got := connectutil.DomainErrorToConnect(context.Background(), plain)
	if got == nil {
		t.Fatal("expected non-nil error")
	}
	if code := connectCode(got); code != connect.CodeInternal {
		t.Errorf("expected CodeInternal for non-domain error, got %v", code)
	}
}
