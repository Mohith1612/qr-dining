package services

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	n, err := parseNumeric(s)
	if err != nil {
		t.Fatalf("parseNumeric(%q): %v", s, err)
	}
	return n
}

func TestComputeEarnPoints(t *testing.T) {
	cases := []struct {
		name       string
		amount     string
		rateAmount string
		ratePoints int64
		want       int64
	}{
		{"simple floor", "250.00", "100.00", 1, 2},
		{"exact boundary", "300.00", "100.00", 1, 3},
		{"just under boundary", "299.99", "100.00", 1, 2},
		{"below one point", "99.99", "100.00", 1, 0},
		{"zero amount", "0.00", "100.00", 1, 0},
		{"multi-point rate", "125.00", "50.00", 2, 5},
		{"fractional rate amount", "100.00", "33.33", 1, 3},
		{"large amount no float drift", "999999.99", "100.00", 1, 9999},
		{"zero rate points", "250.00", "100.00", 0, 0},
		{"negative rate points", "250.00", "100.00", -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeEarnPoints(mustNumeric(t, tc.amount), mustNumeric(t, tc.rateAmount), tc.ratePoints)
			if got != tc.want {
				t.Fatalf("computeEarnPoints(%s, %s, %d) = %d, want %d", tc.amount, tc.rateAmount, tc.ratePoints, got, tc.want)
			}
		})
	}
}

func TestComputeEarnPointsInvalidNumerics(t *testing.T) {
	valid := mustNumeric(t, "100.00")
	if got := computeEarnPoints(pgtype.Numeric{}, valid, 1); got != 0 {
		t.Fatalf("invalid amount should earn 0 points, got %d", got)
	}
	if got := computeEarnPoints(valid, pgtype.Numeric{}, 1); got != 0 {
		t.Fatalf("invalid rate amount should earn 0 points, got %d", got)
	}
}
