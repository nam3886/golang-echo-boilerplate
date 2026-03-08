package testutil

// Ptr returns a pointer to the given value. Useful in tests for optional fields.
func Ptr[T any](v T) *T {
	return &v
}
