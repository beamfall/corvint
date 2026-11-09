// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// AFP-V0-3424 extends the existing ratchet to shell tests. No pre-existing
// baseline exemption can admit a new test. A current audited record scope
// admits it, while adding another reader to the scoped unit invalidates it.
func TestAddedRecordEntryReadScopeRatchet_AFPV3424(t *testing.T) {
	for _, language := range []string{"go", "shell"} {
		t.Run(language, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				name = filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example.test/m\n\ngo 1.27\n")
			write(recordPath, `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`)
			file, id, body := "reader/a_test.go", "go:example.test/m/reader", "package reader\nvar dir = \"../docs/build-log/\"\n" //corvint:fixture-path
			if language == "shell" {
				file, id, body = "reader_test.sh", "shell:reader_test.sh", "#!/bin/sh\ncat docs/build-log/*.md\n" //corvint:fixture-path
			}
			write(file, body)
			var stdout, stderr bytes.Buffer
			if code := run(root, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "declare no read scope") {
				t.Fatalf("new literal reader passed ratchet: code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			graph, err := affected.BuildPartial(context.Background(), root, golang.New(), shell.New())
			if err != nil {
				t.Fatal(err)
			}
			unit, ok := graph.Unit(id)
			if !ok {
				t.Fatal("fixture reader missing")
			}
			digest, err := affected.RecordEntrySourceDigest(root, unit)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"profile": affected.RecordEntryScopesProfile, "units": map[string]affected.RecordEntryScope{
				id: {SourceSHA256: digest, Reads: []string{affected.BuildLogDirectory}, Reason: "fixture lists every entry"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			write(affected.RecordEntryScopesPath, string(data))
			stdout.Reset()
			stderr.Reset()
			if code := run(root, &stdout, &stderr); code != 0 {
				t.Fatalf("declared reader refused: %d %s", code, &stderr)
			}
			write(file, body+"\n")
			stdout.Reset()
			stderr.Reset()
			if code := run(root, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "stale or unknown record-entry read scope") {
				t.Fatalf("changed source binding admitted: %d %s", code, &stderr)
			}
		})
	}
}

// AFP-V0-2747, AFP-V0-3424: an addition-only scope filters existing reader
// matches; it cannot make an ancestor-only walker selectable for new entries.
func TestRecordEntryScopeCannotAdmitWalker_AFPV3424(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reads []string
	}{
		{"directory", []string{"docs/build-log/"}},      //corvint:fixture-path
		{"exact file", []string{"docs/build-log/x.md"}}, //corvint:fixture-path
		{"empty", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				name = filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example.test/m\n\ngo 1.27\n")
			write(recordPath, `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`)
			write("walker/w_test.go", `package walker
import (
	"io/fs"
	"path/filepath"
	"testing"
)
func TestWalk(t *testing.T) {
	if err := filepath.WalkDir("../docs", func(string, fs.DirEntry, error) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
`) //corvint:fixture-path
			write("docs/build-log/x.md", "entry\n") //corvint:fixture-path
			graph, err := affected.BuildPartial(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			const id = "go:example.test/m/walker"
			readers, walkers := graph.RecordReaders(affected.BuildLogDirectory)
			if slices.Contains(readers, id) || !slices.Contains(walkers, id) {
				t.Fatalf("fixture must reach ancestor-only walker classification: readers=%v walkers=%v", readers, walkers)
			}
			unit, ok := graph.Unit(id)
			if !ok {
				t.Fatal("fixture walker missing")
			}
			digest, err := affected.RecordEntrySourceDigest(root, unit)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"profile": affected.RecordEntryScopesProfile, "units": map[string]affected.RecordEntryScope{
				id: {SourceSHA256: digest, Reads: tc.reads, Reason: "walks ../docs"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			write(affected.RecordEntryScopesPath, string(data))
			scopes, err := affected.ReadRecordEntryScopes(root)
			if err != nil || scopes[id].SourceSHA256 != digest {
				t.Fatalf("fixture scope must be valid and current: scopes=%v err=%v", scopes, err)
			}
			var stdout, stderr bytes.Buffer
			if code := run(root, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "[walker] name docs/build-log/'s parent directory") {
				t.Fatalf("scoped walker escaped ratchet: code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
		})
	}
}
