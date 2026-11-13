package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/genesis"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

type guidanceInvocation struct {
	Root, Command, Base string
	MaxRefs             int
	removeScratch       func(string) error
}
type guidanceSkip struct {
	Ref    string `json:"ref"`
	Tip    string `json:"tip"`
	Reason string `json:"reason"`
}
type guidanceOverlap struct {
	Ref   string   `json:"ref"`
	Tip   string   `json:"tip"`
	Base  string   `json:"mergeBase"`
	State string   `json:"state"`
	Paths []string `json:"paths"`
}
type guidanceReview struct {
	CurrentRef   string                 `json:"currentRef"`
	Base         string                 `json:"base"`
	Affected     affectedCompactReceipt `json:"affected"`
	AffectedFull guidanceAffectedFull   `json:"affectedFull"`
	LocalTips    map[string]string      `json:"localTips"`
	Skipped      []guidanceSkip         `json:"skipped"`
	Overlaps     []guidanceOverlap      `json:"overlaps"`
}

// guidanceAffectedFull resolves the compact review.affected (RGV-V0-015):
// Digest is "affected-plan:sha256:" plus the SHA-256 of the canonical
// affected-plan/0 receipt review planned, which is the stdout of Argv without
// its trailing newline when run over the clean worktree at the revision.
type guidanceAffectedFull struct {
	Argv   []string `json:"argv"`
	Digest string   `json:"digest"`
}

const guidanceAffectedFullDigestPrefix = "affected-plan:sha256:"

// guidanceReceipt is the review document. The inferred features, overview fields and
// nextCalls are cut (decision 0597, RGV-V0-001).
type guidanceReceipt struct {
	Profile      string            `json:"profile"`
	Tool         string            `json:"tool"`
	Experimental bool              `json:"experimental"`
	Authority    string            `json:"authority"`
	Mutates      bool              `json:"mutates"`
	Revision     string            `json:"revision"`
	Tree         string            `json:"tree"`
	Index        map[string]string `json:"index"`
	Unknown      []string          `json:"unknown"`
	Omissions    map[string]int    `json:"omissions"`
	Review       *guidanceReview   `json:"review,omitempty"`
}

