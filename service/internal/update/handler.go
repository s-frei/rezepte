package update

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
)

const changelogBase = "https://getrezepte.app/changelog/v"

type latestRelease struct {
	Version      string     `json:"version" doc:"Version of the newest stable release"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty" doc:"When that release was published; absent when GitHub gives no date"`
	ChangelogURL string     `json:"changelogUrl" doc:"Release notes of that release"`
}

type statusBody struct {
	Current string         `json:"current" doc:"Version this instance runs"`
	State   State          `json:"state" enum:"current,available,unknown,off" doc:"current: up to date; available: a newer release exists; unknown: no successful check yet; off: update check disabled, demo mode or a development build"`
	Latest  *latestRelease `json:"latest,omitempty" required:"false" doc:"The newer release; present only when state is available"`
}

type statusOutput struct {
	Body statusBody
}

// Register installs get-update-status, for the instance owner only.
func Register(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-update-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/settings/update",
		Summary:     "Get whether a newer Rezepte release exists",
		Tags:        []string{"settings"},
		Security:    auth.SessionSecurity,
		Errors:      []int{403},
	}, func(ctx context.Context, _ *struct{}) (*statusOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok || !u.Role.IsSuperadmin() {
			return nil, huma.Error403Forbidden("superadmin role required")
		}
		st := svc.Status()
		out := &statusOutput{Body: statusBody{Current: st.Current, State: st.State}}
		if st.State == StateAvailable && st.Latest != nil {
			out.Body.Latest = &latestRelease{
				Version:      st.Latest.Version,
				ChangelogURL: changelogBase + st.Latest.Version + "/",
			}
			if !st.Latest.PublishedAt.IsZero() {
				out.Body.Latest.PublishedAt = &st.Latest.PublishedAt
			}
		}
		return out, nil
	})
}
