package grpc

import (
	"testing"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
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
