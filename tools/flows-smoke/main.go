// SPDX-License-Identifier: Apache-2.0

// flows-smoke is the FSV-V1-007 release smoke test: with only the candidate
// `corvint` binary installed, it runs `corvint affected` and the PROD-DOCS-011
// flow reports (`corvint flows status`, `corvint flows gaps`) on a pinned
// fixture repository, once with a version 0 flows declaration and once on a
// copy whose profile is one above the highest version this source supports.
// It exits 1, naming every difference, when the supported run does not print
// the expected plan and reports, when the unsupported run plans fewer than
// all E2E tests or does not name FLOWS_SCHEMA_UNSUPPORTED, when a report
// prints partial output, or when any other corvint executable is on the PATH
// the candidate runs with. The release workflow wiring is a separate change.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
)

const usage = `usage: go run ./tools/flows-smoke (--archive corvint_linux_amd64.tar.gz | --binary FILE) [--keep DIR]

Runs the FSV-V1-007 smoke test of the flows schema version with corvint
installed alone: corvint affected --base and corvint flows status|gaps on a
pinned fixture, with a version 0 flows declaration and with one above the
highest supported version. --archive extracts only the corvint member of a
release archive; --binary uses a built corvint. --keep DIR keeps the fixture
repositories and outputs under DIR. Exit 0 when every check holds, 1 with one
line per failed check, 2 on bad arguments.

Example:
  go run ./tools/flows-smoke --archive dist/corvint_linux_amd64.tar.gz
`

// maxArchiveMember bounds the extracted binary.
const maxArchiveMember = 512 << 20

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("flows-smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	archive := flags.String("archive", "", "release archive holding corvint")
	binary := flags.String("binary", "", "built corvint binary")
	keep := flags.String("keep", "", "directory that keeps the fixtures and outputs")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || (*archive == "") == (*binary == "") {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprint(stderr, usage)
		return 2
	}
	work := *keep
	if work == "" {
		dir, err := os.MkdirTemp("", "flows-smoke-")
		if err != nil {
			fmt.Fprintln(stderr, "flows-smoke:", err)
			return 1
		}
		defer os.RemoveAll(dir)
		work = dir
	} else if err := os.MkdirAll(work, 0o700); err != nil {
		fmt.Fprintln(stderr, "flows-smoke:", err)
		return 1
	}
	failures, err := smokeCandidate(context.Background(), work, *archive, *binary)
	if err != nil {
		fmt.Fprintln(stderr, "flows-smoke:", err)
		return 1
	}
	for _, failure := range failures {
		fmt.Fprintln(stderr, "flows-smoke: FAIL", failure)
	}
	if len(failures) != 0 {
		return 1
	}
	fmt.Fprintln(stdout, "flows-smoke: PASS corvint alone plans and reports the supported flows file and refuses the unsupported one (FSV-V1-007)")
	return 0
}

// smokeCandidate installs the candidate alone, builds the two fixtures and
// runs the checks.
func smokeCandidate(ctx context.Context, work, archive, binary string) ([]string, error) {
	bin := filepath.Join(work, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return nil, err
	}
	installed := filepath.Join(bin, "corvint")
	if archive != "" {
		if err := extractCorvint(archive, installed); err != nil {
			return nil, err
		}
	} else if err := copyFile(binary, installed); err != nil {
		return nil, err
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git is required to build the fixture: %w", err)
	}
	path := bin + string(os.PathListSeparator) + filepath.Dir(git)
	if failure := aloneOnPath(path, installed); failure != "" {
		return []string{failure}, nil
	}
	env := []string{"PATH=" + path, "HOME=" + filepath.Join(work, "home"), "XDG_CACHE_HOME=" + filepath.Join(work, "cache"), "LC_ALL=C", "TZ=UTC"}
	supported, err := buildFixture(ctx, git, filepath.Join(work, "supported"), appflows.FlowsWriteVersion())
	if err != nil {
		return nil, err
	}
	unsupported, err := buildFixture(ctx, git, filepath.Join(work, "unsupported"), appflows.FlowsWriteVersion()+1)
	if err != nil {
		return nil, err
	}
	runner := func(root string, args ...string) result {
		command := exec.CommandContext(ctx, installed, append([]string{"--root", root}, args...)...)
		command.Dir, command.Env = root, env
		var out, diag bytes.Buffer
		command.Stdout, command.Stderr = &out, &diag
		err := command.Run()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			code, diag = -1, *bytes.NewBufferString(err.Error())
		}
		return result{code: code, stdout: out.String(), stderr: diag.String()}
	}
	return checks(runner, supported, unsupported, appflows.FlowsWriteVersion()+1), nil
}

