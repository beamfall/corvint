// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

const otherPkg = "github.com/Beamfall/corvint/internal/lrfrepo"

// tierModule is a go list stream of module example.com/m: b imports a, c test-imports b, d
// external-test-imports c and e imports d; the root package embeds VERSION and b embeds
// static/x.html.
var tierModule = []doc{
	{"ImportPath": "example.com/m", "EmbedFiles": []any{"VERSION"}},
	{"ImportPath": "example.com/m/a", "Imports": []any{"fmt"}},
	{"ImportPath": "example.com/m/b", "Imports": []any{"example.com/m/a"}, "EmbedFiles": []any{"static/x.html"}},
	{"ImportPath": "example.com/m/c", "TestImports": []any{"example.com/m/b"}},
	{"ImportPath": "example.com/m/d", "XTestImports": []any{"example.com/m/c"}},
	{"ImportPath": "example.com/m/e", "Imports": []any{"example.com/m/d"}},
}

// tiersOf runs tiers.jq over CHANGED (one path per line) and the go list stream PKGS, with no
// documentation declaration.
func tiersOf(t *testing.T, changed string, pkgs []doc) doc {
	t.Helper()
	return tiersDeclared(t, changed, nil, pkgs)
}

// tiersDeclared is tiersOf with DOCUMENTATION, when it is not nil, passed as the raw bytes of
// `.corvint/documentation-paths.json` (AFP-V0-520).
func tiersDeclared(t *testing.T, changed string, documentation []byte, pkgs []doc) doc {
	t.Helper()
	dir := t.TempDir()
	var stream []byte
	for _, p := range pkgs {
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		stream = append(append(stream, b...), '\n')
	}
	writeFile(t, filepath.Join(dir, "changed"), []byte(changed))
	writeFile(t, filepath.Join(dir, "packages.json"), stream)
	args := []string{"-n", "-c", "--rawfile", "changed", filepath.Join(dir, "changed"),
		"--slurpfile", "pkgs", filepath.Join(dir, "packages.json"), "--arg", "module", "example.com/m"}
	if documentation != nil {
		writeFile(t, filepath.Join(dir, "documentation"), documentation)
		args = append(args, "--rawfile", "documentation", filepath.Join(dir, "documentation"))
	}
	var d doc
	if err := json.Unmarshal([]byte(run(t, nil, "jq", append(args, "-f", script(t, "tiers.jq"))...)), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAFPV0480Tiers(t *testing.T) {
	m := func(dirs ...string) []any {
		var out []any
		for _, d := range dirs {
			if d == "" {
				out = append(out, "example.com/m")
			} else {
				out = append(out, "example.com/m/"+d)
			}
		}
		return out
	}
	for _, c := range []struct {
		changed          string
		changed1, c2, d3 []any
	}{
		{"a/x.go\n", m("a"), m("a", "b"), m("a", "b", "c")},
		{"c/c_test.go\n", m("c"), m("c", "d"), m("c", "d", "e")},
		{"main.go\na/y.go\n", m("", "a"), m("", "a", "b"), m("", "a", "b", "c")},
	} {
		d := tiersOf(t, c.changed, tierModule)
		tr := sub(d, "tiers")
		if !reflect.DeepEqual(tr["changed"], c.changed1) || !reflect.DeepEqual(tr["hop1"], c.c2) ||
			!reflect.DeepEqual(tr["hop2"], c.d3) || len(strs(tr["full"])) != 0 || num(tr["universe"]) != 6 ||
			len(strs(d["tierIncomplete"])) != 0 {
			t.Fatalf("tiers of %q = %v", c.changed, d)
		}
	}
	// A change no package graph bounds makes every tier FULL with the reason, never an empty list.
	for changed, reason := range map[string]string{
		"go.mod\na/x.go\n":           "a module file changed: go.mod",
		"sub/go.sum\n":               "a module file changed: sub/go.sum",
		"a/testdata/in.go\n":         "a testdata file changed: a/testdata/in.go",
		"b/static/x.html\n":          "a file a package embeds changed: b/static/x.html",
		"VERSION\n":                  "a file a package embeds changed: VERSION",
		"docs/x.md\n":                "a non-Go file changed: docs/x.md",
		"z/x.go\n":                   "a Go file outside every listed package changed: z/x.go",
		".github/workflows/ci.yml\n": "the change touches .github/: .github/workflows/ci.yml",
	} {
		d := tiersOf(t, changed, tierModule)
		tr := sub(d, "tiers")
		if tr["changed"] != "FULL" || tr["hop1"] != "FULL" || tr["hop2"] != "FULL" ||
			fmt.Sprint(strs(tr["full"])) != fmt.Sprint([]string{reason}) {
			t.Fatalf("tiers of %q = %v, want FULL because %q", changed, d, reason)
		}
	}
	if d := tiersOf(t, "a/x.go\n", nil); fmt.Sprint(strs(d["tierIncomplete"])) != "[go list named no package of module example.com/m]" {
		t.Fatalf("tiers without packages = %v", d)
	}

	// tiers.sh end to end in a two-commit module: git diff plus a static go list, no network.
	testplatform.SkipPOSIXShell(t, "tiers.sh is a POSIX shell script run by sh")
	for _, tool := range []string{"git", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("tiers.sh needs %s: %v", tool, err)
		}
	}
	repo := t.TempDir()
	home := t.TempDir()
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "GOTOOLCHAIN=local", "GOCACHE=" + filepath.Join(home, "gocache"),
		"GOPATH=" + filepath.Join(home, "gopath"), "GIT_CONFIG_NOSYSTEM=1"}
	sh := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = repo, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(files map[string]string) string {
		for p, body := range files {
			writeFile(t, filepath.Join(repo, p), []byte(body))
		}
		sh("git", "add", "-A")
		sh("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-q", "-m", "x")
		return sh("git", "rev-parse", "HEAD")
	}
	sh("git", "init", "-q")
	base := commit(map[string]string{"go.mod": "module example.com/m\n\ngo 1.21\n",
		"a/a.go": "package a\n\nconst A = 1\n", "b/b.go": "package b\n\nimport \"example.com/m/a\"\n\nconst B = a.A\n",
		"c/c.go": "package c\n"})
	goChange := commit(map[string]string{"a/a.go": "package a\n\nconst A = 2\n"})
	modChange := commit(map[string]string{"go.mod": "module example.com/m\n\ngo 1.22\n"})
	// e embeds every .go file of its directory; deleting one is visible only at the base.
	embedAdd := commit(map[string]string{"e/e.go": "package e\n\nimport \"embed\"\n\n//go:embed *.go\nvar FS embed.FS\n",
		"e/b.go": "package e\n"})
	sh("git", "rm", "-q", "e/b.go")
	sh("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-q", "-m", "x")
	embedDelete := sh("git", "rev-parse", "HEAD")
	tiers := func(from, to string) doc {
		t.Helper()
		out := filepath.Join(t.TempDir(), "tiers.json")
		sh("sh", script(t, "tiers.sh"), from, to, out)
		d := readDoc(t, out)
		if ms, ok := d["millis"]; !ok || ms != nil && (num(ms) < 0 || num(ms) > 600000) {
			t.Fatalf("tiers.sh wrote no plausible millis: %v", d)
		}
		return d
	}
	if d := tiers(base, goChange); fmt.Sprint(sub(d, "tiers")["changed"]) != "[example.com/m/a]" ||
		fmt.Sprint(sub(d, "tiers")["hop1"]) != "[example.com/m/a example.com/m/b]" || len(strs(d["tierIncomplete"])) != 0 {
		t.Fatalf("tiers.sh of a Go change = %v", d)
	}
	if d := tiers(goChange, modChange); sub(d, "tiers")["hop2"] != "FULL" ||
		fmt.Sprint(strs(sub(d, "tiers")["full"])) != "[a module file changed: go.mod]" {
		t.Fatalf("tiers.sh of a go.mod change = %v", d)
	}
	if d := tiers(embedAdd, embedDelete); sub(d, "tiers")["changed"] != "FULL" ||
		fmt.Sprint(strs(sub(d, "tiers")["full"])) != "[a deleted file may have been embedded: e/b.go]" {
		t.Fatalf("tiers.sh of a deleted embedded file = %v", d)
	}
	if d := tiers(strings.Repeat("0", 40), modChange); d["tiers"] != nil ||
		len(strs(d["tierIncomplete"])) != 1 || !strings.HasPrefix(strs(d["tierIncomplete"])[0], "git diff ") || d["millis"] != nil {
		t.Fatalf("tiers.sh of an unknown base = %v", d)
	}
	// AFP-V0-520: tiers.sh reads the target's documentation declaration. Adding it is itself a
	// FULL change; afterwards a sealed CEM and declared documentation are not, and a Makefile is.
	declare := commit(map[string]string{".corvint/documentation-paths.json": `{"profile":"corvint-documentation-paths/0","paths":["docs/specs/"]}` + "\n"})
	if d := tiers(embedDelete, declare); fmt.Sprint(strs(sub(d, "tiers")["full"])) != "[a non-Go file changed: .corvint/documentation-paths.json]" {
		t.Fatalf("tiers.sh of a new documentation declaration = %v", d)
	}
	evidence := commit(map[string]string{"docs/specs/s.md": "spec\n",
		".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json": "{}\n"})
	if d := tiers(declare, evidence); fmt.Sprint(sub(d, "tiers")["changed"]) != "[]" || fmt.Sprint(sub(d, "tiers")["hop2"]) != "[]" ||
		len(strs(sub(d, "tiers")["full"])) != 0 || len(strs(d["tierIncomplete"])) != 0 {
		t.Fatalf("tiers.sh of a CEM and a declared spec = %v", d)
	}
	makefile := commit(map[string]string{"Makefile": "all:\n", "docs/specs/s.md": "spec 2\n"})
	if d := tiers(evidence, makefile); fmt.Sprint(strs(sub(d, "tiers")["full"])) != "[a non-Go file changed: Makefile]" {
		t.Fatalf("tiers.sh of a Makefile and a declared spec = %v", d)
	}
}

