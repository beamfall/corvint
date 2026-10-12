package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitstatus"
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
	"constituent-cem-invalid":  "the sealed CEM is unreadable or not a canonical cem/0.2 or cem/0.3 map at its bind commit; rebind and reseal that change with the normal loop, then remerge it",
	"constituent-unverified":   "the verifier did not report canonical ready-for-ci verification of this bind and base; pass --verifier a corvint build of this tree (script/dogfood-batch-check.sh does) and rerun",
	"constituent-cem-policy":   "rebind and reseal that change with the normal loop on its own branch (dogfood-change BASE=<its base>), then remerge it",
	"unbound-commits":          "bind each listed commit through a sealed change: commit it on its own branch, run dogfood-change, dogfood-check and dogfood-seal there with BASE=<its parent>, and merge that branch instead",
	"merge-resolution-unbound": "redo the merge with git merge -X ours (or -X theirs) so its tree is derivable, then commit the manual resolution as its own sealed change",
	"octopus-merge-unbound":    "merge one branch at a time so each merge tree is derivable from two parents",
	// A sealed cem/1.0 map is refused by its own code (CEM-V1-032).
	"constituent-cem-stable-unsupported": "the sealed CEM is a cem/1.0 map, which batch-check does not verify; it covers only Core cem/0.2 or cem/0.3 sealed changes, so rebind and reseal that change with the normal loop, then remerge it",
}

type batch struct {
	flow
	options      BatchCheckOptions
	stdout       io.Writer
	scratch      string
	tempParent   string
	objects      string
	constituents map[string]bool
	covered      map[string]bool
	failures     map[string][]string
	order        []string
	derived      map[string]int
	bootstrap    int
	inherited    inheritedCEM
	archived     bool
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
	b.requireScratchOutside()
	// Core's hardened probe reads status against private metadata and refuses
	// a configuration that could run a repository-owned filter or hook.
	repository, err := gokernel.ProbeRepositoryContext(b.ctx, b.root)
	b.checkpoint()
	if err != nil {
		b.say("%s: REFUSE worktree-status-unavailable\n  %s\n", b.prefix, oneLine(err.Error()))
		b.say("  fix: remove the repository configuration the reason names, or run batch-check in a plain clone (git clone --no-hardlinks)\n")
		b.exit(2)
	}
	if repository.DirtyPathCount != 0 {
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
	b.inherited = b.inheritedCEM()
	// HEAD may keep the archive from BASE itself or from a commit in range.
	b.archived = b.archives(b.inherited, b.target)
	sealed := b.sealedConstituents()
	if len(sealed) > batchConstituentLimit {
		b.say("%s: REFUSE constituents-exceed-%d constituents=%d\n", b.prefix, batchConstituentLimit, len(sealed))
		b.say("  fix: split the batch: check out each piece's last commit detached and run batch-check with BASE=<the previous piece's last commit>\n")
		b.exit(2)
	}
	// HEAD keeps BASE's unsealed CEM in place or archives it (DCW-V0-043).
	if head := b.gitRevision(b.target + ":.corvint/change.cem.json"); head != b.inherited.blob && (head != "" || !b.archived) {
		b.failure("unsealed-cem-at-head", ".corvint/change.cem.json")
	}
	for _, bind := range sealed {
		b.verifyConstituent(bind)
	}
	b.verifyCoverage()
	return b.finish(count, len(sealed))
}

// requireScratchOutside refuses a temporary directory that resolves inside the
// worktree or Git directory, before any scratch file exists, since merge
// derivation and the status probe write their private copies there.
func (b *batch) requireScratchOutside() {
	common := b.gitValue("rev-parse", "--path-format=absolute", "--git-common-dir")
	parent, err := filepath.EvalSymlinks(os.TempDir())
	if err == nil && filepath.IsAbs(parent) {
		err = gitstatus.ScratchOutside(b.ctx, parent, b.root, b.gitDir, common)
	}
	if err != nil {
		b.say("%s: REFUSE scratch-inside-repository\n", b.prefix)
		b.say("  fix: set TMPDIR to an existing directory outside the repository and its Git directory\n")
		b.exit(2)
	}
	b.tempParent = parent
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func (f *flow) gitRevision(spec string) string {
	value, status := f.git(true, "rev-parse", "--verify", "-q", spec)
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
		case path == b.inherited.path && b.archived:
			// BASE's own inherited CEM, archived: bytes BASE already tracks,
			// not a constituent (DCW-V0-043).
		default:
			binds = append(binds, match[1])
			b.constituents[match[1]] = true
		}
	}
	sort.Strings(binds)
	return binds
}

