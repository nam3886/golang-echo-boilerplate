package app

import "testing"

// assertPanics verifies that fn panics. Used across multiple test files in this package.
func assertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", label)
		}
	}()
	fn()
}
