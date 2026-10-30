package affected_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const graphCacheIdentity = "sha256:test-program"

// graphCacheFixture copies the multi-language fixture and adds a small Rails
// tree, so a cached graph carries every plugin's units, edges and frontier.
func graphCacheFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the graph cache pins change times only on darwin and linux (AFP-V0-392)")
	}
	root := t.TempDir()
	source := fixtureRoot(t)
	err := filepath.WalkDir(source, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(source, current)
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(root, relative), 0o755)
		}
		body, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, relative), body, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCacheFixture(t, root, map[string]string{
		"Gemfile":                          "gem 'rails'\n",
		"config/application.rb":            "require 'rails/all'\nmodule App\n  class Application < Rails::Application\n  end\nend\n",
		"app/models/widget.rb":             "class Widget < ApplicationRecord\nend\n",
		"app/models/application_record.rb": "class ApplicationRecord < ActiveRecord::Base\nend\n",
		"spec/rails_helper.rb":             "require 'spec_helper'\n",
		"spec/spec_helper.rb":              "RSpec.configure do |config|\nend\n",
		"spec/models/widget_spec.rb":       "require 'rails_helper'\n\nRSpec.describe Widget do\n  it 'exists' do\n    expect(Widget.new).to be_truthy\n  end\nend\n",
		"tools/report.py":                  "import os\n\nprint(open('data/report.txt').read())\n",
		"tools/test_report.py":             "from tools import report\n",
		// The walk skips .corvint/, so only the absent-path stamp of a
		// declaration inside it can see one appear.
		".corvint/keep": "",
	})
	return root
}

func writeCacheFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, body := range files {
		name := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// pinnable lets an entry be written over files the test just created.
func pinnable(t *testing.T) {
	t.Helper()
	saved := affected.GraphCacheRacyWindow
	affected.GraphCacheRacyWindow = 0
	t.Cleanup(func() { affected.GraphCacheRacyWindow = saved })
}

// privateCacheDir returns a new cache directory only this user may write.
// t.TempDir creates it with mode 0777 less the umask, which a group-writable
// umask such as 0002 leaves group-writable, and the cache refuses that.
func privateCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func buildCached(t *testing.T, root, dir, identity string) (*affected.Graph, affected.CacheOutcome) {
	t.Helper()
	graph, outcome, err := affected.BuildCached(context.Background(), root, dir, identity, allLanguages()...)
	if err != nil {
		t.Fatalf("BuildCached: %v", err)
	}
	return graph, outcome
}

// assertSameGraph requires the cached graph to be the fresh one exactly: the
// canonical body, every unit with its nil lists, and the plans over several
// dirty sets (AFP-V0-393).
func assertSameGraph(t *testing.T, root string, got *affected.Graph) {
	t.Helper()
	want, err := affected.Build(context.Background(), root, allLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	wantBody, _ := want.Canonical()
	gotBody, _ := got.Canonical()
	if want.Digest() != got.Digest() || !bytes.Equal(wantBody, gotBody) {
		t.Fatalf("cached graph %s differs from the fresh graph %s", got.Digest(), want.Digest())
	}
	for _, id := range want.UnitIDs() {
		wantUnit, _ := want.Unit(id)
		gotUnit, _ := got.Unit(id)
		if !reflect.DeepEqual(wantUnit, gotUnit) {
			t.Fatalf("unit %s: cached %#v, fresh %#v", id, gotUnit, wantUnit)
		}
	}
	for _, dirty := range [][]string{nil, {"core/core.go"}, {"app/models/widget.rb"}, {"data/report.txt"}, {"README.unowned"}, {"web/core.mjs", "spec/spec_helper.rb"}} {
		if wantPlan, gotPlan := affected.Select(want, dirty), affected.Select(got, dirty); !reflect.DeepEqual(wantPlan, gotPlan) {
			t.Fatalf("plan over %v differs:\ncached %#v\nfresh  %#v", dirty, gotPlan, wantPlan)
		}
	}
}

// AFP-V0-391/393: a second build over an unchanged tree restores the verified
// entry, and the restored graph is the fresh graph exactly.
func TestAFPV0391CachedGraphIsRestoredExactly(t *testing.T) {
	pinnable(t)
	root := graphCacheFixture(t)
	dir := filepath.Join(t.TempDir(), "cache")
	graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if outcome.Hit || outcome.Reason != "no entry" || !outcome.Stored {
		t.Fatalf("first build must miss with no entry and store one: %+v", outcome)
	}
	assertSameGraph(t, root, graph)
	graph, outcome = buildCached(t, root, dir, graphCacheIdentity)
	if !outcome.Hit {
		t.Fatalf("second build over an unchanged tree must hit: %+v", outcome)
	}
	assertSameGraph(t, root, graph)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the cache directory must hold exactly one entry: %v %v", entries, err)
	}
	if info, _ := entries[0].Info(); info.Mode().Perm() != 0o600 {
		t.Fatalf("entry mode %v, want 0600", info.Mode().Perm())
	}
	// A new program identity replaces the entry rather than adding one.
	if _, outcome = buildCached(t, root, dir, "sha256:other-program"); outcome.Hit || outcome.Reason != "entry key mismatch" || !outcome.Stored {
		t.Fatalf("another program identity must miss and replace the entry: %+v", outcome)
	}
	if entries, _ = os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("a new identity must replace the entry: %v", entries)
	}
}

