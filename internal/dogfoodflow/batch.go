package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// BatchCheckOptions are the inputs of `corvint dogfood batch-check BASE`
// (DCW-V0-039). Verifier runs every constituent `cem status`.
type BatchCheckOptions struct {
	Root     string
	Base     string
	Verifier Runner
}

// The batch bounds are refusals, never silent truncation: a larger batch is
// checked in successive pieces.
const (
	batchCommitLimit      = 1024
	batchConstituentLimit = 256
	batchPathsShown       = 20
)

var sealedCEMPath = regexp.MustCompile(`^\.corvint/changes/([0-9a-f]{40})\.cem\.json$`)

// batchRecovery names the documented recovery of each batch-check failure.
var batchRecovery = map[string]string{
	"sealed-cem-modified":      "a sealed CEM is immutable: restore it with git show BIND:.corvint/change.cem.json > .corvint/changes/BIND.cem.json and commit",
	"sealed-cem-drift":         "restore the sealed file from its bind commit: git show BIND:.corvint/change.cem.json > .corvint/changes/BIND.cem.json and commit",
	"unrecognized-sealed-cem":  "only .corvint/changes/<40-hex bind commit>.cem.json files are sealed CEMs; remove or rename the listed path",
	"unsealed-cem-at-head":     "seal the bound change (make dogfood-seal) or restore BASE's .corvint/change.cem.json before the batch check",
	"constituent-not-merged":   "a sealed CEM names a bind commit HEAD does not contain; merge that change's branch or drop the copied file",
	"constituent-cem-invalid":  "the sealed CEM is unreadable at its bind commit; rebind and reseal that change with the normal loop, then remerge it",
	"constituent-cem-policy":   "rebind and reseal that change with the normal loop on its own branch (dogfood-change BASE=<its base>), then remerge it",
	"unbound-commits":          "bind each listed commit through a sealed change: commit it on its own branch, run dogfood-change, dogfood-check and dogfood-seal there with BASE=<its parent>, and merge that branch instead",
	"merge-resolution-unbound": "redo the merge with git merge -X ours (or -X theirs) so its tree is derivable, then commit the manual resolution as its own sealed change",
	"octopus-merge-unbound":    "merge one branch at a time so each merge tree is derivable from two parents",
}

type batch struct {
	flow
	options      BatchCheckOptions
	stdout       io.Writer
	scratch      string
	objects      string
	constituents map[string]bool
	covered      map[string]bool
	failures     map[string][]string
	order        []string
	derived      map[string]int
	bootstrap    int
}

// BatchCheck verifies a merge batch whose every change is already bound and
// sealed (DCW-V0-039): each sealed CEM the range adds is re-verified against
// its own base and bind commit, every other non-merge commit must be a seal
// commit of one of them, and every merge outside a constituent's range must be
// derivable from its parents. It writes nothing in the repository; what it
// cannot observe is printed as NOT_OBSERVED, never assumed.
func BatchCheck(ctx context.Context, options BatchCheckOptions, stdout, stderr io.Writer) (int, error) {
	run := &batch{
		flow:    flow{ctx: ctx, prefix: "dogfood-batch-check", root: options.Root, stderr: stderr},
		options: options, stdout: stdout,
		constituents: map[string]bool{}, covered: map[string]bool{}, failures: map[string][]string{}, derived: map[string]int{},
	}
	return boundary(ctx, run.run, run.cleanup)
}

func (b *batch) cleanup() {
	if b.scratch != "" {
		_ = os.RemoveAll(b.scratch)
	}
}

func (b *batch) print(format string, values ...any) {
	_, _ = fmt.Fprintf(b.stdout, format, values...)
}

func (b *batch) failure(reason, detail string) {
	if _, seen := b.failures[reason]; !seen {
		b.order = append(b.order, reason)
	}
	b.failures[reason] = append(b.failures[reason], detail)
}