// extractCorvint writes the archive's one `<root>/corvint` member to dst and
// nothing else.
func extractCorvint(archive, dst string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%s: %w", archive, err)
	}
	reader := tar.NewReader(zipped)
	found := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %w", archive, err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "corvint" || strings.Count(strings.Trim(header.Name, "/"), "/") != 1 {
			continue
		}
		if found {
			return fmt.Errorf("%s: more than one corvint member", archive)
		}
		if header.Size > maxArchiveMember {
			return fmt.Errorf("%s: %s is larger than %d bytes", archive, header.Name, maxArchiveMember)
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(reader, maxArchiveMember))
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("%s: no <root>/corvint member", archive)
	}
	return nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o755)
}

// aloneOnPath names any executable on path, other than installed, whose name
// is corvint or begins with corvint-: the test is "installed alone".
func aloneOnPath(path, installed string) string {
	var others []string
	for _, dir := range filepath.SplitList(path) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := filepath.Join(dir, entry.Name())
			if (entry.Name() == "corvint" || strings.HasPrefix(entry.Name(), "corvint-")) && name != installed {
				others = append(others, name)
			}
		}
	}
	if len(others) == 0 {
		return ""
	}
	slices.Sort(others)
	return "corvint is not installed alone: the PATH also holds " + strings.Join(others, ", ")
}

// fixture is one built repository: the base the plan is measured from and
// the E2E test files its flows attach.
type fixture struct {
	root, base, head string
	e2e              []string
}

// fixtureE2E are the E2E test files the fixture's flows attach.
var fixtureE2E = []string{"e2e/checkout.spec.ts", "e2e/search.spec.ts"}

// buildFixture writes the pinned fixture with a flows declaration of
// version and commits it as A (unreviewed links), B (links reviewed at A)
// and C (a change to src/checkout.ts), with fixed identities and dates so
// the commits are the same on every host. B is the base.
func buildFixture(ctx context.Context, git, root string, version int) (fixture, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fixture{}, err
	}
	files := map[string]string{
		"go.mod":                      "module example.test/smoke\n",
		"lib/lib.go":                  "package lib\n\nfunc Help() string { return \"help\" }\n",
		"lib/lib_test.go":             "package lib\n\nimport \"testing\"\n\nfunc TestHelp(t *testing.T) { Help() }\n",
		"package.json":                `{"name":"shop","private":true}` + "\n",
		"package-lock.json":           `{"name":"shop","lockfileVersion":3}` + "\n",
		"playwright.config.ts":        `export default { testDir: "e2e", projects: [{ name: "chromium" }] };` + "\n",
		"e2e/pages/shop.ts":           `export const origin = "http://localhost:3000";` + "\n",
		"e2e/checkout.spec.ts":        "import { origin } from \"./pages/shop\";\n\ntest(\"pays\", () => origin);\n",
		"e2e/search.spec.ts":          "import { origin } from \"./pages/shop\";\n\ntest(\"finds\", () => origin);\n",
		"src/checkout.ts":             "export function pay(n: number): number {\n  return n;\n}\n",
		"src/search.ts":               "export function find(q: string): string {\n  return q;\n}\n",
		appflows.FlowsDeclarationPath: `{"profile":"` + appflows.FlowsProfile(version) + `","flows":"flows","kinds":[{"path":"lib","kind":"unit"}]}` + "\n",
	}
	intents, err := fixtureIntents("")
	if err != nil {
		return fixture{}, err
	}
	for name, body := range intents {
		files[name] = body
	}
	if err := writeFiles(root, files); err != nil {
		return fixture{}, err
	}
	g := func(stamp int, args ...string) (string, error) {
		command := exec.CommandContext(ctx, git, append([]string{"-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false", "-c", "core.autocrlf=false", "-c", "core.fileMode=true"}, args...)...)
		date := time.Date(2026, 11, 11, 0, stamp, 0, 0, time.UTC).Format(time.RFC3339)
		command.Dir = root
		command.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "HOME=" + root, "PATH=" + os.Getenv("PATH"),
			"GIT_AUTHOR_NAME=Smoke", "GIT_AUTHOR_EMAIL=smoke@example.invalid", "GIT_COMMITTER_NAME=Smoke", "GIT_COMMITTER_EMAIL=smoke@example.invalid",
			"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
		out, err := command.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}
	commit := func(stamp int, message string) (string, error) {
		if _, err := g(stamp, "add", "-A"); err != nil {
			return "", err
		}
		if _, err := g(stamp, "commit", "-q", "-m", message); err != nil {
			return "", err
		}
		return g(stamp, "rev-parse", "HEAD")
	}
	if _, err := g(0, "init", "-q"); err != nil {
		return fixture{}, err
	}
	anchor, err := commit(1, "A: fixture")
	if err != nil {
		return fixture{}, err
	}
	if intents, err = fixtureIntents(anchor); err != nil {
		return fixture{}, err
	}
	if err := writeFiles(root, intents); err != nil {
		return fixture{}, err
	}
	base, err := commit(2, "B: review anchors")
	if err != nil {
		return fixture{}, err
	}
	if err := writeFiles(root, map[string]string{"src/checkout.ts": "export function pay(n: number): number {\n  return n + 1;\n}\n"}); err != nil {
		return fixture{}, err
	}
	head, err := commit(3, "C: the change")
	if err != nil {
		return fixture{}, err
	}
	return fixture{root: root, base: base, head: head, e2e: fixtureE2E}, nil
}

