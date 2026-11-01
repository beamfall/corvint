// Package tracemigraterepo binds trace migration to stable Git repository authority.
package tracemigraterepo

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerepopaths"
)

// maximumAncestry is the replay window; a test may lower it to exercise a deep
// history without building one.
var maximumAncestry = 10_000

// migrationOperations bounds the Git operations of one migration: the frozen
// default floor plus operationsPerTrace for every trace file the store admits,
// so a valid store at its maximum never exhausts the budget (LTPM-V0-517).
const migrationOperations = gitrun.DefaultOperations + operationsPerTrace*trace.MaxTraceFiles

// operationsPerTrace is the most Git operations one trace file costs across a
// dry run and an apply, which plans twice: beyond the replay window its name is
// resolved once (one cat-file for its type and tree, one merge-base, both
// cached), and then either its tree is listed once (cached) or, stranded, its
// commit object is checked once (cached). The batched probe (V1-3321) charges
// the same operations: one per object or tree listing it answers from its
// cat-file --batch session, and one for the rev-list that replaces every
// merge-base, so it never needs more than the one-shot path it replaces.
const operationsPerTrace = 3

// batchedProbes selects the V1-3321 batched probe; a test turns it off to
// compare against the one-shot probes it replaces.
var batchedProbes = true

// maximumCommitBytes bounds one commit object read through the probe session;
// a larger commit replays the one-shot probe.
const maximumCommitBytes = 1 << 20

const (
	maximumAncestryBytes  = 2 << 20
	maximumTreeBytes      = 64 << 20
	maximumHistoricalPath = 200_000
)

// Options are the public migrate-traces mode and optional confirmation digest.
type Options struct {
	Apply      bool
	PlanDigest *string
}

// Evaluate plans or applies one trace migration against frozen repository authority.
func Evaluate(ctx context.Context, root string, options Options) (trace.MigrationResult, error) {
	if options.Apply {
		if options.PlanDigest == nil || !lowerHex(*options.PlanDigest, 64) {
			return trace.MigrationResult{}, fmt.Errorf("trace migration apply requires a dry-run plan digest")
		}
	} else if options.PlanDigest != nil {
		return trace.MigrationResult{}, fmt.Errorf("dry-run trace migration does not accept a plan digest")
	}
	authority, stable, closeProbe, err := bindAuthority(ctx, root)
	if err != nil {
		return trace.MigrationResult{}, err
	}
	defer closeProbe()
	if options.Apply {
		plan, err := trace.ApplyMigration(root, authority, *options.PlanDigest, stable)
		if err != nil {
			return trace.MigrationResult{}, err
		}
		return plan.Result(true), nil
	}
	plan, err := trace.PlanMigration(root, authority, stable)
	if err != nil {
		return trace.MigrationResult{}, err
	}
	return plan.Result(false), nil
}

