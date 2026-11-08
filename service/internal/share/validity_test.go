package share

import (
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-24 * time.Hour)
	startsAt5 := now.AddDate(0, 0, -5)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)
	days30 := 30
	createdFortyDaysAgo := now.AddDate(0, 0, -40)

	tests := []struct {
		name       string
		startsAt   time.Time
		expiresAt  *time.Time
		instanceOn bool
		allowed    bool
		maxDays    *int
		wantStatus Status
		wantExpiry *time.Time
		wantServe  bool
	}{
		{
			name:       "on, allowed, permanent",
			startsAt:   createdAt,
			expiresAt:  nil,
			instanceOn: true,
			allowed:    true,
			maxDays:    nil,
			wantStatus: StatusActive,
			wantExpiry: nil,
			wantServe:  true,
		},
		{
			name:       "instance off",
			startsAt:   createdAt,
			expiresAt:  nil,
			instanceOn: false,
			allowed:    true,
			maxDays:    nil,
			wantStatus: StatusPaused,
			wantExpiry: nil,
			wantServe:  false,
		},
		{
			name:       "creator withdrawn",
			startsAt:   createdAt,
			expiresAt:  nil,
			instanceOn: true,
			allowed:    false,
			maxDays:    nil,
			wantStatus: StatusPaused,
			wantExpiry: nil,
			wantServe:  false,
		},
		{
			name:       "own expiry past",
			startsAt:   createdAt,
			expiresAt:  &past,
			instanceOn: true,
			allowed:    true,
			maxDays:    nil,
			wantStatus: StatusActive,
			wantExpiry: &past,
			wantServe:  false,
		},
		{
			name:       "limited by a lowered maximum",
			startsAt:   createdFortyDaysAgo,
			expiresAt:  nil,
			instanceOn: true,
			allowed:    true,
			maxDays:    &days30,
			wantStatus: StatusLimited,
			wantExpiry: ptrTime(createdFortyDaysAgo.AddDate(0, 0, 30)),
			wantServe:  false,
		},
		{
			name:       "the maximum lifted again reverses the limit",
			startsAt:   createdFortyDaysAgo,
			expiresAt:  nil,
			instanceOn: true,
			allowed:    true,
			maxDays:    nil,
			wantStatus: StatusActive,
			wantExpiry: nil,
			wantServe:  true,
		},
		{
			name:       "the maximum counts from startsAt",
			startsAt:   startsAt5,
			expiresAt:  nil,
			instanceOn: true,
			allowed:    true,
			maxDays:    &days30,
			wantStatus: StatusActive,
			wantExpiry: ptrTime(startsAt5.AddDate(0, 0, 30)),
			wantServe:  true,
		},
		{
			name:       "effective expiry is the earlier of both",
			startsAt:   createdAt,
			expiresAt:  &future,
			instanceOn: true,
			allowed:    true,
			maxDays:    &days30,
			wantStatus: StatusActive,
			wantExpiry: ptrTime(earlier(future, createdAt.AddDate(0, 0, 30))),
			wantServe:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, expiry, serving := evaluate(tt.startsAt, tt.expiresAt, tt.instanceOn, tt.allowed, tt.maxDays, now)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if serving != tt.wantServe {
				t.Errorf("serving = %v, want %v", serving, tt.wantServe)
			}
			if !timeEqualPtr(expiry, tt.wantExpiry) {
				t.Errorf("effective expiry = %v, want %v", expiry, tt.wantExpiry)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func timeEqualPtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
