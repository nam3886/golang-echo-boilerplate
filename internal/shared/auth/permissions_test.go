package auth

import (
	"testing"
)

func TestPermissionsForRole(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		wantNil   bool
		wantPerms []string
	}{
		{"admin gets wildcard", "admin", false, []string{"admin:*"}},
		{"member gets read+write", "member", false, []string{"user:read", "user:write"}},
		{"viewer gets read only", "viewer", false, []string{"user:read"}},
		{"unknown role returns nil", "hacker", true, nil},
		{"empty role returns nil", "", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PermissionsForRole(tt.role)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.wantPerms) {
				t.Fatalf("expected %d perms, got %d: %v", len(tt.wantPerms), len(got), got)
			}
			for i, p := range tt.wantPerms {
				if got[i] != p {
					t.Errorf("perm[%d]: expected %q, got %q", i, p, got[i])
				}
			}
		})
	}
}

func TestPermissionsForRole_ReturnsCopy(t *testing.T) {
	perms := PermissionsForRole("admin")
	if len(perms) == 0 {
		t.Fatal("expected non-empty permissions for admin")
	}
	original := perms[0]
	perms[0] = "mutated"

	perms2 := PermissionsForRole("admin")
	if len(perms2) == 0 {
		t.Fatal("expected non-empty permissions for admin on second call")
	}
	if perms2[0] != original {
		t.Errorf("mutation affected source: expected %q, got %q", original, perms2[0])
	}
}
