// Package workflow implements the CEM producer/verifier operations —
// begin, prepare, cite, mark, status, verify, and report — over the CEM
// seams, with the frozen CEM-CB envelope table, validation precedence, and
// resume rules.
package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Default artifact locations beneath the per-worktree Git directory.
const (
	defaultPatchRelative  = "corvint/change.patch"
	defaultReportRelative = "corvint/cem-review.md"
	maxReportBytes        = 8 << 20
)

// Session is one opened worktree root; the repository boundary is validated
// separately by openRepository at its frozen precedence position.
type Session struct {
	root       string
	workRoot   *publish.Root
	repository *gitauth.Repository
	gitRoot    *publish.Root
	commonDir  string // common Git directory, protected from external report output
	// stableRepository is the caller-named root a stable read admits; nil
	// means root (see SetStableRepository).
	stableRepository *string
	// stableHolders maps the mechanics view a mutator read from a stable
	// producer map to its stable document, which writeMap re-encodes.
	stableHolders map[*wire.Map]*wire.StableMap
	// stableOCM verifies an OCM a stable report supplies; nil leaves the
	// report's obligations NOT_PRODUCED (see SetStableOCMReader).
	stableOCM StableOCMReader
	// intentCounter counts an intent's obligations for a recorded deferral;
	// nil leaves each count NOT_CHECKED (see SetIntentObligationCounter).
	intentCounter IntentObligationCounter
	// dataReaders finds a declared data path's readers for the
	// cem-path-policy/1 profile; nil leaves each data row
	// data-reader-not-produced (see SetDataReaderFinder).
	dataReaders DataReaderFinder
}

// Open validates the worktree root as a publication root only. Repository
// validation (precedence stages 5–6) is deferred to openRepository so each
// command judges argument and map validation (stages 1–4) first.
func Open(root string) (*Session, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, cemcode.New(cemcode.RepositoryObjectUnavailable, "root path cannot be made absolute")
	}
	workRoot, err := publish.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	return &Session{root: absolute, workRoot: workRoot}, nil
}

// openRepository runs precedence stages 5–6: repository-root and reciprocal
// worktree validation, then alternate and attribute denial. Every command
// calls it before its first Git operation or publication.
func (s *Session) openRepository() error {
	if s.repository != nil {
		return nil
	}
	repository, err := gitauth.Open(s.root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	gitRoot, err := publish.OpenRoot(repository.GitDir)
	if err != nil {
		return err
	}
	s.repository, s.gitRoot, s.commonDir = repository, gitRoot, repository.CommonDir
	return nil
}

// openPublicationDirectories locates the worktree's Git directories for report
// publication only: it validates the primary/reciprocal worktree boundary but
// not the 0.x object admission (alternates, attributes), so a repository the
// stable verifier refused still has a checkable output path (CEM-V1-035). If
// the directories cannot be located, no output path can be checked and the
// caller must publish nothing.
func (s *Session) openPublicationDirectories() error {
	if s.gitRoot != nil {
		return nil
	}
	gitDir, commonDir, err := gitauth.WorktreeDirectories(s.root)
	if err != nil {
		return err
	}
	gitRoot, err := publish.OpenRoot(gitDir)
	if err != nil {
		return err
	}
	s.gitRoot, s.commonDir = gitRoot, commonDir
	return nil
}

func invalidArguments(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.InvalidArguments, format, args...)
}

// --- document model helpers ---

// buildCandidate constructs the mechanics of a fresh cem/1.0 producer map with
// every hunk explicitly unknown.
func buildCandidate(base string, patchBytes []byte, parsed *patch.Patch) *wire.Map {
	digest := sha256.Sum256(patchBytes)
	document := &wire.Map{Spec: wire.StableSpec, BaseRevision: base, PatchSha256: hex.EncodeToString(digest[:]), ExcludedPath: wire.ExcludedCEMPath}
	for _, hunk := range parsed.Hunks {
		document.Hunks = append(document.Hunks, wire.Hunk{
			ID: hunk.ID, Path: hunk.DisplayPath, OldRange: hunk.OldRange, NewRange: hunk.NewRange,
			Disposition: "unknown", Reason: "no-evidence",
		})
	}
	return document
}

// --- counts, worklist, policy ---

func countsAndWorklist(document *wire.Map) (map[string]any, []any) {
	counts := map[string]any{"total": 0, "supported": 0, "unknown": 0, "mechanical": 0}
	work := []any{}
	for index, hunk := range document.Hunks {
		counts["total"] = counts["total"].(int) + 1
		if current, ok := counts[hunk.Disposition].(int); ok {
			counts[hunk.Disposition] = current + 1
		}
		next := map[string]string{
			"supported": "done", "unknown": "cite-or-mark", "mechanical": "review-mechanical",
		}[hunk.Disposition]
		work = append(work, map[string]any{
			"selector": index + 1, "id": hunk.ID, "path": hunk.Path,
			"oldRange": rangeValue(hunk.OldRange), "newRange": rangeValue(hunk.NewRange),
			"disposition": hunk.Disposition, "reason": hunk.Reason, "next": next,
		})
	}
	return counts, work
}