func writeFiles(root string, files map[string]string) error {
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fixtureIntents are the two flows, checkout and search, each linking one
// source and one E2E test, reviewed at anchor ("" for unreviewed).
func fixtureIntents(anchor string) (map[string]string, error) {
	flow := func(id, step, outcome, source, test, key string) appflows.FlowIntent {
		return appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: id, Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
			Steps:    []appflows.FlowStep{{StepID: step, Action: "do " + step}},
			Outcomes: []appflows.FlowOutcome{{OutcomeID: outcome, Behavior: outcome + " visible", Matcher: "toBeVisible", Locator: "#" + outcome, Value: outcome}},
			Variations: []appflows.FlowVariation{{VariationID: id + ".basic", Preconditions: []string{}, Steps: []string{step}, ObservableFacts: []string{},
				Outcomes: []string{outcome}, Projects: []string{"chromium"}}},
			Links: []appflows.FlowLink{
				{From: step, Basis: "declared", Target: appflows.LinkTarget{Type: "source", Path: source}, ReviewedAt: anchor},
				{From: id + ".basic", Basis: "declared", Target: appflows.LinkTarget{Type: "test", Path: test, TestKey: key}, ReviewedAt: anchor},
			}}
	}
	out := map[string]string{}
	for _, intent := range []appflows.FlowIntent{
		flow("checkout", "pay", "paid", "src/checkout.ts", "e2e/checkout.spec.ts", "checkout.spec.ts > pays"),
		flow("search", "query", "listed", "src/search.ts", "e2e/search.spec.ts", "search.spec.ts > finds"),
	} {
		raw, err := json.Marshal(intent)
		if err != nil {
			return nil, err
		}
		out["flows/"+intent.FlowID+".json"] = string(raw) + "\n"
	}
	return out, nil
}

// result is one command's exit code and output.
type result struct {
	code           int
	stdout, stderr string
}

// runFunc runs the candidate under root with args.
type runFunc func(root string, args ...string) result

// planE2E is the part of an affected receipt the smoke reads.
type planE2E struct {
	Plan struct {
		E2E *struct {
			Scope    string `json:"scope"`
			Selected []struct {
				Path string `json:"path"`
			} `json:"selected"`
			Unknown []struct {
				Reason string `json:"reason"`
				Detail string `json:"detail"`
			} `json:"unknown"`
		} `json:"e2e"`
	} `json:"plan"`
}

