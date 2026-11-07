// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-2745 (V1-4933): the record-directory ratchet fails when a Go test
// package without a declared read scope reads a record directory by a path
// literal and the record does not list it, so a new literal reader cannot
// rejoin every change's selection unnoticed (the V1-1820 regression). A
// package whose read scope is declared is bounded by that declaration and is
// never listed. A listed package that no longer reads the directory is
// reported without failing; a malformed record exits 2. An absent record is
// the empty set.
func TestAFPV2745RecordDirectoryReadersRatchet(t *testing.T) {
	const unbounded = `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`
	const head = `{"profile":"corvint-record-readers/0","directories":`
	for name, c := range map[string]struct {
		record string // "" leaves the record absent
		code   int
		want   string
	}{
		"absent record":      {"", 1, "[lister] name the record directory docs/build-log/"},
		"listed reader":      {head + `{"docs/build-log/":{"lister":"reads every entry"}}}`, 0, ""},
		"stale entry":        {head + `{"docs/build-log/":{"gone":"was a reader","lister":"reads every entry"},".corvint/changes/":{}}}`, 0, "lists [gone] under docs/build-log/"},
		"other directory":    {head + `{".corvint/changes/":{"lister":"reads every entry"}}}`, 1, "[lister] name the record directory docs/build-log/"},
		"blank reason":       {head + `{"docs/build-log/":{"lister":" "}}}`, 2, "want exactly"},
		"unprobed directory": {head + `{"docs/":{"lister":"reads"}}}`, 2, "want exactly"},
		"unknown member":     {head + `{},"extra":1}`, 2, "want exactly"},
		"wrong profile":      {`{"profile":"corvint-record-readers/1","directories":{}}`, 2, "want exactly"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                         "module example.test/m\n\ngo 1.27\n",
				"lister/l.go":                    "package lister\n",
				"lister/l_test.go":               "package lister\n\nvar entries = \"../docs/build-log/\"\n",
				"scoped/s.go":                    "package scoped\n",
				"scoped/s_test.go":               "package scoped\n\nvar entries = \"../docs/build-log/\"\n",
				"docs/build-log/README.md":       "log\n",
				recordPath:                       unbounded,
				".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":["docs/build-log/"]}}`,
			}
			if c.record != "" {
				files[recordReadersPath] = c.record
			}
			for file, body := range files {
				path := filepath.Join(root, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := run(root, &stdout, &stderr)
			if code != c.code || !strings.Contains(stderr.String(), c.want) {
				t.Fatalf("code %d, want %d; stderr:\n%s", code, c.code, stderr.String())
			}
			if strings.Contains(stderr.String(), "[lister scoped]") || strings.Contains(stderr.String(), "[scoped]") {
				t.Fatalf("a package with a declared read scope is listed:\n%s", stderr.String())
			}
		})
	}
}
