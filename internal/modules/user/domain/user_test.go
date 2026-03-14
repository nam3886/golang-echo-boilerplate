// NOTE: errors.Is() on DomainError matches by Code+Key when both errors carry a Key.
// For example, errors.Is(ErrInvalidEmail(), ErrNameRequired()) returns FALSE because
// both have distinct Keys ("user.invalid_email" vs "user.name_required").
// Code-only matching (returns true for same code, any key) only applies when the target
// has no Key set. Use testutil.AssertDomainError() to assert specific error types in tests.
package domain

import (
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
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
	testutil.AssertDomainError(t, err, "invalid email format")
}

func TestNewUser_InvalidName(t *testing.T) {
	_, err := NewUser("test@example.com", "", "hashed_pwd", RoleMember)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	testutil.AssertDomainError(t, err, "name is required")
}

func TestNewUser_NameTooLong(t *testing.T) {
	longName := string(make([]byte, 256))
	_, err := NewUser("test@example.com", longName, "hashed_pwd", RoleMember)
	if err == nil {
		t.Fatal("expected error for name exceeding 255 chars")
	}
	testutil.AssertDomainError(t, err, "name must be 255 characters or less")
}

func TestUser_ChangeName_TooLong(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test", "hashed_pwd", RoleMember)
	longName := string(make([]byte, 256))
	err := user.ChangeName(longName)
	if err == nil {
		t.Fatal("expected error for name exceeding 255 chars")
	}
	testutil.AssertDomainError(t, err, "name must be 255 characters or less")
}

func TestNewUser_InvalidRole(t *testing.T) {
	_, err := NewUser("test@example.com", "Test User", "hashed_pwd", Role("superadmin"))
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	testutil.AssertDomainError(t, err, "invalid role")
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
	testutil.AssertDomainError(t, err, "name is required")
}

func TestUser_ChangeName_NoOp(t *testing.T) {
	user, _ := NewUser("test@example.com", "Same", "hashed_pwd", RoleMember)
	old := user.UpdatedAt()
	if err := user.ChangeName("Same"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.UpdatedAt() != old {
		t.Error("no-op ChangeName should not update timestamp")
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
	testutil.AssertDomainError(t, err, "invalid role")
}

func TestUser_ChangeRole_NoOp(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test", "hashed_pwd", RoleMember)
	old := user.UpdatedAt()
	if err := user.ChangeRole(RoleMember); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.UpdatedAt() != old {
		t.Error("no-op ChangeRole should not update timestamp")
	}
}

func TestUser_ChangeEmail(t *testing.T) {
	user, _ := NewUser("old@example.com", "Test User", "hashed_pwd", RoleMember)
	if err := user.ChangeEmail("new@example.com"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Email() != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", user.Email())
	}
}

func TestUser_ChangeEmail_Invalid(t *testing.T) {
	user, _ := NewUser("old@example.com", "Test User", "hashed_pwd", RoleMember)
	err := user.ChangeEmail("not-an-email")
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
	testutil.AssertDomainError(t, err, "invalid email format")
}

func TestUser_ChangeEmail_NoOp(t *testing.T) {
	user, _ := NewUser("test@example.com", "Test", "hashed_pwd", RoleMember)
	old := user.UpdatedAt()
	if err := user.ChangeEmail("test@example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.UpdatedAt() != old {
		t.Error("no-op ChangeEmail should not update timestamp")
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
