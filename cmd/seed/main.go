package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/gnha/gnha-services/internal/modules/user/adapters/postgres"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/gnha/gnha-services/internal/shared/database"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
)

type seedUser struct {
	email    string
	name     string
	password string
	role     domain.Role
}

var seedUsers = []seedUser{
	{email: "admin@example.com", name: "Admin User", password: "Admin@123456", role: domain.RoleAdmin},
	{email: "member@example.com", name: "Member User", password: "Member@123456", role: domain.RoleMember},
	{email: "viewer@example.com", name: "Viewer User", password: "Viewer@123456", role: domain.RoleViewer},
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	pool, err := database.NewPostgresPool(cfg)
	if err != nil {
		slog.Error("connecting to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := postgres.NewPgUserRepository(pool)
	hasher := auth.NewPasswordHasher()
	ctx := context.Background()

	slog.Info("starting database seed")

	for _, s := range seedUsers {
		if err := seedOne(ctx, repo, hasher, s); err != nil {
			slog.Error("seeding user", "email", s.email, "err", err)
			os.Exit(1)
		}
	}

	slog.Info("seed completed successfully")
}

func seedOne(ctx context.Context, repo domain.UserRepository, hasher auth.PasswordHasher, s seedUser) error {
	existing, err := repo.GetByEmail(ctx, s.email)
	if err != nil && !errors.Is(err, sharederr.ErrNotFound()) {
		return err
	}
	if existing != nil {
		slog.Info("user already exists, skipping", "email", s.email)
		return nil
	}

	hashed, err := hasher.Hash(s.password)
	if err != nil {
		return err
	}

	user, err := domain.NewUser(s.email, s.name, hashed, s.role)
	if err != nil {
		return err
	}

	if err := repo.Create(ctx, user); err != nil {
		return err
	}

	slog.Info("created user", "email", s.email, "role", s.role)
	return nil
}
