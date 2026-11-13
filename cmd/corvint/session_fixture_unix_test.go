//go:build unix

package main

// The session fixture lives under the wider unix constraint because
// darwin||linux tests and unix-tagged tests (inventory_readers_unix_test.go,
// runner_contract_unix_test.go) both use it; the BSD and Solaris cross-vet
// runs fail to type-check when it is narrower (V1-4850).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// sessionFakeRunners is a recording `go` and `make` on PATH: probes answer
// the fixture universe, `go test` reports each named package as passing
// (or, in sleep mode, signals readiness and sleeps), and `make` passes
// except for its rust-test target.
const sessionFakeGo = `#!/bin/sh
echo "go $*" >> "$SESSION_FAKE_LOG"
case "$1" in
env) echo go1.27.1; exit 0;;
list) echo example.test/m/a; echo example.test/m/b; exit 0;;
esac
if [ "$SESSION_FAKE_MODE" = sleep ]; then touch "$SESSION_FAKE_LOG.started"; sleep 60; fi
for arg in "$@"; do
  case "$arg" in example.test/*) echo "{\"Action\":\"pass\",\"Package\":\"$arg\"}";; esac
done
`

const sessionFakeMake = `#!/bin/sh
echo "make $*" >> "$SESSION_FAKE_LOG"
[ "$1" = --version ] && echo "GNU Make 4.4"
[ "$1" = rust-test ] && exit 1
exit 0
`

// sessionDeclaration is the fixture's corvint-test-commands/0 file, with
// any extra language members spliced in.
func sessionDeclaration(languages string) string {
	return `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-p", "1", "-count=1"]}, "languages": {` + languages + `}}` + "\n"
}

// sessionFixture is a two-package module whose HEAD changes package a.
func sessionFixture(t *testing.T, mode string) (root, base, log string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.test/m\n\ngo 1.27\n",
		"a/a.go":                 "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go":            "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"b/b.go":                 "package b\n\nfunc B() int { return 2 }\n",
		"b/b_test.go":            "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
		sessiontest.CommandsPath: sessionDeclaration(""),
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\n\nfunc A() int { return 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a")
	bin := t.TempDir()
	for name, body := range map[string]string{"go": sessionFakeGo, "make": sessionFakeMake} {
		if err := execfile.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log = filepath.Join(t.TempDir(), "runners.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", log)
	t.Setenv("SESSION_FAKE_MODE", mode)
	return root, base, log
}