func bindAuthority(ctx context.Context, root string) (trace.MigrationAuthority, func() error, func(), error) {
	index, err := contextindex.Build(ctx, root)
	if err != nil {
		return trace.MigrationAuthority{}, nil, nil, err
	}
	budget := gitrun.NewBudget(migrationOperations, gitrun.DefaultTotalBudget)
	raw, err := runGit(ctx, budget, root, maximumAncestryBytes,
		"log", fmt.Sprintf("--max-count=%d", maximumAncestry+1), "--format=%H %T", "HEAD")
	if err != nil {
		return trace.MigrationAuthority{}, nil, nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	// A deeper history keeps the newest window and resolves any other trace name
	// on its own, so the cost is bounded by the trace store, not the ancestry;
	// a legacy tree name then cannot be proven to map to one commit and refuses
	// with its recovery (LTPM-V0-517, V1-3034).
	truncated := len(lines) > maximumAncestry
	if truncated {
		lines = lines[:maximumAncestry]
	}
	length := objectIDLength(index.ObjectFormat)
	commits := make(map[string]trace.Revision, len(lines))
	trees := make(map[string][]string)
	currentPaths := tracerepopaths.Paths(index)
	for _, line := range lines {
		fields := strings.Split(line, " ")
		if len(fields) != 2 || !lowerHex(fields[0], length) || !lowerHex(fields[1], length) {
			return trace.MigrationAuthority{}, nil, nil, fmt.Errorf("Git ancestry contains malformed commit/tree identity")
		}
		paths := []string(nil)
		if fields[1] == index.Revision {
			paths = currentPaths
		}
		commits[fields[0]] = trace.Revision{TreeRevision: fields[1], TrackedPaths: paths}
		if !truncated {
			trees[fields[1]] = append(trees[fields[1]], fields[0])
		}
	}
	for tree := range trees {
		sort.Strings(trees[tree])
	}
	dirty := append([]string(nil), index.DirtyPaths...)
	authority := trace.MigrationAuthority{
		CommitRevision: index.CommitRevision, TreeRevision: index.Revision,
		ObjectFormat: index.ObjectFormat, DirtyPaths: dirty, ProfileID: index.ProfileID,
		Commits: commits, Trees: trees,
	}
	probe := &objectProbe{ctx: ctx, budget: budget, root: root, length: length, session: &gitrun.Session{},
		objects: map[string]objectInfo{}, reachable: map[string]bool{}, trees: map[string][]string{index.Revision: currentPaths}}
	if truncated {
		authority.AncestryLimit = maximumAncestry
		authority.ReachableCommit = probe.reachableCommit
		if batchedProbes {
			authority.ProbeBatch = probe.batch
		}
	}
	authority.TrackedPaths = probe.trackedPaths
	authority.CommitObject = probe.commitObject
	// The check reads only header fields, so it observes them rather than
	// compiling an index whose sources it would discard. ProfileID is not
	// compared because it is a pure function of the tree: an equal Revision
	// already implies an equal ProfileID.
	stable := func() error {
		current, err := contextindex.Observe(ctx, root)
		if err != nil {
			return err
		}
		if current.CommitRevision != index.CommitRevision || current.Revision != index.Revision ||
			current.ObjectFormat != index.ObjectFormat ||
			!equalStrings(current.DirtyPaths, dirty) {
			return fmt.Errorf("repository identity changed")
		}
		return nil
	}
	return authority, stable, probe.session.Close, nil
}

// objectProbe answers the migration's per-name Git probes. Object types,
// commit trees and tree listings come from one `git cat-file --batch` session
// for the whole migration, and every name beyond the replay window is placed
// against HEAD by one rev-list, instead of one Git process per probe
// (V1-3321). Each answer is reserved against the same operation budget as the
// one-shot command it replaces; a request the session cannot answer as one
// admitted record (a missing object, an oversized one, a failed child) replays
// that one-shot command under the same reservation, which alone classifies the
// failure, so the batched path never changes an outcome.
type objectProbe struct {
	ctx       context.Context
	budget    *gitrun.Budget
	root      string
	length    int
	session   *gitrun.Session
	objects   map[string]objectInfo
	reachable map[string]bool
	trees     map[string][]string
}

type objectInfo struct {
	commit bool
	tree   string
}

// read answers one full object ID through the session within perOp: its type
// and body, or ok=false when the session cannot answer it as one record.
func (p *objectProbe) read(perOp time.Duration, id string, limit int) (string, []byte, bool, error) {
	if !batchedProbes {
		return "", nil, false, nil
	}
	header, body, ok, err := p.session.Read(p.ctx, perOp, gitrun.Options{Env: gitEnvironment()}, gitArgs(p.root, "cat-file", "--batch"), id,
		func(_ string, fields []string, size int) bool { return fields[0] == id && size <= limit })
	if err != nil || !ok {
		return "", nil, false, err
	}
	return strings.Fields(header)[1], body, true, nil
}

// object resolves an object's type and, for a commit, its tree: one reserved
// operation, answered by the session or by the one-shot --batch-check probe.
func (p *objectProbe) object(id string) (objectInfo, error) {
	if info, ok := p.objects[id]; ok {
		return info, nil
	}
	perOp, err := p.budget.ReserveOperation(0)
	if err != nil {
		return objectInfo{}, err
	}
	kind, body, ok, err := p.read(perOp, id, maximumCommitBytes)
	if err != nil {
		return objectInfo{}, err
	}
	var info objectInfo
	if ok {
		if kind == "commit" {
			first, _, _ := strings.Cut(string(body), "\n")
			tree, found := strings.CutPrefix(first, "tree ")
			if !found || !lowerHex(tree, p.length) {
				return objectInfo{}, fmt.Errorf("Git ancestry contains malformed commit/tree identity")
			}
			info = objectInfo{commit: true, tree: tree}
		}
	} else {
		// --batch-check answers each input line: "<id> <type>" for an object,
		// "<input> missing" otherwise; the second line peels the commit to its
		// tree. It reports a missing object on stdout and exits zero, so an
		// error here is a Git failure, never absence.
		raw, err := gitrun.RunReserved(p.ctx, perOp, gitrun.Options{Env: gitEnvironment(), Stdin: []byte(id + "\n" + id + "^{tree}\n"), StdoutLimit: 512},
			gitArgs(p.root, "cat-file", "--batch-check=%(objectname) %(objecttype)")...)
		if err != nil {
			return objectInfo{}, err
		}
		answers := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		if len(answers) != 2 {
			return objectInfo{}, fmt.Errorf("Git ancestry contains malformed commit/tree identity")
		}
		if answers[0] == id+" commit" {
			tree, kind, _ := strings.Cut(answers[1], " ")
			if kind != "tree" || !lowerHex(tree, p.length) {
				return objectInfo{}, fmt.Errorf("Git ancestry contains malformed commit/tree identity")
			}
			info = objectInfo{commit: true, tree: tree}
		}
	}
	p.objects[id] = info
	return info, nil
}

// batch resolves every name outside the replay window before planning: its
// type and tree through the session, then whether each commit is reachable
// from HEAD with one `git rev-list ^HEAD <commits>`, which prints exactly the
// named commits HEAD does not reach. It is best-effort: on any failure, or
// output past maximumAncestryBytes, the names it did not settle keep their
// one-shot merge-base probe.
func (p *objectProbe) batch(names []string) {
	commits := make([]string, 0, len(names))
	for _, id := range names {
		if _, known := p.reachable[id]; known {
			continue
		}
		info, err := p.object(id)
		if err != nil {
			return
		}
		if info.commit {
			commits = append(commits, id)
		}
	}
	if len(commits) == 0 {
		return
	}
	input := "^HEAD\n" + strings.Join(commits, "\n") + "\n"
	raw, err := gitrun.Run(p.ctx, p.budget, gitrun.Options{Env: gitEnvironment(), Stdin: []byte(input), StdoutLimit: maximumAncestryBytes},
		gitArgs(p.root, "rev-list", "--stdin")...)
	if err != nil {
		return
	}
	unreached := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		unreached[line] = true
	}
	for _, id := range commits {
		p.reachable[id] = !unreached[id]
	}
}

