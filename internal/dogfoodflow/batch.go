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
	"slices"
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

// retiredSeal is verifySealedCEM's result for a sealed cem/0.x map that still
// equals its bind commit's sidecar (CEM-V1-203). It is not a failure reason:
// batch-check and dogfood change admit it unverified, with no coverage.
const retiredSeal = "constituent-retired-spec"

var sealedCEMPath = regexp.MustCompile(`^\.corvint/changes/([0-9a-f]{40})\.cem\.json$`)

// batchRecovery names the documented recovery of each batch-check failure.
var batchRecovery = map[string]string{
	"sealed-cem-modified":           "a sealed CEM is immutable: restore it with git show BIND:.corvint/change.cem.json > .corvint/changes/BIND.cem.json and commit",
	"sealed-cem-drift":              "restore the sealed file from its bind commit: git show BIND:.corvint/change.cem.json > .corvint/changes/BIND.cem.json and commit",
	"unrecognized-sealed-cem":       "only .corvint/changes/<40-hex bind commit>.cem.json files are sealed CEMs; remove or rename the listed path",
	"unsealed-cem-at-head":          "seal the bound change (make dogfood-seal) or restore BASE's .corvint/change.cem.json before the batch check",
	"constituent-not-merged":        "a sealed CEM names a bind commit HEAD does not contain; merge that change's branch or drop the copied file",
	"constituent-cem-invalid":       "the sealed CEM is unreadable or not a cem/1.0 map at its bind commit; rebind and reseal that change with the normal loop, then remerge it",
	"constituent-unverified":        "the verifier did not report a stable ACCEPT of this bind and base; pass --verifier a corvint build of this tree (script/dogfood-batch-check.sh does) and rerun",
	"sealed-cem-migration-reverted": "HEAD holds a sealed CEM a migration commit in the range rewrote, with bytes that commit did not write; restore the migration's bytes with git show <migration commit>:PATH > PATH and commit, or drop the merge that reverted it",
	"constituent-migrated":          "the migration commit rewrote this sealed CEM after the batch's BASE added it; split the batch at the migration commit: batch-check up to its parent, then rerun with BASE=<the migration commit's parent>",
	"constituent-cem-policy":        "rebind and reseal that change with the normal loop on its own branch (dogfood-change BASE=<its base>), then remerge it",
	"unbound-commits":               "bind each listed commit through a sealed change: commit it on its own branch, run dogfood-change, dogfood-check and dogfood-seal there with BASE=<its parent>, and merge that branch instead",
	"merge-resolution-unbound":      "a hand-resolved merge is unbound: drop it, rebase the conflicting change's own work commits (not its old bind and seal commits) onto the merge's first parent, resolve each conflict there, bind, check and seal it with BASE=<that first parent>, then merge it with plain git merge --no-ff so ort derives the tree; -X ours or -X theirs drops the other side's conflicting hunks and fails merge-side-dropped",
	"merge-side-dropped":            "restore each listed hunk's dropped side through a sealed change merged after this merge, or, for a hunk dropped on purpose, add the line Corvint-Dropped-Hunk: <id> <reason> to that merge's message; otherwise redo the merge as the merge-resolution-unbound recovery says",
	"octopus-merge-unbound":         "merge one branch at a time so each merge tree is derivable from two parents",
}

// batchCodeRecovery names the recovery of a constituent verifier reason code
// that lies outside the sealed change, printed after the failure's own fix
// line (DCW-V0-111).
var batchCodeRecovery = map[string]string{
	"unsupported-object-alternates": "the clone borrows objects through .git/objects/info/alternates (git clone --reference or --shared), not a fault of the sealed change; check in a clone made with git clone --no-local, or run git repack -a -d, delete .git/objects/info/alternates and .git/objects/info/commit-graphs, run git commit-graph write --reachable, then rerun batch-check",
}

