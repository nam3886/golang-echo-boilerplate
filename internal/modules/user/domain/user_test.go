package domain

import (
	"testing"
)

func TestNewUser_Success(t *testing.T) {
	user, err := NewUser("test@example.com", "Test User", "hashed_pwd", RoleMember)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Email() != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email())
	}
	if user.Name() != "Test User" {
		t.Errorf("expected name Test User, got %s", user.Name())
	}
	if user.Role() != RoleMember {
		t.Errorf("expected role member, got %s", user.Role())
	}
	if user.ID() == "" {
		t.Error("expected non-empty ID")
	}
	if user.CreatedAt().IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewUser_InvalidEmail(t *testing.T) {
	_, err := NewUser("", "Test User", "hashed_pwd", RoleMember)
	if err == nil {
		t.Fatal("expected error for empty email")
	}
	if err != ErrInvalidEmail {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestNewUser_InvalidName(t *testing.T) {
	_, err := NewUser("test@example.com", "", "hashed_pwd", RoleMember)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if err != ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestNewUser_InvalidRole(t *testing.T) {
	_, err := NewUser("test@example.com", "Test User", "hashed_pwd", Role("superadmin"))
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if err != ErrInvalidRole {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestUser_ChangeName(t *testing.T) {
	user, _ := NewUser("test@example.com", "Old Name", "hashed_pwd", RoleMember)
	oldUpdated := user.UpdatedAt()

	if err := user.ChangeName("New Name"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Name() != "New Name" {
		t.Errorf("expected name New Name, got %s", user.Name())
	}
	if user.UpdatedAt().Before(oldUpdated) {
		t.Error("expected updatedAt to not go backwards")
	}
}

func TestUser_ChangeName_Empty(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test User", "hashed_pwd", RoleMember)
	err := user.ChangeName("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if err != ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestUser_ChangeRole(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test User", "hashed_pwd", RoleMember)
	if err := user.ChangeRole(RoleAdmin); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Role() != RoleAdmin {
		t.Errorf("expected role admin, got %s", user.Role())
	}
}

func TestUser_ChangeRole_Invalid(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test User", "hashed_pwd", RoleMember)
	err := user.ChangeRole(Role("superadmin"))
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if err != ErrInvalidRole {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleAdmin, true},
		{RoleMember, true},
		{RoleViewer, true},
		{Role("unknown"), false},
		{Role(""), false},
	}
	for _, tt := range tests {
		if got := tt.role.IsValid(); got != tt.valid {
			t.Errorf("Role(%q).IsValid() = %v, want %v", tt.role, got, tt.valid)
		}
	}
}