// TestAFPV0520TierExemptions checks that a sealed CEM directly under .corvint/changes/ and a path
// the documentation declaration names no longer force the tiers FULL, while every other non-Go
// path, every earlier FULL rule and an unusable declaration still do.
func TestAFPV0520TierExemptions(t *testing.T) {
	declared := []byte(`{"profile":"corvint-documentation-paths/0","paths":["b/static/","docs/build-log/","docs/specs/","notes.md"]}` + "\n")
	cem := ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	same := func(changed string, want doc) {
		t.Helper()
		d := tiersDeclared(t, changed, declared, tierModule)
		tr := sub(d, "tiers")
		if !reflect.DeepEqual(tr["changed"], want["changed"]) || !reflect.DeepEqual(tr["hop1"], want["hop1"]) ||
			!reflect.DeepEqual(tr["hop2"], want["hop2"]) || len(strs(tr["full"])) != 0 || len(strs(d["tierIncomplete"])) != 0 {
			t.Fatalf("tiers of %q = %v, want the tiers %v", changed, d, want)
		}
	}
	none := doc{"changed": []any{}, "hop1": []any{}, "hop2": []any{}}
	same(cem+"\n", none)
	same("docs/specs/affected-plan-v0.md\ndocs/build-log/2026-10-27-x.md\nnotes.md\n", none)
	goOnly := sub(tiersOf(t, "a/x.go\n", tierModule), "tiers")
	same(cem+"\ndocs/specs/s.md\na/x.go\n", goOnly)

	full := func(documentation []byte, changed string, reasons ...string) {
		t.Helper()
		d := tiersDeclared(t, changed, documentation, tierModule)
		tr := sub(d, "tiers")
		if tr["changed"] != "FULL" || tr["hop1"] != "FULL" || tr["hop2"] != "FULL" ||
			fmt.Sprint(strs(tr["full"])) != fmt.Sprint(reasons) {
			t.Fatalf("tiers of %q with %s = %v, want FULL because %q", changed, documentation, d, reasons)
		}
	}
	// Every other non-Go path, and a CEM anywhere but directly under .corvint/changes/, stays FULL.
	for _, p := range []string{"Makefile", "script/x.sh", "docs/x.md", "docs/specs", ".corvint/change.cem.json",
		".corvint/changes/x/y.cem.json", ".corvint/changes/.cem.json", ".corvint/changes/README.md",
		"docs/x.cem.json", ".corvint/documentation-paths.json", "notes.md.bak"} {
		full(declared, p+"\n", "a non-Go file changed: "+p)
	}
	full(declared, cem+"\ndocs/specs/s.md\nMakefile\na/x.go\n", "a non-Go file changed: Makefile")
	// The earlier rules still win inside a declared directory: testdata, an embedded file, .github/.
	full(declared, "docs/specs/testdata/in.json\n", "a testdata file changed: docs/specs/testdata/in.json")
	full(declared, "b/static/x.html\n", "a file a package embeds changed: b/static/x.html")
	full([]byte(`{"profile":"corvint-documentation-paths/0","paths":[".github/"]}`), ".github/x.md\n",
		"the change touches .github/: .github/x.md")
	// No declaration, an empty one, or one AFP-V0-160 (1) would refuse declares nothing.
	for _, bad := range []string{"", "{", "null", `{"profile":"corvint-documentation-paths/1","paths":["docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/specs/"],"x":1}`,
		`{"profile":"corvint-documentation-paths/0","paths":null}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/specs/","docs/build-log/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/specs/","docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":[".corvint/","docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":[".git/x","docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs//specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/../docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["/docs/specs/"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/specs//"]}`,
		`{"profile":"corvint-documentation-paths/0","paths":[1]}`,
		`{"profile":"corvint-documentation-paths/0","paths":["docs/specs/"]}` + strings.Repeat(" ", 64<<10),
	} {
		full([]byte(bad), "docs/specs/s.md\n", "a non-Go file changed: docs/specs/s.md")
	}
	// The sealed CEM rule needs no declaration.
	if tr := sub(tiersOf(t, cem+"\n", tierModule), "tiers"); !reflect.DeepEqual(tr["changed"], []any{}) {
		t.Fatalf("tiers of a sealed CEM without a declaration = %v", tr)
	}
}

