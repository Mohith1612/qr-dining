package services

import "testing"

func TestValidSubscriptionTransition(t *testing.T) {
	tests := []struct {
		action string
		from   string
		want   bool
	}{
		// activate is permitted from any status (and also creates when none exists)
		{"activate", SubStatusTrial, true},
		{"activate", SubStatusSuspended, true},
		{"activate", SubStatusCancelled, true},
		{"activate", SubStatusExpired, true},
		// suspend only from live-ish statuses
		{"suspend", SubStatusActive, true},
		{"suspend", SubStatusTrial, true},
		{"suspend", SubStatusPastDue, true},
		{"suspend", SubStatusCancelled, false},
		{"suspend", SubStatusExpired, false},
		{"suspend", SubStatusSuspended, false},
		// renew from active/past_due/expired
		{"renew", SubStatusActive, true},
		{"renew", SubStatusExpired, true},
		{"renew", SubStatusTrial, false},
		{"renew", SubStatusCancelled, false},
		// cancel from any non-cancelled
		{"cancel", SubStatusActive, true},
		{"cancel", SubStatusTrial, true},
		{"cancel", SubStatusSuspended, true},
		{"cancel", SubStatusCancelled, false},
		// extend_trial only from trial
		{"extend_trial", SubStatusTrial, true},
		{"extend_trial", SubStatusActive, false},
		// change_plan from any existing
		{"change_plan", SubStatusActive, true},
		{"change_plan", SubStatusTrial, true},
		// unknown action
		{"frobnicate", SubStatusActive, false},
	}
	for _, tt := range tests {
		if got := validSubscriptionTransition(tt.action, tt.from); got != tt.want {
			t.Errorf("validSubscriptionTransition(%q, %q) = %v, want %v", tt.action, tt.from, got, tt.want)
		}
	}
}

func TestMappedAssignmentStatus(t *testing.T) {
	tests := map[string]string{
		SubStatusTrial:     "trial",
		SubStatusActive:    "active",
		SubStatusPastDue:   "active",
		SubStatusSuspended: "suspended",
		SubStatusCancelled: "cancelled",
		SubStatusExpired:   "cancelled",
	}
	for in, want := range tests {
		if got := mappedAssignmentStatus(in); got != want {
			t.Errorf("mappedAssignmentStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidInvoiceTransition(t *testing.T) {
	tests := []struct {
		action string
		from   string
		want   bool
	}{
		{"issue", "draft", true},
		{"issue", "issued", false},
		{"issue", "paid", false},
		{"mark_paid", "draft", true},
		{"mark_paid", "issued", true},
		{"mark_paid", "paid", false},
		{"mark_paid", "cancelled", false},
		{"cancel", "draft", true},
		{"cancel", "issued", true},
		{"cancel", "paid", false},
		{"cancel", "cancelled", false},
	}
	for _, tt := range tests {
		if got := validInvoiceTransition(tt.action, tt.from); got != tt.want {
			t.Errorf("validInvoiceTransition(%q, %q) = %v, want %v", tt.action, tt.from, got, tt.want)
		}
	}
}

func TestFormatInvoiceNumber(t *testing.T) {
	if got := formatInvoiceNumber(2026, 42); got != "INV-2026-000042" {
		t.Errorf("formatInvoiceNumber(2026, 42) = %q, want INV-2026-000042", got)
	}
	if got := formatInvoiceNumber(2026, 1234567); got != "INV-2026-1234567" {
		t.Errorf("formatInvoiceNumber(2026, 1234567) = %q, want INV-2026-1234567", got)
	}
}

func TestParseNumeric(t *testing.T) {
	if _, err := parseNumeric("1499.00"); err != nil {
		t.Errorf("parseNumeric(1499.00) unexpected error: %v", err)
	}
	if _, err := parseNumeric("not-a-number"); err == nil {
		t.Error("parseNumeric(not-a-number) expected error, got nil")
	}
}