// reachableCommit resolves a trace name outside the replay window: a commit
// object that HEAD reaches is reachable, with its tree. The type and tree cost
// one operation; reachability comes from batch, or else from one
// `git merge-base --is-ancestor`, once per name.
func (p *objectProbe) reachableCommit(id string) (trace.Revision, bool, error) {
	info, err := p.object(id)
	if err != nil || !info.commit {
		return trace.Revision{}, false, err
	}
	reachable, known := p.reachable[id]
	if !known {
		if _, err := runGit(p.ctx, p.budget, p.root, 128, "merge-base", "--is-ancestor", id, "HEAD"); err != nil {
			details, ok := cemcode.GitExitFailureDetails(err)
			if !ok || details.ExitCode != 1 {
				return trace.Revision{}, false, err
			}
		} else {
			reachable = true
		}
		p.reachable[id] = reachable
	}
	if !reachable {
		return trace.Revision{}, false, nil
	}
	return trace.Revision{TreeRevision: info.tree}, true, nil
}

// commitObject reports whether an unreachable name is still a commit object.
func (p *objectProbe) commitObject(id string) (bool, error) {
	info, err := p.object(id)
	return info.commit, err
}

// trackedPaths lists a tree's paths as `git ls-tree -r --name-only -z` does,
// under one reserved operation: through the session, reading each subtree
// in order within the one deadline, or by replaying ls-tree itself.
func (p *objectProbe) trackedPaths(tree string) ([]string, error) {
	if cached, ok := p.trees[tree]; ok {
		return append([]string(nil), cached...), nil
	}
	perOp, err := p.budget.ReserveOperation(0)
	if err != nil {
		return nil, err
	}
	paths, ok, err := p.walkTree(time.Now().Add(perOp), tree)
	if err != nil {
		return nil, err
	}
	if !ok {
		raw, err := gitrun.RunReserved(p.ctx, perOp, gitrun.Options{Env: gitEnvironment(), StdoutLimit: maximumTreeBytes + 1},
			gitArgs(p.root, "ls-tree", "-r", "--name-only", "-z", tree)...)
		if err != nil {
			return nil, err
		}
		if len(raw) > maximumTreeBytes {
			return nil, fmt.Errorf("historical trace tree exceeds %d bytes", maximumTreeBytes)
		}
		if bytes.Count(raw, []byte{0}) > maximumHistoricalPath {
			return nil, fmt.Errorf("historical trace tree exceeds %d paths", maximumHistoricalPath)
		}
		paths = make([]string, 0, bytes.Count(raw, []byte{0}))
		for _, path := range bytes.Split(raw, []byte{0}) {
			if len(path) != 0 {
				paths = append(paths, string(path))
			}
		}
	}
	p.trees[tree] = paths
	return append([]string(nil), paths...), nil
}