// AFP-V0-392: every change to what the build observed, and every damaged
// entry, falls back to a full build whose graph is the fresh one.
func TestAFPV0392CacheFailsClosedOnAnyMismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, root, entry string)
		reason string
	}{
		{"edited source of the same size", func(t *testing.T, root, _ string) {
			writeCacheFixture(t, root, map[string]string{"app/models/widget.rb": "class Widget < ApplicationRecorX\nend\n"})
		}, "path changed: app/models/widget.rb"},
		{"added source file", func(t *testing.T, root, _ string) {
			writeCacheFixture(t, root, map[string]string{"app/models/gadget.rb": "class Gadget\nend\n"})
		}, "path changed: app/models"},
		{"removed source file", func(t *testing.T, root, _ string) {
			if err := os.Remove(filepath.Join(root, "tools", "report.py")); err != nil {
				t.Fatal(err)
			}
		}, "path changed: tools"},
		{"file replaced by a symbolic link", func(t *testing.T, root, _ string) {
			name := filepath.Join(root, "web", "solo.mjs")
			if err := os.Remove(name); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("core.mjs", name); err != nil {
				t.Fatal(err)
			}
		}, "path changed: web"},
		{"skipped directory appears", func(t *testing.T, root, _ string) {
			writeCacheFixture(t, root, map[string]string{"app/build/generated.rb": "class Generated\nend\n"})
		}, "path changed: app"},
		{"declaration appears at a path read as absent", func(t *testing.T, root, _ string) {
			writeCacheFixture(t, root, map[string]string{affected.DocumentationPathsPath: `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`})
		}, "path changed: " + affected.DocumentationPathsPath},
		{"runtime readers declaration appears at a path read as absent", func(t *testing.T, root, _ string) {
			writeCacheFixture(t, root, map[string]string{affected.RuntimeReadersPath: `{"profile":"corvint-runtime-readers/0","entries":[]}`})
		}, "path changed: " + affected.RuntimeReadersPath},
		{"corrupt entry byte", func(t *testing.T, _, entry string) {
			body, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			body[len(body)/2] ^= 0x20
			if err := os.WriteFile(entry, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "entry checksum or profile mismatch"},
		{"truncated entry", func(t *testing.T, _, entry string) {
			if err := os.Truncate(entry, 40); err != nil {
				t.Fatal(err)
			}
		}, "entry checksum or profile mismatch"},
		{"rechecksummed entry with a forged digest", func(t *testing.T, _, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return strings.Replace(body, `"digest":"affected-graph:sha256:`, `"digest":"affected-graph:sha256:0`, 1)
			})
		}, "entry graph digest mismatch"},
		{"rechecksummed entry with a dropped unit", func(t *testing.T, _, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return strings.Replace(body, `"id":"ruby:source:app/models/widget"`, `"id":"ruby:source:app/models/widget2"`, 1)
			})
		}, "entry graph digest mismatch"},
		{"rechecksummed entry for another root", func(t *testing.T, _, entry string) {
			rewriteEntry(t, entry, func(body string) string { return strings.Replace(body, `"root":"`, `"root":"/elsewhere`, 1) })
		}, "entry key mismatch"},
		{"rechecksummed entry with every stamp removed, then a source edited", func(t *testing.T, root, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return regexp.MustCompile(`"stamps":\[.*\]\}$`).ReplaceAllString(body, `"stamps":[]}`)
			})
			writeCacheFixture(t, root, map[string]string{"app/models/widget.rb": "class Widget < ApplicationRecorX\nend\n"})
		}, "entry stamps incomplete"},
		{"rechecksummed entry with a source stamp removed, then that source edited", func(t *testing.T, root, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return regexp.MustCompile(`\{"path":"app/models/widget\.rb","stamp":\{[^}]*\}\},`).ReplaceAllString(body, "")
			})
			writeCacheFixture(t, root, map[string]string{"app/models/widget.rb": "class Widget < ApplicationRecorX\nend\n"})
		}, "entry stamps incomplete"},
		{"rechecksummed entry with a directory stamp removed", func(t *testing.T, _, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return regexp.MustCompile(`\{"path":"app/models","stamp":\{[^}]*\}\},`).ReplaceAllString(body, "")
			})
		}, "entry stamps incomplete"},
		{"rechecksummed entry with stamps out of order", func(t *testing.T, _, entry string) {
			rewriteEntry(t, entry, func(body string) string {
				return strings.Replace(body, `"stamps":[`, `"stamps":[{"path":"zz","stamp":{"present":false}},`, 1)
			})
		}, "entry stamps incomplete"},
		{"entry replaced by a directory", func(t *testing.T, _, entry string) {
			if err := os.Remove(entry); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(entry, 0o700); err != nil {
				t.Fatal(err)
			}
		}, "entry is not a readable regular file"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pinnable(t)
			root := graphCacheFixture(t)
			dir := privateCacheDir(t)
			if _, outcome := buildCached(t, root, dir, graphCacheIdentity); !outcome.Stored {
				t.Fatalf("seed entry not stored: %+v", outcome)
			}
			testCase.mutate(t, root, affected.GraphCacheEntryPath(dir, root, allLanguages()))
			graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
			if outcome.Hit || outcome.Reason != testCase.reason {
				t.Fatalf("outcome %+v, want a miss for %q", outcome, testCase.reason)
			}
			assertSameGraph(t, root, graph)
		})
	}
}

