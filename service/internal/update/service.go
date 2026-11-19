package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FeedURL is GitHub's newest stable release of Rezepte.
const FeedURL = "https://api.github.com/repos/s-frei/rezepte/releases/latest"

// checkEvery is how often Start asks again; a variable so tests need not wait.
var checkEvery = 24 * time.Hour

// State says where the running version stands against the latest release.
type State string

// The values of State; the API sends them as strings.
const (
	StateCurrent   State = "current"
	StateAvailable State = "available"
	StateUnknown   State = "unknown"
	StateOff       State = "off"
)

// Release is a published stable release; Version carries no leading v.
type Release struct {
	Version     string
	PublishedAt time.Time
}

// Status is what the last check found. Latest is set only when State is available.
type Status struct {
	Current string
	State   State
	Latest  *Release
}

// Service caches the outcome of asking GitHub for the latest release.
type Service struct {
	current string
	feedURL string
	enabled bool
	client  *http.Client
	logger  *slog.Logger

	mu     sync.RWMutex
	status Status
}

// NewService is off when enabled is false or current does not parse (a dev
// build); otherwise its status is unknown until the first successful Check.
func NewService(current, feedURL string, enabled bool, client *http.Client, logger *slog.Logger) *Service {
	if _, err := parse(current); err != nil {
		enabled = false
	}
	state := StateUnknown
	if !enabled {
		state = StateOff
	}
	return &Service{
		current: current, feedURL: feedURL, enabled: enabled, client: client, logger: logger,
		status: Status{Current: current, State: state},
	}
}

// Status returns a copy of the cached status.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.status
	if st.Latest != nil {
		l := *st.Latest
		st.Latest = &l
	}
	return st
}

// Check fetches the latest release once and updates the cached status; a
// failure is logged and leaves the previous status.
func (s *Service) Check(ctx context.Context) {
	if !s.enabled {
		return
	}
	st, err := s.fetch(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "update: check failed", "err", err)
		return
	}
	s.mu.Lock()
	s.status = st
	s.mu.Unlock()
}

func (s *Service) fetch(ctx context.Context) (Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	if err != nil {
		return Status{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Rezepte")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.client.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("fetch latest release: status %d", resp.StatusCode)
	}
	var rel struct {
		TagName     string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
		Prerelease  bool      `json:"prerelease"`
		Draft       bool      `json:"draft"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return Status{}, fmt.Errorf("decode latest release: %w", err)
	}
	// releases/latest never names one; if it does, the answer is not trusted.
	if rel.Prerelease || rel.Draft {
		return Status{}, fmt.Errorf("latest release %q is a prerelease or draft", rel.TagName)
	}
	st := Status{Current: s.current, State: StateCurrent}
	v := strings.TrimPrefix(rel.TagName, "v")
	newer, err := newer(v, s.current)
	if err != nil {
		return Status{}, fmt.Errorf("compare release %q: %w", rel.TagName, err)
	}
	if newer {
		st.State = StateAvailable
		st.Latest = &Release{Version: v, PublishedAt: rel.PublishedAt}
	}
	return st, nil
}

// Start checks now and then every checkEvery, in its own goroutine, until ctx
// ends; the returned channel closes once it has stopped. When off it makes no
// request and returns a closed channel.
func (s *Service) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if !s.enabled {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		s.tick(ctx)
		ticker := time.NewTicker(checkEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
	return done
}

func (s *Service) tick(ctx context.Context) {
	// A panic would end the process, and with it every later check.
	defer func() {
		if r := recover(); r != nil {
			s.logger.ErrorContext(ctx, "update: check panicked", "panic", r)
		}
	}()
	s.Check(ctx)
}
