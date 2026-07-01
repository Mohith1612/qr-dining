package services

import "testing"

func bp(v bool) *bool   { return &v }
func lp(v int64) *int64 { return &v }

func TestMergeEntitlements(t *testing.T) {
	catalog := []catalogEntitlement{
		{Key: "analytics.basic", Kind: "capability"},
		{Key: "analytics.advanced", Kind: "capability"},
		{Key: "multi_branch", Kind: "capability"},
		{Key: "limit.branches", Kind: "limit"},
		{Key: "limit.tables", Kind: "limit"},
	}

	tests := []struct {
		name      string
		base      []baseEntitlement
		overrides []overrideEntitlement
		wantCaps  map[string]bool
		wantLims  map[string]int64
	}{
		{
			name:     "empty base and overrides: caps false, limits unlimited",
			wantCaps: map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": false},
			wantLims: map[string]int64{"limit.branches": -1, "limit.tables": -1},
		},
		{
			name: "plan base grants capability and finite limit",
			base: []baseEntitlement{
				{Key: "analytics.basic", Enabled: true},
				{Key: "limit.branches", Enabled: true, Limit: lp(3)},
			},
			wantCaps: map[string]bool{"analytics.basic": true, "analytics.advanced": false, "multi_branch": false},
			wantLims: map[string]int64{"limit.branches": 3, "limit.tables": -1},
		},
		{
			name:      "override enables a capability the plan disabled",
			base:      []baseEntitlement{{Key: "multi_branch", Enabled: false}},
			overrides: []overrideEntitlement{{Key: "multi_branch", Enabled: bp(true)}},
			wantCaps:  map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": true},
			wantLims:  map[string]int64{"limit.branches": -1, "limit.tables": -1},
		},
		{
			name:      "override disables a capability the plan enabled",
			base:      []baseEntitlement{{Key: "analytics.basic", Enabled: true}},
			overrides: []overrideEntitlement{{Key: "analytics.basic", Enabled: bp(false)}},
			wantCaps:  map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": false},
			wantLims:  map[string]int64{"limit.branches": -1, "limit.tables": -1},
		},
		{
			name:      "override raises a limit above the plan; nil override inherits",
			base:      []baseEntitlement{{Key: "limit.branches", Enabled: true, Limit: lp(3)}, {Key: "limit.tables", Enabled: true, Limit: lp(10)}},
			overrides: []overrideEntitlement{{Key: "limit.branches", Limit: lp(10)}, {Key: "limit.tables", Limit: nil}},
			wantCaps:  map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": false},
			wantLims:  map[string]int64{"limit.branches": 10, "limit.tables": 10},
		},
		{
			name:      "nil plan limit means unlimited",
			base:      []baseEntitlement{{Key: "limit.branches", Enabled: true, Limit: nil}},
			wantCaps:  map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": false},
			wantLims:  map[string]int64{"limit.branches": -1, "limit.tables": -1},
		},
		{
			name:      "unknown keys outside the catalog are ignored",
			base:      []baseEntitlement{{Key: "not.a.real.key", Enabled: true}},
			overrides: []overrideEntitlement{{Key: "another.bogus", Enabled: bp(true)}},
			wantCaps:  map[string]bool{"analytics.basic": false, "analytics.advanced": false, "multi_branch": false},
			wantLims:  map[string]int64{"limit.branches": -1, "limit.tables": -1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps, lims := mergeEntitlements(catalog, tc.base, tc.overrides)
			for k, want := range tc.wantCaps {
				if caps[k] != want {
					t.Errorf("capability %q = %v, want %v", k, caps[k], want)
				}
			}
			for k, want := range tc.wantLims {
				if lims[k] != want {
					t.Errorf("limit %q = %d, want %d", k, lims[k], want)
				}
			}
			if _, ok := caps["not.a.real.key"]; ok {
				t.Error("unknown key leaked into capabilities")
			}
		})
	}
}

func TestLimitFromInt(t *testing.T) {
	if limitFromInt(-1) != nil {
		t.Error("-1 should map to nil (unlimited)")
	}
	if v := limitFromInt(5); v == nil || *v != 5 {
		t.Errorf("5 should map to *5, got %v", v)
	}
	if v := limitFromInt(0); v == nil || *v != 0 {
		t.Errorf("0 should map to *0, got %v", v)
	}
}
