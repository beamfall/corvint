package cishards

import (
	"bufio"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file reads the checkout's workflows, so it is not one of the files the isolated helper
// modules copy; the go-static job runs it in the checkout (AFP-V0-2726).

// isolatedModule is one isolated helper module a workflow step assembles under $RUNNER_TEMP.
type isolatedModule struct {
	where   string            // workflow:line of the directory assignment
	module  string            // the module path its printf writes to go.mod
	version string            // the go directive of that go.mod
	files   map[string]string // module-relative destination -> repository source
}

var (
	// d="$RUNNER_TEMP/ci-shards-source"
	isolatedDirLine = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)="\$RUNNER_TEMP/([A-Za-z0-9._-]+)"\s*$`)
	// cp .github/cishards/a.go .github/cishards/b.json "$d/"   or   ... "$d/cmd/"
	isolatedCpLine = regexp.MustCompile(`^\s*cp((?:\s+\.github/[A-Za-z0-9._/-]+)+)\s+"\$([A-Za-z_][A-Za-z0-9_]*)/((?:[A-Za-z0-9._-]+/)*)"\s*$`)
	// printf 'module github.com/Beamfall/corvint/.github/cishards\n\ngo 1.27.3\n' > "$d/go.mod"
	isolatedModLine = regexp.MustCompile(`^\s*printf 'module ([^\\' ]+)\\n\\ngo ([0-9.]+)\\n' > "\$([A-Za-z_][A-Za-z0-9_]*)/go\.mod"\s*$`)
	// Any copy of a protected helper source; each must belong to a parsed module.
	helperCpLine = regexp.MustCompile(`^\s*cp\s.*\.github/(cishards|testconfine)/`)
	// A new workflow step or job ends the scope of the step's shell variables.
	stepBoundary = regexp.MustCompile(`^\s*- [A-Za-z_-]+:|^  [A-Za-z0-9_-]+:\s*$`)
)