func (b *batch) run() int {
	b.requireRoot()
	base, status := b.git(true, "rev-parse", "--verify", "-q", b.options.Base+"^{commit}")
	if status != 0 {
		b.refuse("base-unavailable")
	}
	b.base = chomp(base)
	b.target = b.gitValue("rev-parse", "HEAD^{commit}")
	b.gitDir = b.gitValue("rev-parse", "--absolute-git-dir")
	b.resolveAnchor()
	if !b.gitSucceeds("merge-base", "--is-ancestor", b.base, b.target) {
		b.refuse("base-not-ancestor-of-head")
	}
	if b.gitValue("status", "--porcelain", "--untracked-files=all") != "" {
		b.refuse("dirty-worktree")
	}
	count, _ := strconv.Atoi(b.gitValue("rev-list", "--count", b.base+".."+b.target))
	if count == 0 {
		b.refuse("empty-range")
	}
	if count > batchCommitLimit {
		b.say("%s: REFUSE range-exceeds-%d-commits commits=%d\n", b.prefix, batchCommitLimit, count)
		b.say("  fix: split the batch: check out each piece's last commit detached and run batch-check with BASE=<the previous piece's last commit>\n")
		b.exit(2)
	}
	sealed := b.sealedConstituents()
	if len(sealed) > batchConstituentLimit {
		b.say("%s: REFUSE constituents-exceed-%d constituents=%d\n", b.prefix, batchConstituentLimit, len(sealed))
		b.say("  fix: split the batch: check out each piece's last commit detached and run batch-check with BASE=<the previous piece's last commit>\n")
		b.exit(2)
	}
	if b.gitRevision(b.base+":.corvint/change.cem.json") != b.gitRevision(b.target+":.corvint/change.cem.json") {
		b.failure("unsealed-cem-at-head", ".corvint/change.cem.json")
	}
	for _, bind := range sealed {
		b.verifyConstituent(bind)
	}
	b.verifyCoverage()
	return b.finish(count, len(sealed))
}

func (b *batch) gitRevision(spec string) string {
	value, status := b.git(true, "rev-parse", "--verify", "-q", spec)
	if status != 0 {
		return ""
	}
	return chomp(value)
}

// sealedConstituents lists the bind commits whose sealed CEMs BASE..HEAD adds;
// a sealed CEM BASE already had must be unchanged.
func (b *batch) sealedConstituents() []string {
	changed := b.gitValue("diff", "--no-renames", "--name-status", b.base, b.target, "--", ".corvint/changes")
	binds := []string{}
	for _, line := range nonEmpty(strings.Split(changed, "\n")) {
		state, path, _ := strings.Cut(line, "\t")
		match := sealedCEMPath.FindStringSubmatch(path)
		switch {
		case match == nil:
			b.failure("unrecognized-sealed-cem", path)
		case state != "A":
			b.failure("sealed-cem-modified", path+" status="+state)
		default:
			binds = append(binds, match[1])
			b.constituents[match[1]] = true
		}
	}
	sort.Strings(binds)
	return binds
}

type sealedMap struct {
	BaseRevision string `json:"baseRevision"`
	Hunks        []struct {
		Disposition string `json:"disposition"`
		Path        string `json:"path"`
		Reason      string `json:"reason"`
		OldRange    struct {
			Count int `json:"count"`
		} `json:"oldRange"`
	} `json:"hunks"`
}

type cemStatus struct {
	State  string `json:"state"`
	Counts struct {
		Total     int `json:"total"`
		Supported int `json:"supported"`
		Unknown   int `json:"unknown"`
	} `json:"counts"`
}

