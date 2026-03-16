package testutil

// Ptr returns a pointer to the given value. Useful in tests for optional fields.
func Ptr[T any](v T) *T {
	return &v
}

// FakeArgon2Hash is a deterministic argon2id-formatted hash for use in tests
// that call domain constructors (NewUser). Satisfies the "$argon2id$" prefix check.
const FakeArgon2Hash = "$argon2id$v=19$test$hashed_pwd"