// isolatedModules parses every isolated helper module of one workflow file. A copy of a
// .github/cishards or .github/testconfine file that it cannot attribute to a module is an error,
// so a changed copy idiom fails the check instead of escaping it.
func isolatedModules(t *testing.T, workflow string) []*isolatedModule {
	t.Helper()
	f, err := os.Open(workflow)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []*isolatedModule
	vars := map[string]*isolatedModule{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for n := 1; scanner.Scan(); n++ {
		line := scanner.Text()
		where := filepath.Base(workflow) + ":" + strconv.Itoa(n)
		if stepBoundary.MatchString(line) {
			vars = map[string]*isolatedModule{}
		}
		if m := isolatedDirLine.FindStringSubmatch(line); m != nil {
			vars[m[1]] = &isolatedModule{where: where, files: map[string]string{}}
			out = append(out, vars[m[1]])
			continue
		}
		if m := isolatedModLine.FindStringSubmatch(line); m != nil {
			if mod := vars[m[3]]; mod != nil {
				mod.module, mod.version = m[1], m[2]
				continue
			}
			t.Errorf("%s: go.mod written into $%s, which no $RUNNER_TEMP module directory of this step names", where, m[3])
			continue
		}
		if !helperCpLine.MatchString(line) {
			continue
		}
		m := isolatedCpLine.FindStringSubmatch(line)
		if m == nil || vars[m[2]] == nil {
			t.Errorf("%s: a helper source copy that names no isolated module of this step: %s", where, strings.TrimSpace(line))
			continue
		}
		for _, src := range strings.Fields(m[1]) {
			vars[m[2]].files[m[3]+path.Base(src)] = src
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	// A $RUNNER_TEMP directory that copies no helper source is some other scratch directory.
	kept := out[:0]
	for _, mod := range out {
		if len(mod.files) > 0 {
			kept = append(kept, mod)
		}
	}
	return kept
}

// embedGaps names each //go:embed pattern of a copied Go file that matches no copied file.
func embedGaps(t *testing.T, root string, mod *isolatedModule) []string {
	t.Helper()
	var gaps []string
	for dest, src := range mod.files {
		if !strings.HasSuffix(dest, ".go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
		if err != nil {
			t.Fatalf("%s: %v", mod.where, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			rest, ok := strings.CutPrefix(strings.TrimSpace(line), "//go:embed ")
			if !ok {
				continue
			}
			for _, pattern := range strings.Fields(rest) {
				pattern = strings.TrimPrefix(strings.Trim(pattern, "\"`"), "all:")
				want := path.Join(path.Dir(dest), pattern)
				found := false
				for have := range mod.files {
					if ok, _ := path.Match(want, have); ok || strings.HasPrefix(have, want+"/") {
						found = true
						break
					}
				}
				if !found {
					gaps = append(gaps, src+" embeds "+pattern+", which the step does not copy")
				}
			}
		}
	}
	sort.Strings(gaps)
	return gaps
}

// isolatedGo runs one go command in an isolated module with the environment the workflows export.
func isolatedGo(dir string, extra []string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	replace := map[string]string{"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "GOFLAGS": ""}
	for _, kv := range extra {
		k, v, _ := strings.Cut(kv, "=")
		replace[k] = v
	}
	for _, value := range os.Environ() {
		if _, ok := replace[strings.SplitN(value, "=", 2)[0]]; !ok {
			cmd.Env = append(cmd.Env, value)
		}
	}
	for key, value := range replace {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	return cmd.CombinedOutput()
}

// TestIsolatedHelperBuildsCopyWhatTheyNeed checks, for every isolated helper build in every
// workflow, that the files it copies carry every file its packages and their //go:embed
// directives need: it replays each distinct module from the checkout and vets and builds it the
// way the workflows do (AFP-V0-2726).
func TestIsolatedHelperBuildsCopyWhatTheyNeed_AFPV02726(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("no workflow under %s: %v", root, err)
	}
	type replay struct {
		mod   *isolatedModule
		where []string
	}
	distinct := map[string]*replay{}
	perWorkflow := map[string]int{}
	for _, workflow := range workflows {
		for _, mod := range isolatedModules(t, workflow) {
			if mod.module == "" {
				t.Errorf("%s: the isolated module writes no go.mod with printf 'module ...'", mod.where)
				continue
			}
			if gaps := embedGaps(t, root, mod); len(gaps) > 0 {
				t.Errorf("%s: %s", mod.where, strings.Join(gaps, "; "))
			}
			perWorkflow[filepath.Base(workflow)]++
			dests := make([]string, 0, len(mod.files))
			for dest, src := range mod.files {
				dests = append(dests, dest+"<"+src)
			}
			sort.Strings(dests)
			key := mod.module + " go" + mod.version + " " + strings.Join(dests, " ")
			if distinct[key] == nil {
				distinct[key] = &replay{mod: mod}
			}
			distinct[key].where = append(distinct[key].where, mod.where)
		}
	}
	// The repository's helper builds: a parser that finds none of them has drifted.
	for _, name := range []string{"ci.yml", "linecov-index.yml"} {
		if perWorkflow[name] == 0 {
			t.Errorf("found no isolated helper module in %s; the parser no longer reads its copy steps", name)
		}
	}
	keys := make([]string, 0, len(distinct))
	for key := range distinct {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r := distinct[key]
		dir := t.TempDir()
		for dest, src := range r.mod.files {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
			if err != nil {
				// cp would fail the workflow step the same way.
				t.Errorf("%s: copies %s: %v", strings.Join(r.where, ", "), src, err)
				continue
			}
			target := filepath.Join(dir, filepath.FromSlash(dest))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, raw, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		gomod := "module " + r.mod.module + "\n\ngo " + r.mod.version + "\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := isolatedGo(dir, nil, "vet", "./..."); err != nil {
			t.Errorf("%s: the isolated %s module does not vet: %v\n%s", strings.Join(r.where, ", "), r.mod.module, err, out)
		}
		if out, err := isolatedGo(dir, []string{"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0"}, "build", "./..."); err != nil {
			t.Errorf("%s: the isolated %s module does not build for windows/amd64: %v\n%s", strings.Join(r.where, ", "), r.mod.module, err, out)
		}
	}
}