// rewriteEntry edits an entry body and recomputes its checksum, so only the
// checks behind the checksum can catch the edit.
func rewriteEntry(t *testing.T, entry string, edit func(string) string) {
	t.Helper()
	raw, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := bytes.Cut(raw, []byte("\n"))
	edited := edit(string(body))
	if edited == string(body) {
		t.Fatal("edit changed nothing")
	}
	sum := sha256.Sum256([]byte(edited))
	if err := os.WriteFile(entry, []byte(affected.GraphCacheProfile+" "+hex.EncodeToString(sum[:])+"\n"+edited), 0o600); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-392: an observation the stamps cannot pin writes no entry: a path
// changed inside the racy window, a plugin walk of a present subtree, or no
// program identity.
func TestAFPV0392UnpinnableObservationStoresNoEntry(t *testing.T) {
	t.Run("racy window", func(t *testing.T) {
		root := graphCacheFixture(t)
		dir := privateCacheDir(t)
		for range 2 {
			graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
			if outcome.Hit || outcome.Stored || !strings.HasPrefix(outcome.NotStored, "path changed too recently to pin: ") {
				t.Fatalf("files written just now must not be pinned: %+v", outcome)
			}
			assertSameGraph(t, root, graph)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("no entry may be written: %v", entries)
		}
	})
	t.Run("walked subtree", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		writeCacheFixture(t, root, map[string]string{".maestro/flow.yaml": "appId: x\n"})
		_, outcome := buildCached(t, root, privateCacheDir(t), graphCacheIdentity)
		if outcome.Stored || outcome.NotStored != "walked subtree .maestro" {
			t.Fatalf("a walked present subtree must not be pinned: %+v", outcome)
		}
	})
	t.Run("language outside the recorded source", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		languages := append(allLanguages(), unrecordedLanguage{})
		_, outcome, err := affected.BuildCached(context.Background(), root, privateCacheDir(t), graphCacheIdentity, languages...)
		if err != nil || outcome.Stored || outcome.NotStored != "language zz reads outside the recorded source" {
			t.Fatalf("a language reading outside the recorded source must not be pinned: %+v %v", outcome, err)
		}
	})
	for _, mode := range []os.FileMode{0o777, 0o775, 0o770} {
		t.Run(fmt.Sprintf("cache directory writable by others %#o", mode), func(t *testing.T) {
			pinnable(t)
			root := graphCacheFixture(t)
			dir := t.TempDir()
			if err := os.Chmod(dir, mode); err != nil {
				t.Fatal(err)
			}
			graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
			if outcome.Hit || outcome.Stored || outcome.Reason != "cache directory is not private to this user" || outcome.NotStored != outcome.Reason {
				t.Fatalf("a directory another user may write must neither hit nor store: %+v", outcome)
			}
			assertSameGraph(t, root, graph)
		})
	}
	t.Run("no program identity", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		dir := privateCacheDir(t)
		graph, outcome := buildCached(t, root, dir, "")
		if outcome.Hit || outcome.Stored || outcome.Reason != "no program identity" {
			t.Fatalf("an empty identity must neither hit nor store: %+v", outcome)
		}
		assertSameGraph(t, root, graph)
	})
}

