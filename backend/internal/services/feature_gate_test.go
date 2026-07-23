package services

import "testing"

func TestGateDecision(t *testing.T) {
	cases := []struct {
		name          string
		hasCapability bool
		flagEnabled   bool
		want          bool
	}{
		{"both off", false, false, false},
		{"entitlement only", true, false, false},
		{"flag only", false, true, false},
		{"both on", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gateDecision(tc.hasCapability, tc.flagEnabled); got != tc.want {
				t.Fatalf("gateDecision(%v, %v) = %v, want %v", tc.hasCapability, tc.flagEnabled, got, tc.want)
			}
		})
	}
}

func TestPickFlagState(t *testing.T) {
	states := []FlagState{
		{Key: "loyalty", Enabled: true, Source: flagSourceBranchOverride},
		{Key: "staff_performance_analytics", Enabled: false, Source: flagSourceDefault},
	}

	got := pickFlagState(states, "loyalty")
	if !got.Enabled || got.Source != flagSourceBranchOverride {
		t.Fatalf("pickFlagState known key = %+v, want enabled branch_override", got)
	}

	// A key missing from the catalog must resolve disabled, never error.
	got = pickFlagState(states, "not_in_catalog")
	if got.Enabled || got.Source != flagSourceDefault {
		t.Fatalf("pickFlagState missing key = %+v, want disabled default", got)
	}
}
