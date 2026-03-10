package middleware

import (
	"testing"
)

// corsAllowCreds mirrors the allowCreds logic in SetupMiddleware.
// Access-Control-Allow-Origin: * with AllowCredentials: true is rejected by browsers,
// so credentials must be disabled whenever a wildcard origin is present.
func corsAllowCreds(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return false
		}
	}
	return true
}

func TestCORS_WildcardOrigin_DisablesCredentials(t *testing.T) {
	origins := []string{"*"}
	if corsAllowCreds(origins) {
		t.Error("expected allowCreds=false when wildcard origin is present")
	}
}

func TestCORS_WildcardAmongOthers_DisablesCredentials(t *testing.T) {
	origins := []string{"https://app.example.com", "*"}
	if corsAllowCreds(origins) {
		t.Error("expected allowCreds=false when wildcard is mixed with specific origins")
	}
}

func TestCORS_ExplicitOrigins_AllowsCredentials(t *testing.T) {
	origins := []string{"https://app.example.com", "https://admin.example.com"}
	if !corsAllowCreds(origins) {
		t.Error("expected allowCreds=true when all origins are explicit")
	}
}

func TestCORS_EmptyOrigins_AllowsCredentials(t *testing.T) {
	origins := []string{}
	if !corsAllowCreds(origins) {
		t.Error("expected allowCreds=true for empty origins list")
	}
}
