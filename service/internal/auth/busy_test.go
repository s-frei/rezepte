package auth_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/user"
)

func TestBusyErrorAnswersAFullArgonQueueWith503(t *testing.T) {
	mapped := auth.BusyError(fmt.Errorf("verify password: %w", user.ErrBusy))
	var se huma.StatusError
	if !errors.As(mapped, &se) || se.GetStatus() != http.StatusServiceUnavailable {
		t.Fatalf("mapped = %v, want a 503", mapped)
	}
	var withHeaders huma.HeadersError
	if !errors.As(mapped, &withHeaders) || withHeaders.GetHeaders().Get("Retry-After") != "1" {
		t.Fatalf("mapped = %v, want Retry-After: 1", mapped)
	}
}

func TestBusyErrorLeavesOtherErrorsAlone(t *testing.T) {
	for _, err := range []error{nil, user.ErrInvalidCredentials, errors.New("boom")} {
		if mapped := auth.BusyError(err); mapped != nil {
			t.Errorf("BusyError(%v) = %v, want nil", err, mapped)
		}
	}
}
