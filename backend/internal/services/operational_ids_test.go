package services

import (
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
)

func TestOperationalReferencesUseBranchLocalBusinessDate(t *testing.T) {
	branch := sqlc.Branch{BranchCode: "BLR-INDIRANAGAR", Timezone: "Asia/Kolkata"}
	now := time.Date(2026, 5, 21, 20, 45, 0, 0, time.UTC)

	local, businessDate := branchBusinessDate(branch, now)

	if got := local.Format("2006-01-02"); got != "2026-05-22" {
		t.Fatalf("local date = %s, want 2026-05-22", got)
	}
	if got := businessDate.Time.Format("2006-01-02"); got != "2026-05-22" {
		t.Fatalf("business date = %s, want 2026-05-22", got)
	}
	if got := sessionReference(branch.BranchCode, local, 18); got != "BLR-INDIRANAGAR-S-20260522-018" {
		t.Fatalf("session reference = %s", got)
	}
	if got := paymentReference(branch.BranchCode, local, 31); got != "BLR-INDIRANAGAR-PAY-20260522-0031" {
		t.Fatalf("payment reference = %s", got)
	}
}

func TestOperationalReferencesFallbackToUTCForInvalidTimezone(t *testing.T) {
	branch := sqlc.Branch{BranchCode: "NYC-MIDTOWN", Timezone: "bad-zone"}
	now := time.Date(2026, 5, 21, 20, 45, 0, 0, time.UTC)

	local, businessDate := branchBusinessDate(branch, now)

	if got := local.Location().String(); got != "UTC" {
		t.Fatalf("location = %s, want UTC", got)
	}
	if got := businessDate.Time.Format("2006-01-02"); got != "2026-05-21" {
		t.Fatalf("business date = %s, want 2026-05-21", got)
	}
}
