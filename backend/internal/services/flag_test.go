package services

import "testing"

func TestResolveFlag(t *testing.T) {
	b := func(v bool) *bool { return &v }

	tests := []struct {
		name       string
		def        bool
		global     *bool
		org        *bool
		branch     *bool
		wantEnable bool
		wantSource string
	}{
		{"no overrides -> default false", false, nil, nil, nil, false, flagSourceDefault},
		{"no overrides -> default true", true, nil, nil, nil, true, flagSourceDefault},
		{"global beats default", false, b(true), nil, nil, true, flagSourceGlobalOverride},
		{"org beats global", false, b(false), b(true), nil, true, flagSourceOrgOverride},
		{"branch beats org and global", true, b(true), b(true), b(false), false, flagSourceBranchOverride},
		{"org false overrides global true", false, b(true), b(false), nil, false, flagSourceOrgOverride},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enabled, source := resolveFlag(tc.def, tc.global, tc.org, tc.branch)
			if enabled != tc.wantEnable {
				t.Errorf("enabled = %v, want %v", enabled, tc.wantEnable)
			}
			if source != tc.wantSource {
				t.Errorf("source = %q, want %q", source, tc.wantSource)
			}
		})
	}
}

func TestPtrFromMap(t *testing.T) {
	m := map[string]bool{"a": true, "b": false}
	if p := ptrFromMap(m, "a"); p == nil || *p != true {
		t.Errorf("expected *true for present key")
	}
	if p := ptrFromMap(m, "b"); p == nil || *p != false {
		t.Errorf("expected *false for present key")
	}
	if p := ptrFromMap(m, "missing"); p != nil {
		t.Errorf("expected nil for absent key")
	}
}