// documentHunks reports hunks in the shape they are stored in the map. begin
// reports the document it just wrote, not the worklist: the worklist adds
// selector/next and drops basis, which belong to status alone.
func documentHunks(document *wire.Map) []any {
	hunks := make([]any, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		basis := make([]any, 0, len(hunk.Basis))
		for _, item := range hunk.Basis {
			basis = append(basis, map[string]any{"evidenceId": item.EvidenceID, "relation": item.Relation})
		}
		entry := map[string]any{
			"basis": basis, "disposition": hunk.Disposition, "id": hunk.ID,
			"newRange": rangeValue(hunk.NewRange), "oldRange": rangeValue(hunk.OldRange),
			"path": hunk.Path, "reason": hunk.Reason,
		}
		if hunk.Coverage != nil {
			entry["coverage"] = coverageValue(hunk.Coverage)
		}
		if hunk.Discriminates != nil {
			entry["discriminates"] = discriminationValue(hunk.Discriminates)
		}
		hunks = append(hunks, entry)
	}
	return hunks
}

func coverageValue(witness *wire.CoverageWitness) map[string]any {
	covered := make([]any, 0, len(witness.Covered))
	for _, item := range witness.Covered {
		covered = append(covered, rangeValue(item))
	}
	return map[string]any{
		"covered": covered, "mode": witness.Mode, "profileSha256": witness.ProfileSha256,
		"state": witness.State, "testRun": witness.TestRun,
	}
}

func discriminationValue(witness *wire.DiscriminationWitness) map[string]any {
	survivors := make([]any, 0, len(witness.Survivors))
	for _, item := range witness.Survivors {
		survivors = append(survivors, map[string]any{
			"description": item.Description, "line": item.Line, "operator": item.Operator,
		})
	}
	return map[string]any{
		"bounds": map[string]any{
			"maxHunks": witness.Bounds.MaxHunks, "maxMutants": witness.Bounds.MaxMutants,
			"wallTimeSeconds": witness.Bounds.WallTimeSeconds,
		},
		"detail": witness.Detail, "killed": witness.Killed, "mutants": witness.Mutants,
		"selectionSha256": witness.SelectionSha256, "state": witness.State,
		"survived": witness.Survived, "survivors": survivors, "treeRevision": witness.TreeRevision,
	}
}

func rangeValue(value wire.Range) map[string]any {
	return map[string]any{"start": value.Start, "count": value.Count}
}

func firstOpen(work []any) any {
	for _, entry := range work {
		if entry.(map[string]any)["next"] != "done" {
			return entry
		}
	}
	return nil
}

// PolicyLimits are optional disposition-count ceilings.
type PolicyLimits struct {
	MaxUnknown    *int
	MaxMechanical *int
}

func policyIssues(limits PolicyLimits, counts map[string]any) []any {
	issues := []any{}
	for _, policy := range []struct {
		limit *int
		field string
		code  string
	}{{limits.MaxUnknown, "unknown", "max-unknown-exceeded"}, {limits.MaxMechanical, "mechanical", "max-mechanical-exceeded"}} {
		if policy.limit == nil {
			continue
		}
		actual := counts[policy.field].(int)
		if actual > *policy.limit {
			issues = append(issues, map[string]any{
				"code": policy.code, "disposition": policy.field, "actual": actual,
				"maximum": *policy.limit, "message": policy.field + " hunk count exceeds policy maximum",
			})
		}
	}
	return issues
}

// --- shared input helpers ---

// readMapBytes reads one bounded repository-relative map without parsing it.
func (s *Session) readMapBytes(relative string) ([]byte, error) {
	if relative == "" {
		return nil, invalidArguments("--map is required")
	}
	if filepath.IsAbs(relative) {
		return nil, invalidArguments("map path must be repository-relative")
	}
	return s.workRoot.ReadBounded(relative, wire.MaxMapBytes, cemcode.MapUnavailable)
}

// resolveHunk resolves a one-based worklist ordinal or a full derived hunk ID.
func resolveHunk(document *wire.Map, selector string) (int, error) {
	if selector == "" {
		return 0, invalidArguments("--hunk is required")
	}
	// CEM-PILOT-003: only a canonical one-based decimal is an ordinal; "01"
	// and "+1" are not, and resolve to no hunk.
	if strings.Trim(selector, "0123456789") == "" {
		if selector[0] == '0' {
			return 0, cemcode.New(cemcode.InvalidArguments, "decimal hunk selectors must be canonical and one-based")
		}
		ordinal, err := strconv.Atoi(selector)
		if err != nil || ordinal > len(document.Hunks) {
			return 0, cemcode.New(cemcode.InvalidArguments, "hunk selector is outside the worklist")
		}
		return ordinal - 1, nil
	}
	for index, hunk := range document.Hunks {
		if hunk.ID == selector {
			return index, nil
		}
	}
	return 0, cemcode.New(cemcode.UnknownHunkID, "hunk id must identify exactly one mapped hunk")
}