// verifierCode is the shape of a reason code the batch check repeats from the
// verifier's output; anything else is not printed.
var verifierCode = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

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
	// bases maps each verified constituent's bind commit to its base, the
	// bound changes that can restore a dropped hunk (DCW-V0-073).
	bases   map[string]string
	dropped droppedTally
	// codeFixes lists, per failure reason, the verifier reason codes with a
	// recovery of their own (DCW-V0-111).
	codeFixes map[string][]string
	// forward is the merge whose merged parent holds every unbound commit,
	// and that parent (DCW-V0-110).
	forward [2]string
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
		bases:     map[string]string{},
		codeFixes: map[string][]string{},
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
	return b.finish(count, len(b.constituents))
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
// a sealed CEM BASE already had must be unchanged, or changed only by a
// migration commit's rewrite, which HEAD must keep (DCW-V0-050).
func (b *batch) sealedConstituents() []string {
	changed := b.gitValue("diff", "--no-renames", "--name-status", b.base, b.target, "--", ".corvint/changes")
	binds := []string{}
	migrated, reverted := b.migratedSeals(b.base, b.target)
	admitted := 0
	for _, path := range reverted {
		b.failure("sealed-cem-migration-reverted", path)
	}
	for _, line := range nonEmpty(strings.Split(changed, "\n")) {
		state, path, _ := strings.Cut(line, "\t")
		match := sealedCEMPath.FindStringSubmatch(path)
		switch {
		case match == nil:
			b.failure("unrecognized-sealed-cem", path)
		case state == "M" && migrated[path]:
			admitted++
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
	if admitted > 0 {
		b.print("%s: NOTE sealed-cem-migrated maps=%d commit=%s\n", b.prefix, admitted, b.migrationNote(b.base, b.target))
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
	// StableVerification is the stable verifier result of a cem/1.0 status
	// (CEM-V1-017, CEM-V1-148).
	StableVerification *struct {
		Outcome        string  `json:"outcome"`
		Code           string  `json:"code"`
		Accept         *bool   `json:"accept"`
		ExpectedBase   *string `json:"expectedBase"`
		TargetRevision *string `json:"targetRevision"`
	} `json:"stableVerification"`
}

// proves reports whether a status response is the stable verifier's ACCEPT of
// exactly this bind and base with its policy counts; an exit status alone is
// not evidence, and a 0.x ready-for-ci line proves nothing (CEM-V1-202).
func (s cemStatus) proves(base, bind string) bool {
	counted := s.Counts.Total != nil && s.Counts.Supported != nil && s.Counts.Unknown != nil
	stable := s.StableVerification
	return stable != nil && s.OK && s.Tool == "cem-status" && s.State == "accepted" && counted &&
		stable.Outcome == "ACCEPT" && stable.Accept != nil && *stable.Accept &&
		stable.ExpectedBase != nil && *stable.ExpectedBase == base &&
		stable.TargetRevision != nil && *stable.TargetRevision == bind
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
	// code is the verifier's reason code of a failed constituent, when its
	// output names one (DCW-V0-111).
	code string
}

// verifySealedCEM re-verifies the sealed CEM of one bind commit that target
// contains, for batch-check (DCW-V0-039) and for a change that merges sealed
// changes (DCW-V0-042). The bootstrap allowance is the decision-0055 shape the
// bind loop admitted: an unknown, no-evidence hunk adding a whole file absent
// at the change's base. An unknown hunk kept with a recorded reason
// (DCW-V0-019) is admitted beside it. The OCM that admitted it is not re-run. Only a
// cem/1.0 map is verified: the stable verifier derives the patch from the base
// and bind commits, and reads the producer map with the repository root as its
// artifact root (CEM-V1-148, CEM-V1-202). A retired cem/0.x seal that still
// equals its bind commit's sidecar fails constituent-retired-spec, which each
// caller admits without verification or coverage (CEM-V1-203).
func (f *flow) verifySealedCEM(verifier Runner, target, bind string) sealedVerification {
	path := ".corvint/changes/" + bind + ".cem.json"
	result := sealedVerification{detail: "bind=" + bind}
	if !f.gitSucceeds("merge-base", "--is-ancestor", bind, target) {
		result.failure = "constituent-not-merged"
		return result
	}
	sealedBlob, bindBlob := f.gitRevision(target+":"+path), f.gitRevision(bind+":.corvint/change.cem.json")
	if sealedBlob != "" && sealedBlob != bindBlob && f.migratedConstituent(target, path, bindBlob, sealedBlob) {
		result.failure = "constituent-migrated"
		return result
	}
	if sealedBlob == "" || sealedBlob != bindBlob {
		result.failure = "sealed-cem-drift"
		return result
	}
	data, status := f.git(true, "cat-file", "blob", sealedBlob)
	var cem sealedMap
	decoded := status == 0 && json.Unmarshal([]byte(data), &cem) == nil
	if decoded && wire.RetiredSpec(cem.Spec) {
		result.failure, result.detail = retiredSeal, result.detail+" spec="+cem.Spec
		return result
	}
	if !decoded || !wire.DeclaresStable([]byte(data)) || !fullRevision.MatchString(cem.BaseRevision) ||
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
	args := []string{"cem", "status", "--map", path, "--expected-base", cem.BaseRevision,
		"--target", bind, "--max-unknown", strconv.Itoa(len(result.bootstrap) + result.reasoned), "--max-mechanical", "0"}
	// A constituent that recorded deferrals is bounded by its own record,
	// as its check was (DCW-V0-371).
	if deferrals, _ := f.recordedDeferrals(cem.BaseRevision, bind); deferrals > 0 {
		args = append(args, "--max-deferred-intents", strconv.Itoa(deferrals))
	}
	args = append(args, "--artifacts", stableArtifactRoot(f.root))
	code := verifier.Run(f.ctx, f.root, args, &stdout, &stderr)
	f.checkpoint()
	if code != 0 && resourceLimited(stdout.Bytes()) {
		// A constituent over the protocol's 1,024 operations is rerun once
		// with the opt-in scaled bound, as its own check was (DCW-V0-373).
		stdout.Reset()
		stderr.Reset()
		code = verifier.Run(f.ctx, f.root, withScaledOperations(args), &stdout, &stderr)
		f.checkpoint()
	}
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
	if stable := result.observed.StableVerification; result.failure != "" && stable != nil && verifierCode.MatchString(stable.Code) {
		result.code, result.detail = stable.Code, result.detail+" code="+stable.Code
	}
	return result
}

// verifyConstituent re-verifies one sealed CEM of the batch and records the
// commits its own range covers.
func (b *batch) verifyConstituent(bind string) {
	result := b.verifySealedCEM(b.options.Verifier, b.target, bind)
	if result.failure == retiredSeal {
		// A retired seal proves nothing and covers nothing, not even its own
		// seal commit: every commit of its range must be in a verified cem/1.0
		// constituent's range or the batch fails unbound-commits (CEM-V1-203).
		delete(b.constituents, bind)
		b.print("%s: NOTE sealed-cem-retired %s NOT_REVERIFIED coverage=none\n", b.prefix, result.detail)
		b.print("  fix: if its commits are unbound, rebind the branch as one cem/1.0 change whose BASE precedes this bind and that removes this seal (merge origin/main, git rm the seal, then dogfood-change BASE=<the origin/main commit merged>)\n")
		return
	}
	if result.ran {
		observed, code := result.observed, ""
		if result.code != "" {
			code = " code=" + result.code
		}
		b.print("%s: CONSTITUENT bind=%s base=%s hunks=%s supported=%s unknown=%s bootstrapUnknown=%d reasonedUnknown=%d state=%s%s\n",
			b.prefix, bind, result.base, observedCount(observed.Counts.Total), observedCount(observed.Counts.Supported),
			observedCount(observed.Counts.Unknown), len(result.bootstrap), result.reasoned, observed.State, code)
		for _, added := range result.bootstrap {
			b.print("  bootstrap-unknown %s\n", added)
		}
		b.bootstrap += len(result.bootstrap)
	}
	if result.failure != "" {
		b.failure(result.failure, result.detail)
		// A verifier refusal in stderr alone leaves its code as the state.
		code := result.code
		if code == "" {
			code = result.observed.State
		}
		if batchCodeRecovery[code] != "" && !slices.Contains(b.codeFixes[result.failure], code) {
			b.codeFixes[result.failure] = append(b.codeFixes[result.failure], code)
		}
		return
	}
	b.bases[bind] = result.base
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
// own range to be that constituent's seal commit, the migration commit
// (DCW-V0-050) or a derivable merge.
func (b *batch) verifyCoverage() {
	listing := b.gitValue("rev-list", "--topo-order", "--reverse", "--parents", b.base+".."+b.target)
	var merges [][2]string
	for _, line := range nonEmpty(strings.Split(listing, "\n")) {
		fields := strings.Fields(line)
		commit, parents := fields[0], fields[1:]
		if len(parents) == 2 {
			merges = append(merges, [2]string{commit, parents[1]})
		}
		switch {
		case b.covered[commit]:
		case len(parents) == 0:
			b.failure("unbound-commits", commit)
		case len(parents) == 1:
			if (!b.constituents[parents[0]] || !b.isSealCommit(commit)) && !b.archivesInherited(parents[0], commit) && !b.isMigrationCommit(parents[0], commit) && !b.regeneratedCommit(parents[0], commit) {
				b.failure("unbound-commits", commit)
			}
		case len(parents) > 2:
			b.failure("octopus-merge-unbound", commit)
		default:
			b.deriveMerge(commit, parents[0], parents[1])
		}
	}
	b.mergedForward(merges)
}

// regeneratedCommit accepts a one-parent commit that is exactly the output of
// script/batch-regen.sh replayed on its parent (DCW-V0-401): the batch-regen
// commit and a lane's post-seal receipt or line-citation repin. It prints one
// NOTE either way, so an unbound commit names why its replay did not match.
func (b *batch) regeneratedCommit(parent, commit string) bool {
	result := b.regenerated(parent, commit)
	if result.ok {
		b.derived["regenerated"]++
		b.print("%s: NOTE COMMIT %s derivation=regenerated mode=%s (DCW-V0-401)\n", b.prefix, commit, result.mode)
		return true
	}
	if !result.silent {
		b.print("%s: NOTE COMMIT %s regeneration NOT_REPRODUCED %s (DCW-V0-401)\n", b.prefix, commit, result.reason)
	}
	return false
}

// mergedForward finds, for a range whose unbound commits a merge brought in
// from its second parent, the latest such merge (DCW-V0-110): a lane that
// merged the mainline forward and is checked against its original BASE. It
// changes no verdict; the commits stay unbound under this BASE, and finish
// names the merged parent as the BASE that leaves them to the mainline's own
// check while every commit of the branch is still checked.
func (b *batch) mergedForward(merges [][2]string) {
	unbound := b.failures["unbound-commits"]
	if len(unbound) == 0 {
		return
	}
	for index := len(merges) - 1; index >= 0; index-- {
		merged := merges[index][1]
		if b.gitValue(append(append([]string{"rev-list", "--count"}, unbound...), "^"+merged)...) == "0" {
			b.forward = merges[index]
			return
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
// alone; the manual part of any other merge is unbound content. A merge that
// keeps one side of a conflict, or its first parent's tree over a second side
// Git would merge, has each dropped hunk checked (DCW-V0-073).
func (b *batch) deriveMerge(merge, first, second string) {
	tree := b.gitValue("rev-parse", merge+"^{tree}")
	firstTree := tree == b.gitValue("rev-parse", first+"^{tree}")
	var attempted, conflicted string
	for _, strategy := range []struct{ name, option string }{{"ort", ""}, {"ort-ours", "-Xours"}, {"ort-theirs", "-Xtheirs"}} {
		// The merge conflict style is fixed so that the conflict markers
		// droppedSide parses do not depend on any configuration.
		args := []string{"-c", "merge.conflictStyle=merge", "merge-tree", "--write-tree", "--no-messages", "-z"}
		if strategy.option != "" {
			args = append(args, strategy.option)
		}
		result, status := b.scratchGit(append(args, first, second)...)
		written := strings.SplitN(result, "\x00", 2)[0]
		if strategy.name == "ort" && status == 1 {
			conflicted = result
		}
		if status == 0 && written == tree {
			derivation := strategy.name
			if firstTree && strategy.name != "ort-theirs" {
				derivation = "first-parent-tree"
			}
			b.derive(merge, derivation)
			if strategy.name != "ort" {
				b.droppedSide(merge, first, second, strategy.name == "ort-ours", conflicted)
			}
			return
		}
		if attempted == "" && fullRevision.MatchString(written) {
			attempted = written
		}
	}
	detail := ""
	if attempted != "" {
		differing, _ := b.scratchGit("diff-tree", "-r", "--name-only", attempted, tree)
		paths := nonEmpty(strings.Split(chomp(differing), "\n"))
		if len(paths) > batchPathsShown {
			paths = append(paths[:batchPathsShown], fmt.Sprintf("... %d more", len(paths)-batchPathsShown))
		}
		if len(paths) > 0 {
			detail = " paths=" + strings.Join(paths, ",")
		}
	}
	if firstTree {
		// The first parent's tree over changes Git would merge: the whole
		// second side is dropped, not only its conflicting hunks.
		b.derive(merge, "first-parent-tree")
		b.dropUnknown(merge, second, "", "second-side-discarded"+detail)
		return
	}
	b.failure("merge-resolution-unbound", merge+detail)
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
	output, status, bounded := b.readScratch(true, args...)
	if !bounded {
		b.refuse(gitOutputOverBound)
	}
	return output, status
}

// readScratch is scratchGit's read; a repository read reports its overflow
// to the caller instead of refusing (scratchGit must already have run once).
func (b *batch) readScratch(repository bool, args ...string) (string, int, bool) {
	if repository {
		args = append([]string{"--git-dir=repository.git", "-c", "core.hooksPath=" + os.DevNull,
			"-c", "core.attributesFile=" + os.DevNull}, args...)
	}
	b.checkpoint()
	return b.readGit(b.scratchCommand(args...))
}

func (b *batch) runScratch(args ...string) (string, int) {
	output, status, bounded := b.readScratch(false, args...)
	if !bounded {
		b.refuse(gitOutputOverBound)
	}
	return output, status
}

func (b *batch) scratchCommand(args ...string) *exec.Cmd {
	command := exec.CommandContext(b.ctx, "git", append([]string{"-c", "credential.helper="}, args...)...)
	command.Dir = b.scratch
	command.Env = append(gokernel.SanitizedGitEnvironment(), "GIT_ATTR_NOSYSTEM=1", "GIT_CEILING_DIRECTORIES="+filepath.Dir(b.scratch))
	return command
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
			for _, code := range b.codeFixes[reason] {
				b.say("  fix: %s: %s\n", code, batchCodeRecovery[code])
			}
			if reason == "unbound-commits" && b.forward[0] != "" {
				b.say("  fix: every listed commit came in through merge %s from its merged parent %s; if that parent is a mainline commit (git merge-base --is-ancestor %s origin/main), its commits are the mainline's to check: rerun batch-check with BASE=%s, which still checks every commit of this branch; a merged lane branch is not mainline, and its commits need the binding above\n",
					b.forward[0], b.forward[1], b.forward[1], b.forward[1])
			}
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
	if b.dropped.merges > 0 {
		// Every dropped hunk passed only because a later sealed change
		// restored it or the merge's message dispositions it (DCW-V0-073).
		b.print("%s: NOTE conflict-side-dropped merges=%d hunks=%d restored=%d dispositioned=%d\n",
			b.prefix, b.dropped.merges, b.dropped.hunks, b.dropped.restored, b.dropped.dispositioned)
	}
	b.print("%s: NOTE local-outcome NOT_PRODUCED batch-check\n", b.prefix)
	b.print("%s: NOTE verifier-independence NOT_OBSERVED single-verifier verifier=%s\n", b.prefix, verifier)
	b.print("%s: PASS constituents=%d commits=%d\n", b.prefix, constituents, commits)
	return 0
}