// TestAFPV0480SelectionTiers checks selection.jq carries the tier document additively.
func TestAFPV0480SelectionTiers(t *testing.T) {
	want := readDoc(t, filepath.Join("testdata", "selection", "shard-3.json"))
	args := func(extra ...string) []string {
		a := []string{"-n", "-c", "--slurpfile", "dry", filepath.Join("testdata", "shadow", "dry-run-3.json"),
			"--slurpfile", "manifest", filepath.Join("testdata", "shadow", "manifest-3.json")}
		for _, k := range []string{"toolSource", "toolDigest", "head", "base", "target", "tree"} {
			a = append(a, "--arg", k, want[k].(string))
		}
		for _, k := range []string{"pr", "runId", "runAttempt", "shard", "shards", "exit"} {
			a = append(a, "--arg", k, strconv.Itoa(num(want[k])))
		}
		return append(append(a, extra...), "-f", script(t, "selection.jq"))
	}
	dir := t.TempDir()
	tiersDoc := doc{"tiers": doc{"changed": []any{"x"}, "hop1": []any{"x"}, "hop2": "FULL", "full": []any{"a module file changed: go.mod"}, "universe": 9},
		"tierIncomplete": []any{}, "millis": 840}
	writeDoc(t, filepath.Join(dir, "tiers.json"), tiersDoc)
	writeFile(t, filepath.Join(dir, "empty.json"), nil)
	for path, check := range map[string]func(d doc) bool{
		filepath.Join(dir, "tiers.json"): func(d doc) bool {
			return num(sub(d, "tiers")["millis"]) == 840 && sub(d, "tiers")["hop2"] == "FULL" && len(strs(d["tierIncomplete"])) == 0
		},
		filepath.Join(dir, "empty.json"): func(d doc) bool {
			return d["tiers"] == nil && fmt.Sprint(strs(d["tierIncomplete"])) == "[the tier computation printed no document]"
		},
	} {
		var d doc
		if err := json.Unmarshal([]byte(run(t, nil, "jq", args("--slurpfile", "tiers", path)...)), &d); err != nil {
			t.Fatal(err)
		}
		if !check(d) {
			t.Fatalf("selection with %s = tiers %v, tierIncomplete %v", path, d["tiers"], d["tierIncomplete"])
		}
		delete(d, "tiers")
		delete(d, "tierIncomplete")
		if !reflect.DeepEqual(d, want) {
			t.Fatalf("the tiers changed the rest of the selection document: %v", d)
		}
	}
}