func parseGuidanceInvocation(args []string) (guidanceInvocation, bool, error) {
	out := guidanceInvocation{Root: ".", MaxRefs: 32}
	if _, help, _ := parseHelpInvocation(args); help {
		return out, false, nil
	}
	i := 0
	for i < len(args) && (args[i] == "--root" || strings.HasPrefix(args[i], "--root=")) {
		if args[i] == "--root" {
			if !rootPreambleValue(args, i+1) {
				return out, false, nil
			}
			out.Root = args[i+1]
			i += 2
		} else {
			out.Root = strings.TrimPrefix(args[i], "--root=")
			i++
		}
	}
	if i >= len(args) {
		return out, false, nil
	}
	out.Command = args[i]
	if out.Command != "review" {
		return out, false, nil
	}
	// review reads one repository: a workspace refuses it (WSD-V0-146).
	tail, file, named, disabled, err := stripWorkspaceFlags(args[i+1:])
	if err != nil {
		return out, true, err
	}
	args = tail
	seen := map[string]bool{}
	for i = 0; i < len(args); i++ {
		key, value, equals := strings.Cut(args[i], "=")
		if (key != "--base" && key != "--max-refs") || seen[key] {
			return out, true, argumentError("unrecognized guidance argument")
		}
		seen[key] = true
		if !equals {
			i++
			if i >= len(args) {
				return out, true, argumentError("guidance option requires a value")
			}
			value = args[i]
		}
		if key == "--base" {
			out.Base = value
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 32 {
			return out, true, argumentError("--max-refs must be 1..32")
		}
		out.MaxRefs = n
	}
	if !validGitObjectID(out.Base) {
		return out, true, argumentError("review --base requires a full commit id")
	}
	resolved, err := resolveExplicitRoot(out.Root)
	out.Root = resolved
	if err == nil {
		err = checkWorkspace("review", resolved, file, named, disabled, true)
	}
	return out, true, err
}

func guidanceError(err error) error {
	return &gokernel.Error{Code: "unsupported-repository-guidance", Message: err.Error()}
}

func runGuidance(ctx context.Context, in guidanceInvocation, stdout, stderr io.Writer) int {
	raw, err := compileGuidance(ctx, in, nil)
	if err != nil {
		var stale *gokernel.Error
		if errors.As(err, &stale) && stale.Code == selfBinaryStaleCode {
			err = stale // AFP-V0-9730: the refusal keeps its own code (V1-5012).
		} else if refusal := repositoryOwnershipRefusal(ctx, in.Root); refusal != nil {
			err = refusal
		} else {
			err = guidanceError(err)
		}
		emitError(stderr, err)
		return 2
	}
	if _, err = stdout.Write(append(raw, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write guidance receipt"})
		return 2
	}
	return 0
}

// beforeCheck is a test seam for deterministic source/ref drift, never a CLI option.
func compileGuidance(ctx context.Context, in guidanceInvocation, beforeCheck func()) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	snapshot, err := genesis.OpenGuidance(ctx, in.Root)
	if err != nil {
		return nil, err
	}
	receipt := discoverGuidance(snapshot)
	refs, err := snapshot.RefTips(ctx)
	if err != nil {
		return nil, err
	}
	receipt.Review, err = reviewGuidance(ctx, snapshot, in, refs, &receipt)
	if err != nil {
		return nil, err
	}
	raw, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, fmt.Errorf("guidance output exceeds 1 MiB")
	}
	if beforeCheck != nil {
		beforeCheck()
	}
	if err = snapshot.Check(ctx); err != nil {
		return nil, err
	}
	after, err := snapshot.RefTips(ctx)
	if err != nil {
		return nil, err
	}
	if after != refs {
		return nil, fmt.Errorf("local ref drift")
	}
	current, err := snapshot.CurrentRef(ctx)
	if err != nil {
		return nil, err
	}
	if current != receipt.Review.CurrentRef {
		return nil, fmt.Errorf("current ref drift")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

// discoverGuidance counts the inventory and unread sources review needs for the RGV-V0-013
// fallback; feature discovery is cut (decision 0597).
func discoverGuidance(s *genesis.GuidanceSnapshot) guidanceReceipt {
	out := guidanceReceipt{Profile: "repository-guidance/0", Tool: "review", Experimental: true, Authority: "inferred", Revision: s.Revision, Tree: s.Tree,
		Index:     map[string]string{"state": "UNKNOWN", "reason": "not-read; index-derived fields omitted"},
		Unknown:   []string{"advisory only; CEM, OCM, frontier and mandatory test obligations remain open"},
		Omissions: map[string]int{"inventory": s.Omitted}}
	for _, r := range s.Records {
		if r.Path == nil || r.Classification != "INCLUDED" {
			out.Omissions["unread-sources"]++
		}
	}
	if out.Omissions["inventory"] > 0 || out.Omissions["unread-sources"] > 0 {
		out.Unknown = append(out.Unknown, "inventory or source bytes omitted; absence is not negative evidence")
	}
	return out
}

func reviewGuidance(ctx context.Context, s *genesis.GuidanceSnapshot, in guidanceInvocation, refs string, out *guidanceReceipt) (*guidanceReview, error) {
	ancestor, err := s.MergeBase(ctx, in.Base, s.Revision)
	if err != nil {
		return nil, err
	}
	if ancestor != in.Base {
		return nil, fmt.Errorf("base must be an ancestor of target")
	}
	paths, err := s.Paths(ctx, in.Base, s.Revision)
	if err != nil {
		return nil, err
	}
	review := &guidanceReview{Base: in.Base, LocalTips: map[string]string{}, Skipped: []guidanceSkip{}, Overlaps: []guidanceOverlap{}}
	changed := map[string]bool{}
	for _, p := range paths {
		changed[p] = true
	}
	// RGV-V0-013, AFP-V0-9730 (V1-5012): review.affected is this binary's
	// plan, so a stale binary refuses before planning, as the standalone
	// command it must equal does.
	freshness := selfBinaryStale(ctx, in.Root)
	if freshness.Code == selfBinaryStaleCode {
		return nil, guidanceStaleRefusal(freshness)
	}
	var full affectedReceipt
	if out.Omissions["inventory"] > 0 || out.Omissions["unread-sources"] > 0 {
		// A capped inventory cannot support a selection or a gate-absence claim
		// (RGV-V0-013): plan through the standalone affected path instead.
		if full, err = compileAffected(ctx, affectedInvocation{Root: in.Root, Base: in.Base}); err != nil {
			return nil, err
		}
		if full.Revision != s.Revision {
			return nil, fmt.Errorf("HEAD drift")
		}
		out.Unknown = append(out.Unknown, guidanceAffectedFallback(in.Base))
	} else if full, err = snapshotReviewAffected(ctx, s, in, paths); err != nil {
		return nil, err
	}
	if err := review.setAffected(degradeGuidanceAffected(full, freshness, out)); err != nil {
		return nil, err
	}
	current, err := s.CurrentRef(ctx)
	if err != nil {
		return nil, err
	}
	review.CurrentRef = current
	names := []string{}
	for _, line := range strings.Split(strings.TrimSpace(refs), "\n") {
		if line == "" {
			continue
		}
		name, tip, ok := strings.Cut(line, " ")
		if !ok || !strings.HasPrefix(name, "refs/heads/") || !validGitObjectID(tip) {
			return nil, fmt.Errorf("malformed ref map")
		}
		review.LocalTips[name] = tip
		names = append(names, name)
	}
	admitted := []string{}
	processed := 0
	for _, name := range names {
		tip := review.LocalTips[name]
		if name == current {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "current-ref"})
			continue
		}
		if processed >= in.MaxRefs {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "ref-budget"})
			out.Omissions["refs"]++
			continue
		}
		processed++
		base, e := s.MergeBase(ctx, tip, s.Revision)
		if e != nil {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "UNKNOWN ancestry unavailable"})
			continue
		}
		reason := ""
		if base == tip {
			reason = "ancestor-of-target"
		}
		if base == s.Revision {
			reason = "descendant-of-target"
		}
		if reason != "" {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, reason})
			continue
		}
		admitted = append(admitted, name)
	}
	overlapBudget := 64
	for i, name := range admitted {
		tip := review.LocalTips[name]
		stacked := false
		for j, other := range admitted {
			if i == j {
				continue
			}
			otherTip := review.LocalTips[other]
			base, e := s.MergeBase(ctx, tip, otherTip)
			if e != nil {
				out.Omissions["ancestry-comparisons"]++
				continue
			}
			if base == tip && (tip != otherTip || i > j) {
				stacked = true
				break
			}
		}
		if stacked {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "ancestry-stacked"})
			continue
		}
		base, e := s.MergeBase(ctx, in.Base, tip)
		if e != nil {
			review.Overlaps = append(review.Overlaps, guidanceOverlap{Ref: name, Tip: tip, State: "UNKNOWN", Paths: []string{}})
			continue
		}
		branchPaths, e := s.Paths(ctx, base, tip)
		row := guidanceOverlap{Ref: name, Tip: tip, Base: base, State: "DISJOINT", Paths: []string{}}
		if e != nil || len(branchPaths) > 256 {
			row.State = "UNKNOWN"
			out.Omissions["branch-paths"]++
			review.Overlaps = append(review.Overlaps, row)
			continue
		}
		for _, p := range branchPaths {
			if changed[p] {
				row.Paths = append(row.Paths, p)
			}
		}
		if len(row.Paths) > 0 {
			row.State = "OVERLAP"
		}
		if len(row.Paths) > overlapBudget {
			out.Omissions["overlap-paths"] += len(row.Paths) - overlapBudget
			row.Paths = row.Paths[:overlapBudget]
			row.State = "OVERLAP_INCOMPLETE"
		}
		overlapBudget -= len(row.Paths)
		review.Overlaps = append(review.Overlaps, row)
	}
	return review, nil
}

