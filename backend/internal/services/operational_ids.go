package services

import (
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func branchBusinessDate(branch sqlc.Branch, now time.Time) (time.Time, pgtype.Date) {
	loc, err := time.LoadLocation(branch.Timezone)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return local, pgtype.Date{Time: day, Valid: true}
}

func sessionReference(branchCode string, local time.Time, seq int32) string {
	return fmt.Sprintf("%s-S-%s-%03d", branchCode, local.Format("20060102"), seq)
}

func paymentReference(branchCode string, local time.Time, seq int32) string {
	return fmt.Sprintf("%s-PAY-%s-%04d", branchCode, local.Format("20060102"), seq)
}
