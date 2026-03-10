//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

func setupRepo(t *testing.T) *PgUserRepository {
	t.Helper()
	pool := testutil.NewTestPostgres(t)
	testutil.RunMigrations(t, pool)
	return NewPgUserRepository(pool)
}

func createTestUser(t *testing.T, email string) *domain.User {
	t.Helper()
	user, err := domain.NewUser(email, "Test User", "hashed_pwd", domain.RoleMember)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user
}

func TestPgUserRepository_Create(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	user := createTestUser(t, "create@example.com")
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify by GetByID — the domain UUID should match
	got, err := repo.GetByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID() != user.ID() {
		t.Errorf("ID mismatch: got %s, want %s", got.ID(), user.ID())
	}
	if got.Email() != "create@example.com" {
		t.Errorf("Email mismatch: got %s, want create@example.com", got.Email())
	}
}

func TestPgUserRepository_Create_DuplicateEmail(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	user1 := createTestUser(t, "dup@example.com")
	if err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	user2 := createTestUser(t, "dup@example.com")
	err := repo.Create(ctx, user2)
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
	// errors.Is works here because DomainError.Is() compares by ErrorCode,
	// so wrapped or pointer-distinct DomainError values match the sentinel.
	if !errors.Is(err, domain.ErrEmailTaken()) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestPgUserRepository_GetByID_NotFound(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, domain.UserID("00000000-0000-0000-0000-000000000000"))
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !errors.Is(err, sharederr.ErrNotFound()) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPgUserRepository_SoftDelete(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	user := createTestUser(t, "delete@example.com")
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	deleted, err := repo.SoftDelete(ctx, user.ID())
	if err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if deleted.ID() != user.ID() {
		t.Errorf("SoftDelete returned wrong user: got %s, want %s", deleted.ID(), user.ID())
	}

	// GetByID should return not found after soft delete
	_, err = repo.GetByID(ctx, user.ID())
	if !errors.Is(err, sharederr.ErrNotFound()) {
		t.Errorf("expected ErrNotFound after soft delete, got %v", err)
	}
}

func TestPgUserRepository_List_Pagination(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	// Insert 3 users
	emails := []string{"list1@example.com", "list2@example.com", "list3@example.com"}
	for _, email := range emails {
		user := createTestUser(t, email)
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create %s: %v", email, err)
		}
	}

	// Page 1 with pageSize=2 → should get 2 users + total=3
	res1, err := repo.List(ctx, 1, 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res1.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(res1.Users))
	}
	if res1.Total != 3 {
		t.Errorf("expected total=3, got %d", res1.Total)
	}

	// Page 2 → should get 1 user + total=3
	res2, err := repo.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(res2.Users) != 1 {
		t.Errorf("expected 1 user on page 2, got %d", len(res2.Users))
	}
	if res2.Total != 3 {
		t.Errorf("expected total=3, got %d", res2.Total)
	}
}