// withTiers gives every shard's selection document the tier document T.
func withTiers(f *fixture, t doc) *fixture {
	for _, d := range f.sel {
		d["tiers"], d["tierIncomplete"] = t, []any{}
	}
	return f
}

func TestAFPV0481ShadowCost(t *testing.T) {
	costs, err := filepath.Abs("../../.github/cishards/package-costs.json")
	if err != nil {
		t.Fatal(err)
	}
	withCosts := map[string]string{"ROW_COSTS": costs}
	cost := func(row doc) doc { return sub(row, "shadowCost") }
	has := func(list any, fragment string) bool {
		for _, s := range strs(list) {
			if strings.Contains(s, fragment) {
				return true
			}
		}
		return false
	}

	t.Run("an old selection document without tiers prices S only", func(t *testing.T) {
		row, _ := queueFixture(t).classify(t, withCosts)
		c := cost(row)
		if row["class"] != "CLEAN" || c["changed"] != nil || c["hop1"] != nil || c["hop2"] != nil ||
			!has(c["incomplete"], "no selection document carries tiers") {
			t.Fatalf("row %v shadowCost = %v", row["class"], c)
		}
		s, full := sub(c, "selection"), sub(c, "full")
		if pm := num(s["permille"]); pm <= 0 || pm >= 1000 || num(s["missed"]) != 0 || num(full["packages"]) != 334 ||
			num(s["packages"]) == 0 || c["medianMs"] == nil {
			t.Fatalf("selection cost = %v, full = %v", s, full)
		}
		// Without a cost file nothing is priced, and the row says so; the class never changes.
		row, _ = queueFixture(t).classify(t, nil)
		if c = cost(row); row["class"] != "CLEAN" || sub(c, "selection")["ms"] != nil || sub(c, "selection")["permille"] != nil ||
			!has(c["incomplete"], "package-costs.json is missing") || c["unpriced"] != nil {
			t.Fatalf("unpriced row %v shadowCost = %v", row["class"], c)
		}
	})

	t.Run("a package missing from the cost file takes the median and is counted", func(t *testing.T) {
		f := queueFixture(t)
		set := map[string]bool{}
		for _, d := range f.out {
			for p := range sub(d, "outcomes") {
				set[p] = true
			}
		}
		var full []string
		for p := range set {
			full = append(full, p)
		}
		sort.Strings(full)
		ms, sum := doc{}, 0
		for i, p := range full[1:] {
			ms[p] = i + 1
			sum += i + 1
		}
		priced := len(full) - 1
		median := priced/2 + 1 // sorted[len/2] of 1..priced, the cishards convention
		path := filepath.Join(t.TempDir(), "costs.json")
		writeDoc(t, path, doc{"profile": "corvint-ci-package-costs/0", "milliseconds": ms})
		row, _ := f.classify(t, map[string]string{"ROW_COSTS": path})
		c := cost(row)
		if num(c["unpriced"]) != 1 || num(c["medianMs"]) != median || num(sub(c, "full")["ms"]) != sum+median ||
			has(c["incomplete"], "package-costs.json") {
			t.Fatalf("shadowCost = %v, want 1 unpriced at median %d of full %d", c, median, sum+median)
		}
		// A cost file of another profile is never read as prices.
		writeDoc(t, path, doc{"profile": "other/0", "milliseconds": ms})
		if row, _ = f.classify(t, map[string]string{"ROW_COSTS": path}); !has(cost(row)["incomplete"], "package-costs.json is missing") {
			t.Fatalf("shadowCost of a foreign cost file = %v", cost(row))
		}
	})

	t.Run("a go.mod change makes every tier FULL", func(t *testing.T) {
		tr := tiersOf(t, "go.mod\n", nil)
		tiers := sub(tr, "tiers")
		tiers["millis"] = 870
		row, _ := withTiers(queueFixture(t), tiers).classify(t, withCosts)
		c := cost(row)
		for _, name := range []string{"changed", "hop1", "hop2"} {
			if m := sub(c, name); m["full"] != true || num(m["permille"]) != 1000 || num(m["missed"]) != 0 {
				t.Fatalf("%s = %v, want FULL", name, m)
			}
		}
		if row["class"] != "CLEAN" || fmt.Sprint(strs(c["fullReasons"])) != "[a module file changed: go.mod]" ||
			num(c["tierMillis"]) != 870 || len(strs(c["incomplete"])) != 0 {
			t.Fatalf("row %v shadowCost = %v", row["class"], c)
		}
	})

	t.Run("a real failure outside a tier is that tier's miss; a FLAKY one is not", func(t *testing.T) {
		tiers := doc{"changed": []any{otherPkg}, "hop1": []any{otherPkg, flakyPkg}, "hop2": "FULL", "full": []any{"x"}, "universe": 9}
		for _, rerun := range []string{"fail", "pass"} {
			base := queueFixture(t)
			fail(base.out[7], flakyPkg, "fail", "RAN", rerun)
			want, _ := base.classify(t, withCosts)
			f := withTiers(queueFixture(t), tiers)
			fail(f.out[7], flakyPkg, "fail", "RAN", rerun)
			row, _ := f.classify(t, withCosts)
			if row["class"] != want["class"] || !reflect.DeepEqual(row["missed"], want["missed"]) {
				t.Fatalf("tiers changed the row: class %v missed %v, want %v %v", row["class"], row["missed"], want["class"], want["missed"])
			}
			c := cost(row)
			wantMiss := 0
			if rerun == "fail" {
				wantMiss = 1
			}
			ch := sub(c, "changed")
			if num(ch["missed"]) != wantMiss || num(sub(c, "hop1")["missed"]) != 0 || num(sub(c, "hop2")["missed"]) != 0 ||
				num(sub(c, "selection")["missed"]) != 0 || num(ch["packages"]) != 1 {
				t.Fatalf("rerun %s: shadowCost = %v", rerun, c)
			}
			if rerun == "fail" && fmt.Sprint(strs(ch["missedPackages"])) != "["+flakyPkg+"]" {
				t.Fatalf("missedPackages = %v", ch["missedPackages"])
			}
		}
	})

	t.Run("an oversized tier document never changes the row or its class", func(t *testing.T) {
		want, _ := queueFixture(t).classify(t, withCosts)
		delete(want, "shadowCost")
		var reasons []any
		for range 25 {
			reasons = append(reasons, strings.Repeat("x", 70000))
		}
		for _, inIncomplete := range []bool{false, true} {
			f := withTiers(queueFixture(t), doc{"changed": "FULL", "hop1": "FULL", "hop2": "FULL", "full": reasons, "universe": 9})
			if inIncomplete {
				for _, d := range f.sel {
					d["tierIncomplete"] = reasons
				}
			}
			row, _ := f.classify(t, withCosts)
			c := cost(row)
			delete(row, "shadowCost")
			if !reflect.DeepEqual(row, want) {
				t.Fatalf("an oversized tier document (tierIncomplete %v) changed the row: %v, want %v", inIncomplete, row, want)
			}
			if inIncomplete == (len(strs(c["fullReasons"])) > 0) || len(strs(c["fullReasons"])) > 20 || len(strs(c["incomplete"])) > 21 {
				t.Fatalf("shadowCost (tierIncomplete %v) is unbounded: %d reasons, %d incomplete", inIncomplete,
					len(strs(c["fullReasons"])), len(strs(c["incomplete"])))
			}
			for _, s := range append(strs(c["fullReasons"]), strs(c["incomplete"])...) {
				if len(s) > 310 {
					t.Fatalf("shadowCost keeps a %d-byte string", len(s))
				}
			}
		}
	})

	t.Run("shards that disagree on the tiers measure no tier", func(t *testing.T) {
		f := withTiers(queueFixture(t), doc{"changed": []any{otherPkg}, "hop1": []any{otherPkg}, "hop2": []any{otherPkg}, "full": []any{}, "universe": 9})
		f.sel[0]["tiers"] = doc{"changed": []any{}, "hop1": []any{}, "hop2": []any{}, "full": []any{}, "universe": 9}
		row, _ := f.classify(t, withCosts)
		if c := cost(row); row["class"] != "CLEAN" || c["changed"] != nil || !has(c["incomplete"], "selection shards disagree on the tiers") {
			t.Fatalf("row %v shadowCost = %v", row["class"], c)
		}
	})
}