type sealedMap struct {
	Spec         string `json:"spec"`
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
	OK     bool   `json:"ok"`
	Tool   string `json:"tool"`
	State  string `json:"state"`
	Counts struct {
		Total     *int `json:"total"`
		Supported *int `json:"supported"`
		Unknown   *int `json:"unknown"`
	} `json:"counts"`
	Verification struct {
		Valid          bool   `json:"valid"`
		Assurance      string `json:"assurance"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
	} `json:"verification"`
}

// proves reports whether a status response is canonical ready-for-ci
// verification of exactly this bind and base; an exit status alone is not
// evidence.
func (s cemStatus) proves(base, bind string) bool {
	return s.OK && s.Tool == "cem-status" && s.State == "ready-for-ci" && s.Verification.Valid &&
		s.Counts.Total != nil && s.Counts.Supported != nil && s.Counts.Unknown != nil &&
		s.Verification.Assurance == "canonical" && s.Verification.BaseRevision == base && s.Verification.TargetRevision == bind
}

// sealedVerification is one sealed CEM re-verified against its own base and
// bind commit; failure is empty when the verifier proved it.
type sealedVerification struct {
	failure   string
	detail    string
	base      string
	bootstrap []string
	reasoned  int
	observed  cemStatus
	ran       bool
}

// verifySealedCEM re-verifies the sealed CEM of one bind commit that target
// contains, for batch-check (DCW-V0-039) and for a change that merges sealed
// changes (DCW-V0-042). The bootstrap allowance is the decision-0055 shape the
// bind loop admitted: an unknown, no-evidence hunk adding a whole file absent
// at the change's base. An unknown hunk kept with a recorded reason
// (DCW-V0-019) is admitted beside it. The OCM that admitted it is not re-run. Only a
// canonical map is accepted: its verifier derives the patch from the base and
// bind commits, whereas a cem/0.1 map verifies a cached patch that need not
// equal the committed change.
func (f *flow) verifySealedCEM(verifier Runner, target, bind string) sealedVerification {
	path := ".corvint/changes/" + bind + ".cem.json"
	result := sealedVerification{detail: "bind=" + bind}
	if !f.gitSucceeds("merge-base", "--is-ancestor", bind, target) {
		result.failure = "constituent-not-merged"
		return result
	}
	sealedBlob := f.gitRevision(target + ":" + path)
	if sealedBlob == "" || sealedBlob != f.gitRevision(bind+":.corvint/change.cem.json") {
		result.failure = "sealed-cem-drift"
		return result
	}
	data, status := f.git(true, "cat-file", "blob", sealedBlob)
	// CEM-V1-032: a stable map is refused by its declared spec before the
	// loose 0.x decode, so it is never reported as a malformed 0.x map.
	if status == 0 && wire.DeclaresStable([]byte(data)) {
		result.failure = "constituent-cem-stable-unsupported"
		return result
	}
	var cem sealedMap
	if status != 0 || json.Unmarshal([]byte(data), &cem) != nil || !wire.Canonical(cem.Spec) || !fullRevision.MatchString(cem.BaseRevision) ||
		!f.gitSucceeds("merge-base", "--is-ancestor", cem.BaseRevision, bind) {
		result.failure = "constituent-cem-invalid"
		return result
	}
	result.base = cem.BaseRevision
	for _, hunk := range cem.Hunks {
		if hunk.Disposition == "unknown" && hunk.Reason == "no-evidence" && hunk.OldRange.Count == 0 &&
			!f.gitSucceeds("cat-file", "-e", cem.BaseRevision+":"+hunk.Path) {
			result.bootstrap = append(result.bootstrap, hunk.Path)
		}
		if hunk.Disposition == "unknown" && recordedUnknownReasons[hunk.Reason] {
			result.reasoned++
		}
	}
	var stdout, stderr bytes.Buffer
	code := verifier.Run(f.ctx, f.root, []string{"cem", "status", "--map", path, "--expected-base", cem.BaseRevision,
		"--target", bind, "--max-unknown", strconv.Itoa(len(result.bootstrap) + result.reasoned), "--max-mechanical", "0"}, &stdout, &stderr)
	f.checkpoint()
	result.ran = true
	parsed := json.Unmarshal(firstLine(stdout.Bytes()), &result.observed) == nil
	if result.observed.State == "" {
		result.observed.State = failureReason(stderr.Bytes(), code)
	}
	switch {
	case code != 0:
		result.failure, result.detail = "constituent-cem-policy", result.detail+" state="+result.observed.State
	case !parsed || !result.observed.proves(cem.BaseRevision, bind):
		result.failure, result.detail = "constituent-unverified", result.detail+" state="+result.observed.State
	}
	return result
}

// verifyConstituent re-verifies one sealed CEM of the batch and records the
// commits its own range covers.
func (b *batch) verifyConstituent(bind string) {
	result := b.verifySealedCEM(b.options.Verifier, b.target, bind)
	if result.ran {
		observed := result.observed
		b.print("%s: CONSTITUENT bind=%s base=%s hunks=%s supported=%s unknown=%s bootstrapUnknown=%d reasonedUnknown=%d state=%s\n",
			b.prefix, bind, result.base, observedCount(observed.Counts.Total), observedCount(observed.Counts.Supported),
			observedCount(observed.Counts.Unknown), len(result.bootstrap), result.reasoned, observed.State)
		for _, added := range result.bootstrap {
			b.print("  bootstrap-unknown %s\n", added)
		}
		b.bootstrap += len(result.bootstrap)
	}
	if result.failure != "" {
		b.failure(result.failure, result.detail)
		return
	}
	covered := b.gitValue("rev-list", bind, "^"+result.base, "^"+b.base)
	for _, commit := range nonEmpty(strings.Split(covered, "\n")) {
		b.covered[commit] = true
	}
}

func observedCount(value *int) string {
	if value == nil {
		return "NOT_OBSERVED"
	}
	return strconv.Itoa(*value)
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
		case len(parents) == 0:
			b.failure("unbound-commits", commit)
		case len(parents) == 1:
			if (!b.constituents[parents[0]] || !b.isSealCommit(commit)) && !b.archivesInherited(parents[0], commit) {
				b.failure("unbound-commits", commit)
			}
		case len(parents) > 2:
			b.failure("octopus-merge-unbound", commit)
		default:
			b.deriveMerge(commit, parents[0], parents[1])
		}
	}
}

// archivesInherited reports a commit that only archives BASE's inherited CEM
// at its sealed path, by adding that file or renaming the unchanged sidecar
// there (DCW-V0-043).
func (b *batch) archivesInherited(parent, commit string) bool {
	if !b.archived {
		return false
	}
	changed, _ := b.git(false, "diff-tree", "-r", "-M", "--no-commit-id", "--name-status", parent, commit)
	changed = chomp(changed)
	return (changed == "A\t"+b.inherited.path || changed == "R100\t.corvint/change.cem.json\t"+b.inherited.path) &&
		b.archives(b.inherited, commit)
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

// scratchGit runs a Git command in a private temporary bare repository outside
// the checked repository, which reads the repository's objects as an
// alternate. New objects land there, and neither the repository's
// configuration nor its attributes apply: no configured merge driver or other
// command can run or write, and a merge tree is derived from the parents
// alone (product invariant 4).
func (b *batch) scratchGit(args ...string) (string, int) {
	b.checkpoint()
	if b.scratch == "" {
		b.objects = b.gitValue("rev-parse", "--path-format=absolute", "--git-path", "objects")
		scratch, err := os.MkdirTemp(b.tempParent, "corvint-batch-check-")
		if err != nil {
			b.refuse("scratch-repository-unavailable")
		}
		b.scratch = scratch
		if _, status := b.runScratch("init", "-q", "--bare", "--template=", "repository.git"); status != 0 ||
			os.WriteFile(filepath.Join(scratch, "repository.git", "objects", "info", "alternates"), []byte(b.objects+"\n"), 0o600) != nil {
			b.refuse("scratch-repository-unavailable")
		}
	}
	return b.runScratch(append([]string{"--git-dir=repository.git", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.attributesFile=" + os.DevNull}, args...)...)
}

func (b *batch) runScratch(args ...string) (string, int) {
	var stdout bytes.Buffer
	command := exec.CommandContext(b.ctx, "git", append([]string{"-c", "credential.helper="}, args...)...)
	command.Dir = b.scratch
	command.Env = append(gokernel.SanitizedGitEnvironment(), "GIT_ATTR_NOSYSTEM=1", "GIT_CEILING_DIRECTORIES="+filepath.Dir(b.scratch))
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
	if b.archived {
		b.print("%s: NOTE inherited-cem-archived NOT_REVERIFIED path=%s\n", b.prefix, b.inherited.path)
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
