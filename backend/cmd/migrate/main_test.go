package main

import (
	"fmt"
	"io/fs"
	"testing"

	"github.com/Mohith1612/qr-dining/migrations"
)

// The deploy pre-flight in deploy/vm/deploy.sh cross-checks the version list
// this binary reports against the migration files at the deployed commit. If
// sourceVersions ever stopped reading the embedded FS — or started skipping
// entries — the two would silently agree on a wrong answer, so pin the contract.
func TestSourceVersionsMatchesEmbeddedFS(t *testing.T) {
	got, err := sourceVersions()
	if err != nil {
		t.Fatalf("sourceVersions: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no migrations found in the embedded FS")
	}

	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("versions not strictly ascending at %d: %v", i, got)
		}
	}

	// Every reported version must have an up file that actually embedded.
	for _, v := range got {
		matches, err := fs.Glob(migrations.FS, fmt.Sprintf("%06d_*.up.sql", v))
		if err != nil {
			t.Fatalf("glob %d: %v", v, err)
		}
		if len(matches) != 1 {
			t.Fatalf("version %d: expected exactly one up file, got %v", v, matches)
		}
	}

	ups, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil {
		t.Fatalf("glob up files: %v", err)
	}
	if len(ups) != len(got) {
		t.Fatalf("reported %d versions but %d up files are embedded", len(got), len(ups))
	}
}

func TestJoinVersions(t *testing.T) {
	for _, tc := range []struct {
		in   []uint
		want string
	}{
		{nil, ""},
		{[]uint{40}, "40"},
		{[]uint{38, 39, 40}, "38,39,40"},
	} {
		if got := joinVersions(tc.in); got != tc.want {
			t.Errorf("joinVersions(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