// verifyConstituent re-verifies one sealed CEM against its own base and bind
// commit. The bootstrap allowance is the decision-0055 shape the bind loop
// admitted: an unknown, no-evidence hunk adding a whole file absent at the
// change's base. Each such path is printed; the OCM that admitted it is not
// re-run.
func (b *batch) verifyConstituent(bind string) {
	path := ".corvint/changes/" + bind + ".cem.json"
	if !b.gitSucceeds("merge-base", "--is-ancestor", bind, b.target) {
		b.failure("constituent-not-merged", "bind="+bind)
		return
	}
	sealedBlob := b.gitRevision(b.target + ":" + path)
	if sealedBlob == "" || sealedBlob != b.gitRevision(bind+":.corvint/change.cem.json") {
		b.failure("sealed-cem-drift", "bind="+bind)
		return
	}
	data, status := b.git(true, "cat-file", "blob", sealedBlob)
	var cem sealedMap
	if status != 0 || json.Unmarshal([]byte(data), &cem) != nil || !fullRevision.MatchString(cem.BaseRevision) ||
		!b.gitSucceeds("merge-base", "--is-ancestor", cem.BaseRevision, bind) {
		b.failure("constituent-cem-invalid", "bind="+bind)
		return
	}
	bootstrap := []string{}
	for _, hunk := range cem.Hunks {
		if hunk.Disposition == "unknown" && hunk.Reason == "no-evidence" && hunk.OldRange.Count == 0 &&
			!b.gitSucceeds("cat-file", "-e", cem.BaseRevision+":"+hunk.Path) {
			bootstrap = append(bootstrap, hunk.Path)
		}
	}
	var stdout, stderr bytes.Buffer
	code := b.options.Verifier.Run(b.ctx, b.root, []string{"cem", "status", "--map", path, "--expected-base", cem.BaseRevision,
		"--target", bind, "--max-unknown", strconv.Itoa(len(bootstrap)), "--max-mechanical", "0"}, &stdout, &stderr)
	b.checkpoint()
	var observed cemStatus
	_ = json.Unmarshal(firstLine(stdout.Bytes()), &observed)
	if observed.State == "" {
		observed.State = failureReason(stderr.Bytes(), code)
	}
	b.print("%s: CONSTITUENT bind=%s base=%s hunks=%d supported=%d unknown=%d bootstrapUnknown=%d state=%s\n",
		b.prefix, bind, cem.BaseRevision, observed.Counts.Total, observed.Counts.Supported, observed.Counts.Unknown, len(bootstrap), observed.State)
	for _, added := range bootstrap {
		b.print("  bootstrap-unknown %s\n", added)
	}
	b.bootstrap += len(bootstrap)
	if code != 0 {
		b.failure("constituent-cem-policy", "bind="+bind+" state="+observed.State)
		return
	}
	covered := b.gitValue("rev-list", bind, "^"+cem.BaseRevision, "^"+b.base)
	for _, commit := range nonEmpty(strings.Split(covered, "\n")) {
		b.covered[commit] = true
	}
}

// verifyCoverage requires every commit of the range outside a constituent's
// own range to be that constituent's seal commit or a derivable merge.
func (b *batch) verifyCoverage() {
	listing := b.gitValue("rev-list", "--topo-order", "--reverse", "--parents", b.base+".."+b.target)
	for _, line := range nonEmpty(strings.Split(listing, "\n")) {
		fields := strings.Fields(line)
		commit, parents := fields[0], fields[1:]
		switch {
		case b.covered[commit]:
		case len(parents) == 1:
			if !b.constituents[parents[0]] || !b.isSealCommit(commit) {
				b.failure("unbound-commits", commit)
			}
		case len(parents) > 2:
			b.failure("octopus-merge-unbound", commit)
		default:
			b.deriveMerge(commit, parents[0], parents[1])
		}
	}
}

