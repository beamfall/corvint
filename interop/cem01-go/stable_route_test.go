package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Retired 0.x routes (CEM-V1-222): the cem/0.1 default verify path, verify-03 and ci are removed.
// Every invocation outside the cem/1.0 commands is refused; a map declaring a cem/0.x spec is
// refused with the closed code unsupported-spec, never verified and never an internal error.

// sealedStableMap returns the public stable positive map bytes, or skips outside the repository.
func sealedStableMap(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "repository-envelope-packet", "inherited", "fixtures", "maps", "sha1-positive-sealed.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("NOT_RUN: stable packet absent outside the Corvint repository: %v", err)
	}
	return b
}

func strOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func TestRetiredRouteRefusesEveryZeroXMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name string, body []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, tc := range map[string]struct {
		body   []byte
		code   string
		status int
	}{
		"cem/0.1":            {[]byte(`{"spec":"cem/0.1","baseRevision":"x"}`), "unsupported-spec", 1},
		"cem/0.2":            {[]byte(`{"spec":"cem/0.2"}`), "unsupported-spec", 1},
		"cem/0.3":            {[]byte(`{"spec":"cem/0.3","hunks":[]}`), "unsupported-spec", 1},
		"escaped 0.2":        {[]byte(`{"spec":"cem\/0.2"}`), "unsupported-spec", 1},
		"stable":             {sealedStableMap(t), "invocation", 2},
		"candidate":          {[]byte(`{"spec":"cem/1.0-experimental.1"}`), "invocation", 2},
		"duplicate spec":     {[]byte(`{"spec":"cem/0.1","spec":"cem/0.1"}`), "invocation", 2},
		"non-string spec":    {[]byte(`{"spec":1}`), "invocation", 2},
		"not an object":      {[]byte(`["cem/0.1"]`), "invocation", 2},
		"trailing data":      {[]byte(`{"spec":"cem/0.1"} {}`), "invocation", 2},
		"unpaired surrogate": {[]byte(`{"spec":"cem/0.1","x":"\ud800"}`), "invocation", 2},
	} {
		p := write(filepath.Base(name)+".json", tc.body)
		for _, argv := range [][]string{
			{"verify", "--repository", dir, "--map", p, "--patch", filepath.Join(dir, "absent.patch")},
			{"verify-03", "--repository", dir, "--map", p},
			{"ci", "--map", p},
		} {
			if code, status := retiredRoute(argv); code != tc.code || status != tc.status {
				t.Fatalf("CEM-V1-222 %s %v: got %s/%d, want %s/%d", name, argv[0], code, status, tc.code, tc.status)
			}
		}
	}
	for _, argv := range [][]string{nil, {"verify"}, {"ci", "--repository", dir}, {"--map"}, {"--map", filepath.Join(dir, "absent.json")}} {
		if code, status := retiredRoute(argv); code != "invocation" || status != 2 {
			t.Fatalf("CEM-V1-222 %v: got %s/%d, want invocation/2", argv, code, status)
		}
	}
	if refusalFix("unsupported-spec") != retiredFix || refusalFix("invocation") != invocationFix {
		t.Fatal("CEM-V1-222: only a cem/0.x map refusal may print the retired-map fix line")
	}
}
