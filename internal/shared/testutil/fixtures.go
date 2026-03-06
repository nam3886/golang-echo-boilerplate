package testutil

// UserFixture holds test data for creating a user in tests.
type UserFixture struct {
	Email    string
	Name     string
	Password string
	Role     string
}

// DefaultUserFixture returns a standard member user fixture.
func DefaultUserFixture() UserFixture {
	return UserFixture{
		Email:    "user@example.com",
		Name:     "Test User",
		Password: "password123",
		Role:     "member",
	}
}

// AdminUserFixture returns an admin user fixture.
func AdminUserFixture() UserFixture {
	return UserFixture{
		Email:    "admin@example.com",
		Name:     "Admin User",
		Password: "adminpass123",
		Role:     "admin",
	}
}

// ViewerUserFixture returns a read-only viewer user fixture.
func ViewerUserFixture() UserFixture {
	return UserFixture{
		Email:    "viewer@example.com",
		Name:     "Viewer User",
		Password: "viewerpass123",
		Role:     "viewer",
	}
}
