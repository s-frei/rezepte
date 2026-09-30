package transfer

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

// Without a caller in the context spoolBody refuses before reading a byte,
// even when mounted where no auth middleware ran.
func TestSpoolBodyFailsClosed(t *testing.T) {
	_, api := humatest.New(t)
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/spool",
		Middlewares: huma.Middlewares{spoolBody(api, t.TempDir())},
	}, func(context.Context, *struct{}) (*struct{}, error) {
		t.Fatal("handler reached without a caller")
		return nil, nil
	})
	if rec := api.Post("/spool", strings.NewReader("PK")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no caller: %d", rec.Code)
	}
}
