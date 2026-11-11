// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const head = "0123456789abcdef0123456789abcdef01234567"

func writeMap(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(Path)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNCMV1002MissingProofIsProposed(t *testing.T) {
	read, err := Parse([]byte(`{"profile":"corvint-data-map/0","entries":[
		{"path":"config/app.yml","tests":["go:internal/app::TestLoad"],"evidence":"reviewed read",
		 "accepted_by":"owner","accepted_at":"2026-11-11","pinned_commit":"` + head + `"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	everything := Env{
		TestExists: func(string) bool { return true },
		IsAncestor: func(string) (bool, error) { return true, nil },
		FaultCase: func(string) (FaultCase, bool) {
			return FaultCase{Path: "config/app.yml", Killed: true, Killers: []string{"go:internal/app::TestLoad"}}, true
		},
	}
	status, detail := Evaluate(read.Entries[0], everything)
	if status != StatusProposed || !strings.Contains(detail, "no proof") {
		t.Fatalf("status = %s (%s), want proposed for a missing proof", status, detail)
	}
	decision := Decide(Input{Dirty: []string{"config/app.yml"}, Unowned: []string{"config/app.yml"}, Map: read, Env: everything})
	line, full := decision.FullRun()
	if !full || line.Reason != ReasonUnmapped || line.Path != "config/app.yml" {
		t.Fatalf("FullRun = %+v %v, want non-code-unmapped config/app.yml", line, full)
	}
	if got := strings.Join(decision.Audit(), "\n"); !strings.Contains(got, "data-map config/app.yml proposed: no proof") {
		t.Fatalf("audit does not report the entry proposed:\n%s", got)
	}
}

func TestNCMV1007RefusesUnknownVersion(t *testing.T) {
	for _, profile := range []string{`"corvint-data-map/1"`, `"corvint-data-map"`, `""`} {
		_, err := Parse([]byte(`{"profile":` + profile + `,"entries":[]}`))
		if !IsInvalid(err) || !strings.HasPrefix(err.Error(), "data-map-invalid: ") {
			t.Fatalf("profile %s: err = %v, want data-map-invalid", profile, err)
		}
	}
	if _, err := Parse([]byte(`{"entries":[]}`)); !IsInvalid(err) {
		t.Fatalf("missing profile: err = %v, want data-map-invalid", err)
	}
	root := writeMap(t, `{"profile":"corvint-data-map/1","entries":[]}`)
	if _, err := Read(root); !IsInvalid(err) || !strings.Contains(err.Error(), "corvint-data-map/1") {
		t.Fatalf("Read: err = %v, want data-map-invalid naming the version", err)
	}
}

func TestNCMV1007RefusesUnknownMember(t *testing.T) {
	for name, body := range map[string]string{
		"top-level":     `{"profile":"corvint-data-map/0","entries":[],"extra":1}`,
		"entry":         `{"profile":"corvint-data-map/0","entries":[{"path":"a.json","tests":["t"],"evidence":"e","kind":"data"}]}`,
		"duplicate key": `{"profile":"corvint-data-map/0","profile":"corvint-data-map/0","entries":[]}`,
	} {
		if _, err := Parse([]byte(body)); !IsInvalid(err) {
			t.Fatalf("%s: err = %v, want data-map-invalid", name, err)
		}
	}
}

func TestNCMV1007RefusesMalformedOrOversized(t *testing.T) {
	entry := func(members string) string {
		return `{"profile":"corvint-data-map/0","entries":[{` + members + `}]}`
	}
	for name, body := range map[string]string{
		"not json":         `{"profile":`,
		"trailing data":    `{"profile":"corvint-data-map/0","entries":[]} {}`,
		"no entries":       `{"profile":"corvint-data-map/0"}`,
		"path and glob":    entry(`"path":"a","glob":"b/*","tests":["t"],"evidence":"e"`),
		"neither":          entry(`"tests":["t"],"evidence":"e"`),
		"unclean path":     entry(`"path":"../a","tests":["t"],"evidence":"e"`),
		"backslash":        entry(`"path":"a\\b","tests":["t"],"evidence":"e"`),
		"bad glob":         entry(`"glob":"a/[","tests":["t"],"evidence":"e"`),
		"no tests":         entry(`"path":"a","tests":[],"evidence":"e"`),
		"repeated test":    entry(`"path":"a","tests":["t","t"],"evidence":"e"`),
		"no evidence":      entry(`"path":"a","tests":["t"]`),
		"bad date":         entry(`"path":"a","tests":["t"],"evidence":"e","accepted_at":"yesterday"`),
		"short commit":     entry(`"path":"a","tests":["t"],"evidence":"e","pinned_commit":"abc123"`),
		"control in proof": entry(`"path":"a","tests":["t"],"evidence":"e","proof":"x\u0001"`),
		"repeated key": `{"profile":"corvint-data-map/0","entries":[{"path":"a","tests":["t"],"evidence":"e"},` +
			`{"path":"a","tests":["u"],"evidence":"e"}]}`,
	} {
		if _, err := Parse([]byte(body)); !IsInvalid(err) {
			t.Fatalf("%s: err = %v, want data-map-invalid", name, err)
		}
	}
	oversized := `{"profile":"corvint-data-map/0","entries":[],"pad":"` + strings.Repeat("x", MaxBytes) + `"}`
	if _, err := Read(writeMap(t, oversized)); !IsInvalid(err) || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("oversized: err = %v, want data-map-invalid larger than", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(Path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); !IsInvalid(err) || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory: err = %v, want data-map-invalid not a regular file", err)
	}
	read, err := Read(t.TempDir())
	if err != nil || read.Profile != Profile || len(read.Entries) != 0 {
		t.Fatalf("missing map: %+v %v, want an empty map", read, err)
	}
}

func TestNCMV1009RepeatedReadIsByteIdentical(t *testing.T) {
	body := `{"profile":"corvint-data-map/0","entries":[
		{"glob":"config/*.yml","tests":["go:internal/b::TestB","go:internal/a::TestA"],"evidence":"e","accepted_by":"owner",
		 "accepted_at":"2026-11-11T10:00:00Z","pinned_commit":"` + head + `","proof":"FIH-NC-0001"},
		{"path":"tests/fixtures/users.json","tests":["go:tests::TestUsers"],"evidence":"e"}]}`
	root := writeMap(t, body)
	before := snapshot(t, root)
	render := func() string {
		read, err := Read(root)
		if err != nil {
			t.Fatal(err)
		}
		decision := Decide(Input{
			Dirty:   []string{"tests/fixtures/users.json", "config/app.yml"},
			Unowned: []string{"config/app.yml", "tests/fixtures/users.json"},
			Map:     read,
			Env:     Env{TestExists: func(string) bool { return true }, IsAncestor: func(string) (bool, error) { return true, nil }},
		})
		return fmt.Sprint(decision.Audit())
	}
	first, second := render(), render()
	if first != second {
		t.Fatalf("two runs differ:\n%s\n%s", first, second)
	}
	if after := snapshot(t, root); !bytes.Equal(before, after) {
		t.Fatalf("reading and deciding changed the tree")
	}
}

// snapshot lists every file under root with its bytes.
func snapshot(t *testing.T, root string) []byte {
	t.Helper()
	var out bytes.Buffer
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out.WriteString(name + "\n")
		if entry.Type().IsRegular() {
			body, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			out.Write(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
