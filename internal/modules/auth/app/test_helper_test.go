package app

import (
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/testutil"
)

// assertPanics delegates to testutil.AssertPanics for backward compatibility.
func assertPanics(t *testing.T, label string, fn func()) {
	t.Helper()
	testutil.AssertPanics(t, label, fn)
}