// deriveMerge accepts a merge whose tree Git reproduces from its two parents
// alone; the manual part of any other merge is unbound content.
func (b *batch) deriveMerge(merge, first, second string) {
	tree := b.gitValue("rev-parse", merge+"^{tree}")
	if tree == b.gitValue("rev-parse", first+"^{tree}") {
		b.derive(merge, "first-parent-tree")
		return
	}
	var attempted string
	for _, strategy := range []struct{ name, option string }{{"ort", ""}, {"ort-ours", "-Xours"}, {"ort-theirs", "-Xtheirs"}} {
		args := []string{"merge-tree", "--write-tree", "--no-messages"}
		if strategy.option != "" {
			args = append(args, strategy.option)
		}
		result, status := b.scratchGit(append(args, first, second)...)
		written := strings.SplitN(result, "\n", 2)[0]
		if status == 0 && written == tree {
			b.derive(merge, strategy.name)
			return
		}
		if attempted == "" && fullRevision.MatchString(written) {
			attempted = written
		}
	}
	detail := merge
	if attempted != "" {
		differing, _ := b.scratchGit("diff-tree", "-r", "--name-only", attempted, tree)
		paths := nonEmpty(strings.Split(chomp(differing), "\n"))
		if len(paths) > batchPathsShown {
			paths = append(paths[:batchPathsShown], fmt.Sprintf("... %d more", len(paths)-batchPathsShown))
		}
		if len(paths) > 0 {
			detail += " paths=" + strings.Join(paths, ",")
		}
	}
	b.failure("merge-resolution-unbound", detail)
}

func (b *batch) derive(merge, derivation string) {
	b.derived[derivation]++
	b.print("%s: MERGE %s derivation=%s\n", b.prefix, merge, derivation)
}

// scratchGit runs a Git command whose new objects land in a private temporary
// object directory outside the repository, which reads the repository's
// objects as an alternate; merge-tree therefore leaves the repository
// unchanged (product invariant 4).
func (b *batch) scratchGit(args ...string) (string, int) {
	b.checkpoint()
	if b.scratch == "" {
		b.objects = b.gitValue("rev-parse", "--path-format=absolute", "--git-path", "objects")
		scratch, err := os.MkdirTemp("", "corvint-batch-check-")
		if err != nil {
			b.refuse("scratch-object-directory-unavailable")
		}
		b.scratch = scratch
	}
	var stdout bytes.Buffer
	command := exec.CommandContext(b.ctx, "git", append([]string{"-C", b.root, "-c", "credential.helper="}, args...)...)
	command.Env = append(gokernel.SanitizedGitEnvironment(), "GIT_OBJECT_DIRECTORY="+b.scratch, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+b.objects)
	command.Stdout = &stdout
	status := exitStatus(command.Run())
	b.checkpoint()
	return stdout.String(), status
}

func (b *batch) finish(commits, constituents int) int {
	derivations := []string{}
	for _, name := range []string{"first-parent-tree", "ort", "ort-ours", "ort-theirs"} {
		derivations = append(derivations, fmt.Sprintf("%s=%d", name, b.derived[name]))
	}
	b.print("%s: SUMMARY base=%s head=%s commits=%d constituents=%d merges %s bootstrapUnknown=%d\n",
		b.prefix, b.base, b.target, commits, constituents, strings.Join(derivations, " "), b.bootstrap)
	if len(b.order) > 0 {
		for _, reason := range b.order {
			details := b.failures[reason]
			b.say("%s: FAIL %s count=%d\n", b.prefix, reason, len(details))
			for _, detail := range details {
				b.say("  %s\n", detail)
			}
			b.say("  fix: %s\n", batchRecovery[reason])
		}
		return 1
	}
	verifier := "unavailable"
	if digest, err := fileSHA256(b.options.Verifier.Path); err == nil {
		verifier = "sha256:" + digest
	}
	b.print("%s: NOTE ocm NOT_REVERIFIED sealed-constituent\n", b.prefix)
	b.print("%s: NOTE composition NOT_OBSERVED merged-change-interaction\n", b.prefix)
	if sided := b.derived["ort-ours"] + b.derived["ort-theirs"]; sided > 0 {
		// A one-sided resolution keeps bound content only, but the dropped
		// side's conflicting hunks are absent from HEAD.
		b.print("%s: NOTE conflict-side-dropped merges=%d\n", b.prefix, sided)
	}
	b.print("%s: NOTE local-outcome NOT_PRODUCED batch-check\n", b.prefix)
	b.print("%s: NOTE verifier-independence NOT_OBSERVED single-verifier verifier=%s\n", b.prefix, verifier)
	b.print("%s: PASS constituents=%d commits=%d\n", b.prefix, constituents, commits)
	return 0
}