// setAffected embeds the affected-plan/1 projection of the planned
// affected-plan/0 receipt and the digest and command that reproduce the full
// receipt (RGV-V0-015).
func (review *guidanceReview) setAffected(full affectedReceipt) error {
	compact, err := compactAffected(full)
	if err != nil {
		return err
	}
	encoded, err := gokernel.CanonicalJSON(full)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(encoded)
	review.Affected = compact
	review.AffectedFull = guidanceAffectedFull{Argv: []string{"corvint", "affected", "--base", full.Range.Base, "--full"}, Digest: guidanceAffectedFullDigestPrefix + hex.EncodeToString(sum[:])}
	return nil
}

// guidanceStaleRefusal is the AFP-V0-9730 refusal for `corvint review`
// (RGV-V0-013, V1-5012). Review takes no --accept-stale-binary: the
// commands its review.affected and affectedFull must equal refuse without it,
// so the fix: line names only the rebuild and the uninstalled build.
func guidanceStaleRefusal(freshness binaryFreshness) error {
	return &gokernel.Error{Code: selfBinaryStaleCode, Message: freshness.Detail +
		"; review.affected would be this binary's plan, which may be narrower than this checkout's own build gives, so the review is refused" +
		"; fix: rebuild the binary from this checkout (go install ./cmd/corvint) or run that build without installing it (go run ./cmd/corvint review --base FULL_SHA)"}
}

