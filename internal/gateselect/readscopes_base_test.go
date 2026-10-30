package gateselect

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// baseRepository writes files and symlinks (link name to target) into a new
// Git repository, commits them and returns the root and the commit id.
func baseRepository(t *testing.T, files, links map[string]string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root := writeFixture(t, files)
	for name, target := range links {
		link := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, root, "init", "-q")
	fixtureGit(t, root, "add", "-A")
	fixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "base")
	return root, strings.TrimSpace(fixtureGit(t, root, "rev-parse", "HEAD"))
}

func fixtureGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(output)
}

// emptyTreeCommit commits, on top of base, an empty tree at name, which the
// index cannot hold, replacing any entry already at name.
func emptyTreeCommit(t *testing.T, root, base, name string) string {
	t.Helper()
	mktree := func(listing string) string {
		command := exec.Command("git", "-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "mktree", "-z")
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		command.Stdin = strings.NewReader(listing)
		output, err := command.Output()
		if err != nil {
			t.Fatalf("git mktree: %v", err)
		}
		return strings.TrimSpace(string(output))
	}
	// with is the tree at directory in base, absent or not, with leaf set to entry.
	with := func(directory, leaf, entry string) string {
		listing := ""
		if output, err := exec.Command("git", "-C", root, "ls-tree", "-z", base+":"+directory).Output(); err == nil {
			for _, record := range strings.Split(string(output), "\x00") {
				if _, name, _ := strings.Cut(record, "\t"); record != "" && name != leaf {
					listing += record + "\x00"
				}
			}
		}
		return mktree(listing + "040000 tree " + entry + "\t" + leaf + "\x00")
	}
	entry := mktree("")
	for directory := name; directory != "."; directory = path.Dir(directory) {
		parent := path.Dir(directory)
		if parent == "." {
			parent = ""
		}
		entry = with(parent, path.Base(directory), entry)
	}
	return strings.TrimSpace(fixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", entry, "-p", base, "-m", "empty tree"))
}

func declaration(packages string) string {
	return `{"profile":"corvint-test-read-scopes/0","packages":{` + packages + `}}`
}

// declaredLines returns the packages selectPackages selects on a `declared`
// line, in order, and the verdict.
func declaredLines(plan Receipt, root string) ([]string, string) {
	output := selectPackages(plan, fixtureModule, root, true)
	var declared []string
	for _, line := range output[:len(output)-1] {
		if rest, ok := strings.CutPrefix(line, "declared "+fixtureModule+"/"); ok {
			declared = append(declared, strings.Split(rest, " ")[0])
		}
	}
	return declared, output[len(output)-1]
}

// AFP-V0-050 (V1-1399): with the plan's base known, an edit of the read-scope
// declaration selects only the packages whose key was added, changed or
// removed, and a package whose declared scope holds the declaration; an
// unknown base keeps the AFP-V0-023 rule and selects every declared package.
func TestSelectPackagesNarrowsDeclarationEditsAgainstTheBase_AFPV0050(t *testing.T) {
	files := map[string]string{}
	for _, name := range []string{"alpha", "beta", "gamma", "delta", "keeper"} {
		files[name+"/"+name+".go"] = "package " + name + "\n"
	}
	files[readScopesPath] = declaration(`"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"keeper":[".corvint/"]`)
	root, base := baseRepository(t, files, nil)
	cases := []struct {
		name, current, revision, base string
		want                          []string
		snapshot                      bool
	}{
		{"added key", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, base, "", []string{"gamma", "keeper"}, false},
		{"changed entries", `"alpha":["docs/"],"beta":["spec/"],"delta":["docs/"],"keeper":[".corvint/"]`, base, "", []string{"beta", "keeper"}, false},
		{"removed key", `"alpha":["docs/"],"beta":["docs/"],"keeper":[".corvint/"]`, base, "", []string{"delta", "keeper"}, false},
		{"range base wins over revision", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, strings.Repeat("0", 40), base, []string{"gamma", "keeper"}, false},
		{"no base", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, "", "", []string{"alpha", "beta", "delta", "gamma", "keeper"}, false},
		{"missing commit", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, strings.Repeat("0", 40), "", []string{"alpha", "beta", "delta", "gamma", "keeper"}, false},
		{"snapshot plan", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, base, base, []string{"alpha", "beta", "delta", "gamma", "keeper"}, true},
		{"abbreviated commit", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"gamma":["docs/"],"keeper":[".corvint/"]`, base[:12], "", []string{"alpha", "beta", "delta", "gamma", "keeper"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(readScopesPath)), []byte(declaration(tc.current)), 0o644); err != nil {
				t.Fatal(err)
			}
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{readScopesPath}
			plan.Revision, plan.Range.Base = tc.revision, tc.base
			if tc.snapshot {
				plan.Snapshot = map[string]any{"id": "s"}
			}
			declared, verdict := declaredLines(plan, root)
			if !slices.Equal(declared, tc.want) {
				t.Fatalf("declared %v, want %v (verdict %q)", declared, tc.want, verdict)
			}
		})
	}
	// Any other dirty path keeps the AFP-V0-023 rule whatever the base.
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(readScopesPath)), []byte(files[readScopesPath]), 0o644); err != nil {
		t.Fatal(err)
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"docs/x.md"}
	plan.Revision = base
	if declared, _ := declaredLines(plan, root); !slices.Equal(declared, []string{"alpha", "beta", "delta"}) {
		t.Fatalf("docs/x.md declared %v, want alpha, beta and delta", declared)
	}
}

// AFP-V0-050 (V1-1399, V1-1400): the selector reads the base declaration with
// the same existence and ownership checks as golang.CommittedReadScopes, and
// agrees with it on every case: an unknown base is nil there and an error
// here.
func TestBaseReadScopesFollowCommittedReadScopes_AFPV0050(t *testing.T) {
	common := map[string]string{
		"go.mod":               "module example.com/fixture\n",
		"kept/kept.go":         "package kept\n",
		"nested/go.mod":        "module example.test/nested\n",
		"nested/x/x.go":        "package x\n",
		"inner/go.mod":         "module example.test/inner\n",
		"inner/inner.go":       "package inner\n",
		"kept/testdata/go.mod": "module example.test/fixture\n",
		"kept/testdata/f/f.go": "package f\n",
		"kept/testdata/g/g.go": "package g\n",
	}
	for name, test := range map[string]struct {
		packages string
		extra    map[string]string
		links    map[string]string
		absent   bool
		known    bool
		// emptyTree names an empty directory committed on top.
		emptyTree string
	}{
		"the root module's package":       {packages: `"kept":[]`, known: true},
		"no declaration":                  {absent: true, known: true},
		"below a nested module":           {packages: `"kept":[],"nested/x":[]`},
		"a nested module's root":          {packages: `"inner":[]`},
		"in a fixture module":             {packages: `"kept/testdata/f":[]`},
		"in the owner's testdata":         {packages: `"kept/testdata/g":[]`},
		"a workspace-listed module":       {packages: `"kept":[],"nested/x":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse (\n\t.\n\t./nested // listed\n)\n"}, known: true},
		"an unlisted root in a workspace": {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse ./nested\n"}},
		// The planner reads go.work with its strict manifest grammar (V1-2317).
		"a raw-string workspace use":       {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse `.`\n"}},
		"an unknown workspace directive":   {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuses .\nuse .\n"}},
		"a raw-string workspace block use": {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse (\n\t.\n\t`./nested`\n)\n"}},
		"a module directive in go.work":    {packages: `"kept":[]`, extra: map[string]string{"go.work": "module example.com/fixture\n\nuse .\n"}},
		"an unterminated workspace block":  {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse (\n\t.\n"}},
		"a quoted workspace use":           {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\n\nuse \".\" // root\n"}, known: true},
		// The planner checks each directive's arguments (AFP-V0-403, V1-2370).
		"a malformed workspace go":        {packages: `"kept":[]`, extra: map[string]string{"go.work": "go latest\n\nuse .\n"}},
		"a malformed workspace toolchain": {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\ntoolchain gccgo\n\nuse .\n"}},
		"a malformed workspace godebug":   {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\ngodebug panicnil\n\nuse .\n"}},
		"a malformed workspace replace":   {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27\nreplace example.com/a v1 => ./a\n\nuse .\n"}},
		"valid workspace directives":      {packages: `"kept":[]`, extra: map[string]string{"go.work": "go 1.27.2\ntoolchain go1.27.2\ngodebug panicnil=1\nreplace example.com/a v1.2.3 => ./a\n\nuse .\n"}, known: true},
		"a directory named go.mod":        {packages: `"kept":[]`, extra: map[string]string{"kept/go.mod/data": "x\n"}, known: true},
		"a symlinked go.mod":              {packages: `"solo":[]`, extra: map[string]string{"solo/solo.go": "package solo\n"}, links: map[string]string{"solo/go.mod": "../go.mod"}},
		"in a vendor directory":           {packages: `"vendor/v":[]`, extra: map[string]string{"vendor/v/v.go": "package v\n"}},
		"in a hidden directory":           {packages: `".hidden/h":[]`, extra: map[string]string{".hidden/h/h.go": "package h\n"}},
		"in an admitted build directory":  {packages: `"build/b":[]`, extra: map[string]string{"build/b/b.go": "package b\n"}, known: true},
		"a directory without Go source":   {packages: `"docs":[]`, extra: map[string]string{"docs/x.md": "x\n"}},
		"a symlinked Go source":           {packages: `"linked":[]`, links: map[string]string{"linked/l.go": "../kept/kept.go"}},
		"an invalid declaration":          {packages: `"kept":["z","a"]`},
		// Git stores a symlink's target text as its blob: here that text is
		// itself a valid declaration, which only the mode check refuses.
		"a symlinked declaration": {absent: true, links: map[string]string{readScopesPath: declaration(`"kept":[]`)}},
		// `git ls-tree -r` alone omits an empty tree, which would read as absent.
		"an empty tree at go.work":         {packages: `"kept":[]`, emptyTree: workspaceManifest},
		"an empty tree at the declaration": {absent: true, emptyTree: readScopesPath},
		"an empty tree named go.mod":       {packages: `"solo":[]`, extra: map[string]string{"solo/solo.go": "package solo\n"}, emptyTree: "solo/go.mod", known: true},
	} {
		t.Run(name, func(t *testing.T) {
			files := maps.Clone(common)
			maps.Copy(files, test.extra)
			if !test.absent {
				files[readScopesPath] = declaration(test.packages)
			}
			root, base := baseRepository(t, files, test.links)
			if test.emptyTree != "" {
				base = emptyTreeCommit(t, root, base, test.emptyTree)
			}
			var plan Receipt
			plan.Revision = base
			scopes := baseReadScopes(plan, root)
			if (scopes != nil) != test.known {
				t.Fatalf("base declaration = %v; want known = %v", scopes, test.known)
			}
			committed, err := golang.CommittedReadScopes(context.Background(), root, base)
			if (err == nil) != test.known || (test.known && !maps.EqualFunc(scopes, committed, slices.Equal)) {
				t.Fatalf("CommittedReadScopes = %v, %v; selector = %v", committed, err, scopes)
			}
		})
	}
}

// A tree naming one path twice is malformed: the planner's verified reader
// refuses it, so the selector's base is unknown too.
func TestParseBaseTreeRefusesADuplicateName(t *testing.T) {
	object := strings.Repeat("1", 40)
	record := "100644 blob " + object + "\t" + readScopesPath + "\x00"
	if _, ok := parseBaseTree([]byte(record)); !ok {
		t.Fatal("a single record is refused")
	}
	if _, ok := parseBaseTree([]byte(record + record)); ok {
		t.Fatal("a duplicate name is admitted")
	}
}

// skippedDirectories is affected.SkippedDirectories less the hidden names and
// the ones the Go plugin admits.
func TestSkippedDirectoriesMirrorTheObserverWalk(t *testing.T) {
	want := map[string]bool{}
	for name := range affected.SkippedDirectories {
		if !strings.HasPrefix(name, ".") && !slices.Contains([]string{"build", "dist", "target"}, name) {
			want[name] = true
		}
	}
	if !maps.Equal(skippedDirectories, want) {
		t.Fatalf("skippedDirectories = %v, want %v", skippedDirectories, want)
	}
}
