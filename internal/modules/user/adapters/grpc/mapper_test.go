package grpc

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/connectutil"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"connectrpc.com/connect"
)

func TestToProto_MapsDomainFields(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	user := domain.Reconstitute(
		"test-id",
		"test@example.com",
		"Test User",
		"hashed_pwd",
		domain.RoleAdmin,
		now,
		now,
		nil,
	)

	pb := toProto(user)

	if pb.Id != "test-id" {
		t.Errorf("expected id 'test-id', got %s", pb.Id)
	}
	if pb.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", pb.Email)
	}
	if pb.Name != "Test User" {
		t.Errorf("expected name 'Test User', got %s", pb.Name)
	}
	if pb.Role != "admin" {
		t.Errorf("expected role 'admin', got %s", pb.Role)
	}
	if pb.CreatedAt.AsTime() != now {
		t.Errorf("expected createdAt %v, got %v", now, pb.CreatedAt.AsTime())
	}
	if pb.UpdatedAt.AsTime() != now {
		t.Errorf("expected updatedAt %v, got %v", now, pb.UpdatedAt.AsTime())
	}
}

func TestToProto_HandlesZeroTimestamps(t *testing.T) {
	user := domain.Reconstitute(
		"id",
		"test@example.com",
		"Test",
		"pwd",
		domain.RoleMember,
		time.Time{},
		time.Time{},
		nil,
	)

	pb := toProto(user)

	if pb.CreatedAt == nil {
		t.Error("expected non-nil CreatedAt proto")
	}
	if pb.UpdatedAt == nil {
		t.Error("expected non-nil UpdatedAt proto")
	}
}

func TestDomainErrorToConnect_DomainError(t *testing.T) {
	tests := []struct {
		code         sharederr.ErrorCode
		expectedCode connect.Code
	}{
		{sharederr.CodeNotFound, connect.CodeNotFound},
		{sharederr.CodeInvalidArgument, connect.CodeInvalidArgument},
		{sharederr.CodeAlreadyExists, connect.CodeAlreadyExists},
		{sharederr.CodePermissionDenied, connect.CodePermissionDenied},
		{sharederr.CodeUnauthenticated, connect.CodeUnauthenticated},
		{sharederr.CodeFailedPrecondition, connect.CodeFailedPrecondition},
		{sharederr.CodeInternal, connect.CodeInternal},
		{sharederr.CodeUnavailable, connect.CodeUnavailable},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := sharederr.New(tt.code, "test message")
			result := connectutil.DomainErrorToConnect(err)

			ce := &connect.Error{}
			if !errors.As(result, &ce) {
				t.Fatalf("expected connect.Error, got %T", result)
			}
			if ce.Code() != tt.expectedCode {
				t.Errorf("expected code %v, got %v", tt.expectedCode, ce.Code())
			}
		})
	}
}

func TestDomainErrorToConnect_NonDomainError(t *testing.T) {
	err := fmt.Errorf("some random error")
	result := connectutil.DomainErrorToConnect(err)

	ce := &connect.Error{}
	if !errors.As(result, &ce) {
		t.Fatalf("expected connect.Error, got %T", result)
	}
	if ce.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal for non-DomainError, got %v", ce.Code())
	}
}