// degradeGuidanceAffected adds a self-binary-unverified verdict to the plan
// review embeds, the members `corvint affected --base` gives the same receipt
// (AFP-V0-846, ACX-V0-750), so review.affected and the affectedFull digest
// still equal that command's output; one receipt-level unknown keeps the
// notice visible in the review document (RGV-V0-013, V1-5012). A fresh
// binary, or one Corvint cannot judge, leaves the plan unchanged.
func degradeGuidanceAffected(full affectedReceipt, freshness binaryFreshness, out *guidanceReceipt) affectedReceipt {
	if freshness.Code == "" {
		return full
	}
	notice := selfBinaryNotice(freshness)
	full.Degradations = append(full.Degradations, freshness.Code)
	full.DegradationDetails = map[string]string{freshness.Code: notice}
	out.Unknown = append(out.Unknown, "review.affected was planned by a "+freshness.Code+" binary: "+notice)
	return full
}

// guidanceAffectedFallback names why review.affected was planned over the clean
// worktree rather than the immutable snapshot, and the equivalent command.
func guidanceAffectedFallback(base string) string {
	return "guidance inventory incomplete; review.affected is the `corvint affected --base " + base + "` plan over the clean worktree at the captured revision, not over immutable snapshot blobs"
}

// snapshotReviewAffected plans over the complete immutable snapshot materialized
// in private scratch; the scratch is removed before a successful return.
func snapshotReviewAffected(ctx context.Context, s *genesis.GuidanceSnapshot, in guidanceInvocation, paths []string) (receipt affectedReceipt, resultErr error) {
	scratch, err := s.Materialize()
	if err != nil {
		return affectedReceipt{}, err
	}
	remove := in.removeScratch
	if remove == nil {
		remove = os.RemoveAll
	}
	defer func() {
		resultErr = cleanupGuidanceScratch(scratch, resultErr, remove)
		if resultErr != nil {
			receipt = affectedReceipt{}
		}
	}()
	graph, err := affected.Build(ctx, scratch, affectedLanguages()...)
	if err != nil {
		return affectedReceipt{}, err
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return affectedReceipt{}, affectedGitExecutableRefusal()
	}
	// The snapshot is HEAD's tree, so the prior read scopes and the additive
	// records are measured from the base commit in the repository, through
	// the same planning tail as `corvint affected --base` (AFP-V0-930).
	graph, plan, provider, advice := planAffectedGraph(ctx, affectedPlanInputs{
		gitExecutable: gitExecutable, repository: in.Root, tree: scratch, base: in.Base, revision: s.Revision, additive: true,
	}, graph, affected.NormalizePaths(paths))
	receipt = affectedReceipt{Advice: advice, Mutates: false, OK: true, Plan: plan, Profile: affectedProfile, Provider: affectedProvider{Go: provider}, Range: affectedRange{Base: in.Base, Paths: paths}, Revision: s.Revision, Tool: "affected"}
	// SLF-V1-007: the verdict `corvint affected --base` decides for the plan,
	// over the repository: the Go selector compares against the base commit
	// there, which the scratch tree has no Git history for (RGV-V0-015).
	if err := affectedDecideVerdict(ctx, in.Root, &receipt, graph, nil); err != nil {
		return affectedReceipt{}, err
	}
	return receipt, nil
}

func cleanupGuidanceScratch(root string, original error, remove func(string) error) error {
	if err := remove(root); err != nil {
		return errors.Join(original, fmt.Errorf("scratch cleanup failed for %q: %w", root, err))
	}
	return original
}