func TestAFPV0484BarShadowCost(t *testing.T) {
	name := func(class string, pr int) string { return fmt.Sprintf("ci-forward-%s-pr%d-%040x-e1-f0", class, pr, pr) }
	costName := func(class string, pr int, epoch, slots string) string {
		f := "0"
		if class == "MISS" {
			f = "1"
		}
		return fmt.Sprintf("ci-forward-cost-pr%d-%040x-e%s-%s-f%s-%s", pr, pr, epoch, class, f, slots)
	}
	missName := func(pr int) string { return fmt.Sprintf("ci-forward-MISS-pr%d-%040x-e1-f1", pr, pr) }
	bar := func(t *testing.T, names []string) doc {
		in := filepath.Join(t.TempDir(), "names")
		writeFile(t, in, lines(names...))
		cmd := exec.Command("jq", "-R", "-s", "-c", "--arg", "epoch", "1", "--arg", "format", "json", "-f", script(t, "bar.jq"))
		r, err := os.Open(in)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		cmd.Stdin = r
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("bar.jq: %v", err)
		}
		var d doc
		if err := json.Unmarshal(out, &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	// Newest first: each job uploads its row and then its cost name, so a row's cost is listed
	// just before it.
	rows := []string{name("CLEAN", 1), missName(2), missName(3), name("INCOMPLETE", 4)}
	plain := bar(t, rows)
	with := bar(t, []string{
		costName("CLEAN", 1, "1", "s0400-b0010-cna-d1000-m0n00"),
		name("CLEAN", 1),
		costName("MISS", 2, "1", "s0999-b0999-c0999-d0999-m0000"), // listed earlier: not the nearest
		costName("MISS", 2, "1", "s0600-b0020-c0100-d0300-m1110"),
		missName(2),
		costName("MISS", 2, "1", "s0001-b0001-c0001-d0001-m0000"), // uploaded before the row: an older run
		costName("MISS", 3, "0", "s0001-b0001-c0001-d0001-m0000"), // another epoch
		missName(3),
		costName("CLEAN", 4, "1", "s0001-b0001-c0001-d0001-m0000"), // an INCOMPLETE row is not measured
		name("INCOMPLETE", 4),
		costName("CLEAN", 5, "1", "s0001-b0001-c0001-d0001-m0000"), // no row
		"ci-forward-cost-pr1-bad"})
	sc := sub(with, "shadowCost")
	delete(with, "shadowCost")
	delete(plain, "shadowCost")
	if !reflect.DeepEqual(with, plain) {
		t.Fatalf("cost names changed the bar: %v, want %v", with, plain)
	}
	if plain["killSwitch"] != "TRIPPED" {
		t.Fatalf("fixture bar = %v, want the kill switch tripped", plain)
	}
	e := sub(sc, "epoch")
	if num(e["complete"]) != 3 || num(e["priced"]) != 2 {
		t.Fatalf("epoch shadowCost = %v", e)
	}
	want := map[string]string{
		"selection": "map[meanShare:0.5 measured:2 medianShare:0.6 missMeasured:2 missRows:1]",
		"changed":   "map[meanShare:0.015 measured:2 medianShare:0.02 missMeasured:1 missRows:1]",
		"hop1":      "map[meanShare:0.1 measured:1 medianShare:0.1 missMeasured:2 missRows:1]",
		"hop2":      "map[meanShare:0.65 measured:2 medianShare:1 missMeasured:2 missRows:0]",
	}
	for k, v := range want {
		if got := fmt.Sprint(e[k]); got != v {
			t.Fatalf("epoch %s = %s, want %s", k, got, v)
		}
	}
	if !reflect.DeepEqual(sc["window"], sc["epoch"]) || !strings.Contains(fmt.Sprint(sc["advisory"]), "never feeds") {
		t.Fatalf("shadowCost = %v", sc)
	}
	// A CLEAN re-run whose cost upload failed is unpriced: neither the older MISS run's cost nor an
	// older CLEAN run's cost prices it.
	retry := sub(bar(t, []string{
		name("CLEAN", 6),
		costName("MISS", 6, "1", "s0500-b0100-c0200-d0300-m1000"),
		missName(6),
		costName("CLEAN", 6, "1", "s0500-b0100-c0200-d0300-m0000"),
		name("CLEAN", 6)}), "shadowCost")
	if e := sub(retry, "epoch"); num(e["complete"]) != 1 || num(e["priced"]) != 0 || num(sub(e, "selection")["missRows"]) != 0 {
		t.Fatalf("a re-run without its own cost = %v, want unpriced", e)
	}
}
