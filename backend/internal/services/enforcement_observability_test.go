package services

import (
	"testing"
	"time"
)

func TestClassifySubscription(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-48 * time.Hour)
	soon := now.Add(3 * 24 * time.Hour)
	later := now.Add(60 * 24 * time.Hour)

	tests := []struct {
		name     string
		status   string
		trial    *time.Time
		expires  *time.Time
		wantReason string
	}{
		{"suspended", SubStatusSuspended, nil, nil, "suspended"},
		{"past_due", SubStatusPastDue, nil, nil, "past_due"},
		{"expired status", SubStatusExpired, nil, nil, "expired"},
		{"cancelled is silent", SubStatusCancelled, nil, nil, ""},
		{"active healthy", SubStatusActive, nil, &later, ""},
		{"active lapsed", SubStatusActive, nil, &past, "expired"},
		{"trial healthy", SubStatusTrial, &later, nil, ""},
		{"trial ending soon", SubStatusTrial, &soon, nil, "trial_ending"},
		{"trial expired", SubStatusTrial, &past, nil, "trial_expired"},
		{"active no expiry", SubStatusActive, nil, nil, ""},
	}
	for _, tt := range tests {
		if got := classifySubscription(tt.status, tt.trial, tt.expires, now); got != tt.wantReason {
			t.Errorf("%s: classifySubscription = %q, want %q", tt.name, got, tt.wantReason)
		}
	}
}