// unrecordedLanguage observes through Units, whose reads no stamp records.
type unrecordedLanguage struct{}

func (unrecordedLanguage) Name() string     { return "zz" }
func (unrecordedLanguage) Owns(string) bool { return false }
func (unrecordedLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{}, nil
}

// AFP-V0-392: a read that fails for a reason its stamp does not pin (here an
// injected I/O error) writes no entry, so once the fault clears the next call
// builds the graph with the file instead of reusing the graph without it.
func TestAFPV0392TransientReadFailureStoresNoEntry(t *testing.T) {
	pinnable(t)
	root := graphCacheFixture(t)
	dir := privateCacheDir(t)
	failures := 0
	restore := affected.SetReadSource(func(root, relative string) ([]byte, error) {
		if relative == "app/models/widget.rb" && failures == 0 {
			failures++
			return nil, errors.New("injected input/output error")
		}
		return affected.ReadSource(root, relative)
	})
	graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
	restore()
	if failures != 1 || outcome.Stored || outcome.NotStored != "unreadable path app/models/widget.rb" {
		t.Fatalf("a transient read failure must not be pinned: failures=%d %+v", failures, outcome)
	}
	if _, ok := graph.Unit("ruby:source:app/models/widget"); ok {
		t.Fatal("the faulted build was expected to miss the unreadable unit")
	}
	graph, outcome = buildCached(t, root, dir, graphCacheIdentity)
	if outcome.Hit || outcome.Reason != "no entry" || !outcome.Stored {
		t.Fatalf("after recovery the call must build fresh: %+v", outcome)
	}
	assertSameGraph(t, root, graph)
}
