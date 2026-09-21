package auth_test

import (
	"testing"

	"github.com/s-frei/rezepte/service/internal/auth"
)

// TestProtectedPanicsWithNoScopes guards against an operation declaring
// auth.Protected() with no scope, which would make it reachable by any
// valid API token regardless of what it may do.
func TestProtectedPanicsWithNoScopes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Protected() with no scopes did not panic")
		}
	}()
	auth.Protected()
}