// reports are the PROD-DOCS-011 flow reports the smoke runs.
var reports = [][]string{{"flows", "status", "--flows", "flows"}, {"flows", "gaps", "--flows", "flows"}}

// checks runs the supported and unsupported fixtures through run and lists
// every difference from the expected output.
func checks(run runFunc, supported, unsupported fixture, unsupportedVersion int) []string {
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }

	// The supported run: the flow join narrows to the flow the change
	// reaches, with no flows unknown, and each report prints a document.
	plan, err := affectedPlan(run, supported)
	switch {
	case err != nil:
		fail("supported: %v", err)
	case plan.Plan.E2E == nil:
		fail("supported: the plan has no e2e member")
	default:
		e2e := plan.Plan.E2E
		var selected []string
		for _, s := range e2e.Selected {
			selected = append(selected, s.Path)
		}
		slices.Sort(selected)
		if e2e.Scope != "BOUNDED" || !slices.Equal(selected, []string{"e2e/checkout.spec.ts"}) || len(e2e.Unknown) != 0 {
			fail("supported: plan.e2e is %s selecting %v with unknowns %v; want BOUNDED selecting [e2e/checkout.spec.ts] with none", e2e.Scope, selected, e2e.Unknown)
		}
	}
	for _, args := range reports {
		r := run(supported.root, args...)
		var document map[string]any
		if r.code != 0 || json.Unmarshal([]byte(r.stdout), &document) != nil || len(document) == 0 {
			fail("supported: corvint %s exited %d without a report: %s", strings.Join(args, " "), r.code, firstLine(r.stderr))
		}
	}

	// The unsupported run: every E2E test with FLOWS_SCHEMA_UNSUPPORTED, and
	// each report refuses with the reason and prints nothing.
	plan, err = affectedPlan(run, unsupported)
	switch {
	case err != nil:
		fail("unsupported: %v", err)
	case plan.Plan.E2E == nil:
		fail("unsupported: the plan has no e2e member, so it plans fewer than all E2E tests")
	default:
		e2e := plan.Plan.E2E
		selected := map[string]bool{}
		for _, s := range e2e.Selected {
			selected[s.Path] = true
		}
		var missing []string
		for _, path := range unsupported.e2e {
			if !selected[path] {
				missing = append(missing, path)
			}
		}
		named := false
		for _, u := range e2e.Unknown {
			if u.Reason == "FLOWS_SCHEMA_UNSUPPORTED" && strings.Contains(u.Detail, "version "+strconv.Itoa(unsupportedVersion)) {
				named = true
			}
		}
		// FULL with nothing listed is "every E2E test"; a listed selection
		// must hold every fixture E2E test.
		if e2e.Scope != "FULL" || len(e2e.Selected) != 0 && len(missing) != 0 {
			fail("unsupported: the unsupported-version fixture plans fewer than all E2E tests: plan.e2e is %s, missing %v", e2e.Scope, missing)
		}
		if !named {
			fail("unsupported: plan.e2e does not name FLOWS_SCHEMA_UNSUPPORTED with version %d: %v", unsupportedVersion, e2e.Unknown)
		}
	}
	for _, args := range reports {
		r := run(unsupported.root, args...)
		if r.code == 0 || r.stdout != "" || !strings.Contains(r.stderr, "FLOWS_SCHEMA_UNSUPPORTED") {
			fail("unsupported: corvint %s exited %d with %d bytes of output; want a non-zero exit naming FLOWS_SCHEMA_UNSUPPORTED and no report: %s", strings.Join(args, " "), r.code, len(r.stdout), firstLine(r.stderr))
		}
	}
	return failures
}

func affectedPlan(run runFunc, f fixture) (planE2E, error) {
	var plan planE2E
	r := run(f.root, "affected", "--base", f.base)
	if r.code != 0 {
		return plan, fmt.Errorf("corvint affected --base %s exited %d: %s", f.base, r.code, firstLine(r.stderr))
	}
	if err := json.Unmarshal([]byte(r.stdout), &plan); err != nil {
		return plan, fmt.Errorf("corvint affected --base %s printed no receipt: %v", f.base, err)
	}
	return plan, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
