// SPDX-License-Identifier: Apache-2.0
// Command affected-edit-oracle measures controlled changes against independently
// authored expectations. It never changes the selector or writes a source repo.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const profile = "affected-edit-oracle/0"
const outputLimit = 32 << 20

type edit struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Before    string `json:"before,omitempty"`
	Content   string `json:"content,omitempty"`
}
type testCommand struct {
	Argv   []string `json:"argv"`
	Format string   `json:"format"`           // go-json or node-assert
	UnitID string   `json:"unitId,omitempty"` // node assertions have one file/command
}
type mutation struct {
	Edit     edit          `json:"edit"`
	Commands []testCommand `json:"commands"`
}
type testCase struct {
	ID         string    `json:"id"`
	Repo       string    `json:"repo"`
	BaseSHA    string    `json:"baseSha"`
	EditClass  string    `json:"editClass"`
	Edit       edit      `json:"edit"`
	MustSelect []string  `json:"mustSelect"`
	MaySelect  []string  `json:"maySelect"` // exact IDs or witness:KIND
	Why        string    `json:"why"`
	Mutation   *mutation `json:"mutation,omitempty"`
}
type corpus struct {
	Profile string     `json:"profile"`
	Cases   []testCase `json:"cases"`
}
type witness struct {
	Kind      string `json:"kind"`
	DirtyPath string `json:"dirtyPath"`
	ViaLength int    `json:"viaLength"`
}
type selection struct {
	Kind    string  `json:"kind,omitempty"`
	UnitID  string  `json:"unitId"`
	Witness witness `json:"witness"`
}
type receipt struct {
	OK   bool `json:"ok"`
	Plan *struct {
		Selected []selection       `json:"selected"`
		Unknown  []json.RawMessage `json:"unknown"`
	} `json:"plan"`
	Summary struct {
		Line string `json:"line"`
	} `json:"summary"`
}
type comparison struct {
	Selected  int            `json:"selected"`
	MustHit   int            `json:"mustHit"`
	Missing   []string       `json:"missing"`
	Over      []selection    `json:"over"`
	ByWitness map[string]int `json:"overByWitness"`
}
type safety struct {
	Status       string   `json:"status"`
	FailingUnits []string `json:"failingUnits,omitempty"`
	FailingTests []string `json:"failingTests,omitempty"`
	Missing      []string `json:"missing,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Detail       string   `json:"detail,omitempty"`
}
type row struct {
	ID         string     `json:"id"`
	Repo       string     `json:"repo"`
	BaseSHA    string     `json:"baseSha"`
	Status     string     `json:"status"`
	Summary    string     `json:"summary"`
	Unknown    int        `json:"unknown"`
	Must       int        `json:"must"`
	Comparison comparison `json:"comparison"`
	Safety     safety     `json:"safety"`
	Error      string     `json:"error,omitempty"`
}
type report struct {
	Profile       string         `json:"profile"`
	CorpusSHA256  string         `json:"corpusSha256"`
	BinarySHA256  string         `json:"binarySha256"`
	Cases         []row          `json:"cases"`
	Totals        map[string]int `json:"totals"`
	OverByWitness map[string]int `json:"overByWitness"`
}
type config struct {
	Root, Workspace, Clones, Binary string
	Mutate                          bool
}

// compare is deliberately independent of the affected implementation. AEO-V0-003.
func compare(c testCase, selected []selection) comparison {
	out := comparison{Missing: []string{}, Over: []selection{}, ByWitness: map[string]int{}}
	seen := map[string]bool{}
	ordered := slices.Clone(selected)
	slices.SortFunc(ordered, func(a, b selection) int {
		if n := strings.Compare(a.UnitID, b.UnitID); n != 0 {
			return n
		}
		if n := strings.Compare(a.Witness.Kind, b.Witness.Kind); n != 0 {
			return n
		}
		return strings.Compare(a.Witness.DirtyPath, b.Witness.DirtyPath)
	})
	for _, s := range ordered {
		if seen[s.UnitID] {
			continue
		}
		seen[s.UnitID] = true
		out.Selected++
		if !slices.Contains(c.MustSelect, s.UnitID) && !slices.Contains(c.MaySelect, s.UnitID) && !slices.Contains(c.MaySelect, "witness:"+s.Witness.Kind) {
			out.Over = append(out.Over, s)
			out.ByWitness[s.Witness.Kind]++
		}
	}
	for _, id := range c.MustSelect {
		if seen[id] {
			out.MustHit++
		} else {
			out.Missing = append(out.Missing, id)
		}
	}
	slices.Sort(out.Missing)
	return out
}

func validate(c corpus) error {
	if c.Profile != profile || len(c.Cases) == 0 || len(c.Cases) > 256 {
		return errors.New("expected affected-edit-oracle/0 with 1..256 cases")
	}
	ids := map[string]bool{}
	for _, tc := range c.Cases {
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(tc.ID) || ids[tc.ID] {
			return fmt.Errorf("invalid/duplicate case id %q", tc.ID)
		}
		ids[tc.ID] = true
		if tc.Repo != "corvint" && !regexp.MustCompile(`^beamfall-workspace/[a-z0-9-]+$`).MatchString(tc.Repo) {
			return fmt.Errorf("%s: invalid repo", tc.ID)
		}
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(tc.BaseSHA) || tc.Why == "" || tc.EditClass == "" || tc.MustSelect == nil || tc.MaySelect == nil {
			return fmt.Errorf("%s: missing pin, rationale, class or expectations", tc.ID)
		}
		for _, list := range [][]string{tc.MustSelect, tc.MaySelect} {
			seen := map[string]bool{}
			for _, id := range list {
				if id == "" || seen[id] {
					return fmt.Errorf("%s: empty/duplicate expectation", tc.ID)
				}
				seen[id] = true
			}
		}
		for _, id := range tc.MustSelect {
			if strings.HasPrefix(id, "witness:") {
				return fmt.Errorf("%s: mustSelect requires unit IDs", tc.ID)
			}
		}
		if tc.Edit.Operation == "none" && (tc.EditClass != "empty" || tc.Edit.Path != "" || len(tc.MustSelect)+len(tc.MaySelect) != 0) {
			return fmt.Errorf("%s: empty edit must select zero", tc.ID)
		}
		if err := validateEdit(tc.Edit); err != nil {
			return fmt.Errorf("%s: %w", tc.ID, err)
		}
		if m := tc.Mutation; m != nil {
			if m.Edit.Operation != "replace-literal" || tc.Edit.Operation != "replace-literal" || m.Edit.Path != tc.Edit.Path || m.Edit.Before != tc.Edit.Before || m.Edit.Content == tc.Edit.Content || len(m.Commands) == 0 {
				return fmt.Errorf("%s: mutation must break the same literal site and name commands", tc.ID)
			}
			if err := validateEdit(m.Edit); err != nil {
				return err
			}
			for _, cmd := range m.Commands {
				if len(cmd.Argv) == 0 || (cmd.Format != "go-json" && cmd.Format != "node-assert") || (cmd.Format == "node-assert" && cmd.UnitID == "") {
					return fmt.Errorf("%s: invalid mutation command", tc.ID)
				}
			}
		}
	}
	return nil
}
func validateEdit(e edit) error {
	if e.Operation == "none" && e.Path == "" {
		return nil
	}
	if e.Path == "" || !filepath.IsLocal(e.Path) || filepath.ToSlash(filepath.Clean(e.Path)) != e.Path || strings.Contains(e.Path, "\\") {
		return errors.New("edit needs canonical local path")
	}
	for _, part := range strings.Split(e.Path, "/") {
		if strings.EqualFold(part, ".git") {
			return errors.New("editing Git metadata forbidden")
		}
	}
	switch e.Operation {
	case "append-line", "add-file":
		if e.Content == "" {
			return errors.New("empty edit content")
		}
	case "delete-file":
	case "replace-literal":
		if e.Before == "" || e.Before == e.Content {
			return errors.New("replacement must change a nonempty literal")
		}
	default:
		return fmt.Errorf("unsupported edit operation %q", e.Operation)
	}
	return nil
}

func apply(root string, e edit) error {
	if err := validateEdit(e); err != nil {
		return err
	}
	if e.Operation == "none" {
		return nil
	}
	// Refuse symlinks in every ancestor, including an existing final component.
	at := root
	for _, part := range strings.Split(e.Path, "/") {
		at = filepath.Join(at, part)
		st, err := os.Lstat(at)
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink edit refused")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	name := filepath.Join(root, filepath.FromSlash(e.Path))
	if e.Operation == "add-file" {
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = f.WriteString(e.Content)
		return errors.Join(err, f.Close())
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	switch e.Operation {
	case "delete-file":
		return os.Remove(name)
	case "append-line":
		body = append(body, []byte("\n"+e.Content+"\n")...)
	case "replace-literal":
		if bytes.Count(body, []byte(e.Before)) != 1 {
			return fmt.Errorf("%s: expected exactly one replacement site", e.Path)
		}
		body = bytes.Replace(body, []byte(e.Before), []byte(e.Content), 1)
	}
	return os.WriteFile(name, body, 0600)
}

// Bounded synchronized capture also works when a child writes both streams.
type capture struct {
	mu       sync.Mutex
	body     []byte
	overflow bool
}

func (b *capture) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	space := outputLimit - len(b.body)
	if n > space {
		b.overflow = true
		p = p[:space]
	}
	b.body = append(b.body, p...)
	return n, nil
}
func command(ctx context.Context, root string, timeout time.Duration, argv ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = root
	// Ambient Git pointers must not redirect a clone operation to the source index.
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if strings.HasPrefix(key, "GIT_") || slices.Contains([]string{"GOFLAGS", "GOWORK", "GOTOOLCHAIN", "LC_ALL", "HEAVY_REMOTE", "HEAVY_LOCAL_REASON", "HEAVY_REMOTE_HOST", "HEAVY_REMOTE_HOSTS", "HEAVY_NICE", "HEAVY_SELECTION", "HEAVY_SELECTION_REASON"}, key) {
			continue
		}
		c.Env = append(c.Env, v)
	}
	c.Env = append(c.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_DATE=2026-11-08T00:00:00Z", "GIT_COMMITTER_DATE=2026-11-08T00:00:00Z", "LC_ALL=C", "GOTOOLCHAIN=local", "GOWORK=off", "HEAVY_REMOTE=0", "HEAVY_LOCAL_REASON=owner forbids network writes; local controlled experiment", "HEAVY_REMOTE_HOST=", "HEAVY_REMOTE_HOSTS=", "HEAVY_NICE=0", "HEAVY_SELECTION=manual", "HEAVY_SELECTION_REASON=controlled edit oracle single-package mutation")
	var out capture
	c.Stdout = &out
	c.Stderr = &out
	c.WaitDelay = 5 * time.Second
	owner, err := groupreap.Start(c)
	if err != nil {
		return nil, err
	}
	select {
	case <-owner.Exited():
	case <-ctx.Done():
		owner.Stop()
	}
	retirement, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	result := owner.Finish(retirement.Done())
	if result.State != groupreap.Released {
		return out.body, fmt.Errorf("process cleanup unproved: %w", result.Err)
	}
	if ctx.Err() != nil {
		return out.body, ctx.Err()
	}
	if out.overflow {
		return nil, errors.New("command output exceeded 32 MiB")
	}
	return out.body, result.WaitErr
}
func git(ctx context.Context, root string, args ...string) (string, error) {
	argv := append([]string{"git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Edit Oracle", "-c", "user.email=oracle@invalid"}, args...)
	out, err := command(ctx, root, 2*time.Minute, argv...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func commit(ctx context.Context, root string, expected edit) error {
	if _, err := git(ctx, root, "add", "--all"); err != nil {
		return err
	}
	if _, err := git(ctx, root, "commit", "--quiet", "--allow-empty", "-m", "controlled edit"); err != nil {
		return err
	}
	paths, err := git(ctx, root, "diff", "--name-only", "HEAD~1", "HEAD")
	if err != nil {
		return err
	}
	if paths != expected.Path {
		return fmt.Errorf("committed paths %q differ from edit path %q (possibly ignored file)", paths, expected.Path)
	}
	return nil
}
func plan(ctx context.Context, root, bin string) (receipt, error) {
	out, err := command(ctx, root, 2*time.Minute, bin, "affected", "--base", "HEAD~1", "--format", "json")
	if err != nil {
		return receipt{}, fmt.Errorf("affected: %w: %s", err, out)
	}
	var p receipt
	if err = json.Unmarshal(out, &p); err != nil {
		return p, fmt.Errorf("decode affected: %w", err)
	}
	if !p.OK || p.Plan == nil || p.Plan.Selected == nil || p.Plan.Unknown == nil || p.Summary.Line == "" {
		return p, errors.New("incomplete affected receipt")
	}
	for _, s := range p.Plan.Selected {
		if s.UnitID == "" || s.Witness.Kind == "" {
			return p, errors.New("selection missing identity/witness")
		}
	}
	return p, nil
}

func executeCase(ctx context.Context, cfg config, tc testCase) (r row) {
	r = row{ID: tc.ID, Repo: tc.Repo, BaseSHA: tc.BaseSHA, Status: "ERROR", Must: len(tc.MustSelect), Safety: safety{Status: "NOT_RUN"}}
	err := func() error {
		source := cfg.Root
		if tc.Repo != "corvint" {
			source = filepath.Join(cfg.Workspace, strings.TrimPrefix(tc.Repo, "beamfall-workspace/"))
		}
		source, err := filepath.EvalSymlinks(source)
		if err != nil {
			return err
		}
		cloneParent, err := filepath.EvalSymlinks(cfg.Clones)
		if err != nil {
			return err
		}
		if rel, err := filepath.Rel(source, cloneParent); err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return errors.New("clone parent must be outside source repository")
		}
		root := filepath.Join(cloneParent, tc.ID)
		if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clone destination already exists or is unreadable: %s", root)
		}
		// A failed clone may still leave a directory; only this fresh case path is owned.
		defer func() {
			if err := os.RemoveAll(root); err != nil {
				r.Status = "ERROR"
				r.Error = "clone cleanup: " + err.Error()
			}
		}()
		if _, err := git(ctx, cfg.Clones, "clone", "--quiet", "--no-hardlinks", "--shared", source, root); err != nil {
			return err
		}
		if _, err := git(ctx, root, "checkout", "--quiet", "--detach", tc.BaseSHA); err != nil {
			return err
		}
		head, err := git(ctx, root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if head != tc.BaseSHA {
			return errors.New("clone pin mismatch")
		}
		if err = apply(root, tc.Edit); err != nil {
			return err
		}
		if err = commit(ctx, root, tc.Edit); err != nil {
			return err
		}
		p, err := plan(ctx, root, cfg.Binary)
		if err != nil {
			return err
		}
		r.Summary = p.Summary.Line
		r.Unknown = len(p.Plan.Unknown)
		r.Comparison = compare(tc, p.Plan.Selected)
		r.Status = "PASS"
		if len(r.Comparison.Over) > 0 {
			r.Status = "OVER"
		}
		if len(r.Comparison.Missing) > 0 {
			r.Status = "FAIL"
		}
		if cfg.Mutate && tc.Mutation != nil {
			r.Safety = runMutation(ctx, cfg, root, tc)
		}
		return nil
	}()
	if err != nil {
		r.Error = err.Error()
		r.Status = "ERROR"
	}
	return r
}

type observation struct{ Action, Package, Test string }

func testObservations(cmd testCommand, out []byte) (passes int, units, tests []string, err error) {
	if cmd.Format == "node-assert" {
		if bytes.Contains(out, []byte("ERR_ASSERTION")) {
			return 0, []string{cmd.UnitID}, []string{cmd.UnitID}, nil
		}
		// A successful direct assertion script has no test protocol; its exit status
		// establishes completion. Failed scripts still need ERR_ASSERTION below.
		return 1, nil, nil, nil
	}
	unitSet := map[string]bool{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) == 0 || line[0] != '{' {
			continue
		} // heavy admission diagnostics
		var o observation
		if json.Unmarshal(line, &o) != nil {
			return 0, nil, nil, errors.New("malformed Go test event")
		}
		if o.Test == "" {
			continue
		}
		if o.Action == "pass" {
			passes++
		}
		if o.Action == "fail" {
			if o.Package == "" {
				return 0, nil, nil, errors.New("Go failure missing package")
			}
			id := "go:" + o.Package
			unitSet[id] = true
			tests = append(tests, id+"#"+o.Test)
		}
	}
	for id := range unitSet {
		units = append(units, id)
	}
	slices.Sort(units)
	slices.Sort(tests)
	return passes, units, slices.Compact(tests), nil
}
func runMutation(ctx context.Context, cfg config, root string, tc testCase) (s safety) {
	s.Status = "ERROR"
	fail := func(err error) safety { s.Detail = err.Error(); return s }
	if _, err := git(ctx, root, "reset", "--hard", tc.BaseSHA); err != nil {
		return fail(err)
	}
	if _, err := git(ctx, root, "clean", "-fdx"); err != nil {
		return fail(err)
	}
	// Baseline before injection prevents missing dependencies or fixture failures
	// from masquerading as mutation killers (AEO-V0-004).
	for _, cmd := range tc.Mutation.Commands {
		out, err := command(ctx, root, 2*time.Minute, cmd.Argv...)
		passes, failed, _, parseErr := testObservations(cmd, out)
		if err != nil || parseErr != nil || passes == 0 || len(failed) > 0 {
			return fail(fmt.Errorf("baseline test did not pass: %v; %v; %s", err, parseErr, out))
		}
	}
	// Remove outputs of baseline tests before constructing the mutation commit.
	if _, err := git(ctx, root, "reset", "--hard", tc.BaseSHA); err != nil {
		return fail(err)
	}
	if _, err := git(ctx, root, "clean", "-fdx"); err != nil {
		return fail(err)
	}
	if err := apply(root, tc.Mutation.Edit); err != nil {
		return fail(err)
	}
	if err := commit(ctx, root, tc.Mutation.Edit); err != nil {
		return fail(err)
	}
	p, err := plan(ctx, root, cfg.Binary)
	if err != nil {
		return fail(err)
	}
	s.Summary = p.Summary.Line
	for _, cmd := range tc.Mutation.Commands {
		out, runErr := command(ctx, root, 2*time.Minute, cmd.Argv...)
		_, failed, tests, parseErr := testObservations(cmd, out)
		if parseErr != nil {
			return fail(parseErr)
		}
		if runErr == nil {
			s.Status = "SURVIVED"
			return fail(errors.New("mutation command passed; no kill observed"))
		}
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() != 1 || len(failed) == 0 {
			return fail(fmt.Errorf("mutation did not produce an assertion failure: %v; %s", runErr, out))
		}
		s.FailingUnits = append(s.FailingUnits, failed...)
		s.FailingTests = append(s.FailingTests, tests...)
	}
	slices.Sort(s.FailingUnits)
	s.FailingUnits = slices.Compact(s.FailingUnits)
	slices.Sort(s.FailingTests)
	s.FailingTests = slices.Compact(s.FailingTests)
	c := compare(testCase{MustSelect: s.FailingUnits}, p.Plan.Selected)
	s.Missing = c.Missing
	s.Status = "PASS"
	if len(s.Missing) > 0 {
		s.Status = "SAFETY_FAIL"
	}
	return s
}
func summarize(rows []row) (map[string]int, map[string]int) {
	totals := map[string]int{"cases": len(rows), "selected": 0, "mustHit": 0, "must": 0, "over": 0, "missing": 0, "PASS": 0, "OVER": 0, "FAIL": 0, "ERROR": 0, "SAFETY_PASS": 0, "SAFETY_FAIL": 0, "SAFETY_ERROR": 0}
	witnesses := map[string]int{}
	for _, r := range rows {
		totals[r.Status]++
		totals["selected"] += r.Comparison.Selected
		totals["mustHit"] += r.Comparison.MustHit
		totals["must"] += r.Must
		totals["over"] += len(r.Comparison.Over)
		totals["missing"] += len(r.Comparison.Missing)
		for k, n := range r.Comparison.ByWitness {
			witnesses[k] += n
		}
		switch r.Safety.Status {
		case "PASS":
			totals["SAFETY_PASS"]++
		case "SAFETY_FAIL":
			totals["SAFETY_FAIL"]++
		case "ERROR", "SURVIVED":
			totals["SAFETY_ERROR"]++
		}
	}
	return totals, witnesses
}
func rejected(rows []row, allowOver bool) bool {
	for _, r := range rows {
		if r.Status == "FAIL" || r.Status == "ERROR" || (r.Status == "OVER" && !allowOver) || (r.Safety.Status != "PASS" && r.Safety.Status != "NOT_RUN") {
			return true
		}
	}
	return false
}
func sha(body []byte) string { h := sha256.Sum256(body); return hex.EncodeToString(h[:]) }
func boundedRead(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("input too large")
	}
	return b, e
}
func run(ctx context.Context, args []string, w io.Writer) int {
	fs := flag.NewFlagSet("affected-edit-oracle", flag.ContinueOnError)
	fs.SetOutput(w)
	var cfg config
	file := fs.String("corpus", "tools/affected-edit-oracle/testdata/corpus.json", "versioned corpus file")
	fs.StringVar(&cfg.Root, "root", ".", "Corvint source checkout (read only)")
	fs.StringVar(&cfg.Workspace, "workspace", "/Users/russelllewis/projects/beamfall-workspace", "Beamfall source parent (read only)")
	fs.StringVar(&cfg.Clones, "clones", "", "required existing private clone parent; each case is removed after execution")
	fs.StringVar(&cfg.Binary, "corvint", "corvint", "installed planner binary path or PATH name")
	fs.BoolVar(&cfg.Mutate, "mutate", false, "run heavy baseline and breaking-mutation tests")
	allow := fs.Bool("allow-over", false, "permit OVER only; FAIL and SAFETY_FAIL still exit nonzero")
	output := fs.String("out", "", "write deterministic JSON report (default stdout)")
	fs.Usage = func() {
		fmt.Fprintln(w, "Usage: affected-edit-oracle --clones DIR [--corpus FILE] [--corvint BIN] [--mutate] [--allow-over] [--out FILE]\nExample: affected-edit-oracle --clones /private/tmp/oracle-clones --out /private/tmp/oracle.json")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	fatal := func(err error) int { fmt.Fprintln(w, err); return 2 }
	if fs.NArg() != 0 || cfg.Clones == "" {
		fs.Usage()
		return 2
	}
	for _, p := range []*string{&cfg.Root, &cfg.Workspace, &cfg.Clones} {
		abs, e := filepath.Abs(*p)
		if e != nil {
			return fatal(e)
		}
		*p = abs
	}
	st, err := os.Stat(cfg.Clones)
	if err != nil || !st.IsDir() {
		return fatal(errors.New("--clones must be an existing private directory"))
	}
	cfg.Binary, err = exec.LookPath(cfg.Binary)
	if err != nil {
		return fatal(err)
	}
	cfg.Binary, err = filepath.Abs(cfg.Binary)
	if err != nil {
		return fatal(err)
	}
	body, err := boundedRead(*file, 4<<20)
	if err != nil {
		return fatal(err)
	}
	var c corpus
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return fatal(err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fatal(errors.New("trailing corpus data"))
	}
	if err = validate(c); err != nil {
		return fatal(err)
	}
	binary, err := boundedRead(cfg.Binary, 256<<20)
	if err != nil {
		return fatal(err)
	}
	result := report{Profile: profile, CorpusSHA256: sha(body), BinarySHA256: sha(binary), Cases: []row{}}
	slices.SortFunc(c.Cases, func(a, b testCase) int { return strings.Compare(a.ID, b.ID) })
	for _, tc := range c.Cases {
		r := row{ID: tc.ID, Repo: tc.Repo, BaseSHA: tc.BaseSHA, Must: len(tc.MustSelect), Status: "ERROR", Safety: safety{Status: "NOT_RUN"}}
		if ctx.Err() != nil {
			r.Error = "not run: " + ctx.Err().Error()
		} else {
			r = executeCase(ctx, cfg, tc)
		}
		result.Cases = append(result.Cases, r)
		if *output != "" {
			fmt.Fprintf(w, "%s %s selected=%d must=%d/%d over=%d safety=%s\n", r.ID, r.Status, r.Comparison.Selected, r.Comparison.MustHit, r.Must, len(r.Comparison.Over), r.Safety.Status)
			for _, s := range r.Comparison.Over {
				fmt.Fprintf(w, "  OVER %s %s %s\n", s.UnitID, s.Witness.Kind, s.Witness.DirtyPath)
			}
		}
	}
	result.Totals, result.OverByWitness = summarize(result.Cases)
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fatal(err)
	}
	encoded = append(encoded, '\n')
	if *output == "" {
		_, err = w.Write(encoded)
	} else {
		err = os.WriteFile(*output, encoded, 0600)
	}
	if err != nil {
		return fatal(err)
	}
	if rejected(result.Cases, *allow) {
		return 1
	}
	return 0
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(code)
}