// walkTree reads tree and its subtrees through the session in Git's tree order,
// so the paths match ls-tree -r. Any record the session cannot answer, a
// malformed entry, or a listing past ls-tree's own bounds returns ok=false and
// the caller replays ls-tree.
func (p *objectProbe) walkTree(deadline time.Time, tree string) ([]string, bool, error) {
	paths := []string{}
	listed := 0
	var walk func(id, prefix string) (bool, error)
	walk = func(id, prefix string) (bool, error) {
		kind, body, ok, err := p.read(time.Until(deadline), id, maximumTreeBytes)
		if err != nil || !ok || kind != "tree" {
			return false, err
		}
		for len(body) != 0 {
			space := bytes.IndexByte(body, ' ')
			end := bytes.IndexByte(body, 0)
			if space <= 0 || end <= space+1 || len(body) < end+1+p.length/2 {
				return false, nil
			}
			mode, name := string(body[:space]), prefix+string(body[space+1:end])
			object := hex.EncodeToString(body[end+1 : end+1+p.length/2])
			body = body[end+1+p.length/2:]
			if mode == "40000" {
				if ok, err := walk(object, name+"/"); err != nil || !ok {
					return false, err
				}
				continue
			}
			listed += len(name) + 1
			paths = append(paths, name)
			if listed > maximumTreeBytes || len(paths) > maximumHistoricalPath {
				return false, nil
			}
		}
		return true, nil
	}
	ok, err := walk(tree, "")
	return paths, ok, err
}

func runGit(ctx context.Context, budget *gitrun.Budget, root string, limit int, arguments ...string) ([]byte, error) {
	return runGitInput(ctx, budget, root, nil, limit, arguments...)
}

func runGitInput(ctx context.Context, budget *gitrun.Budget, root string, input []byte, limit int, arguments ...string) ([]byte, error) {
	return gitrun.Run(ctx, budget, gitrun.Options{Env: gitEnvironment(), Stdin: input, StdoutLimit: limit}, gitArgs(root, arguments...)...)
}

func gitArgs(root string, arguments ...string) []string {
	args := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-C", root,
		"-c", "advice.graftFileDeprecated=false",
	}
	return append(args, arguments...)
}

func gitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull,
		"GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func objectIDLength(format string) int {
	if format == "sha256" {
		return 64
	}
	return 40
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
