// Package docmaintain implements an explicitly enabled, bounded local session
// that keeps one human documentation page's generated blocks in sync with
// immutable Git source, per the IPR-05 slice of
// docs/plans/integrated-product-roadmap-2026-09-12.md and the maintenance
// requirements in docs/specs/source-documentation-draft-v0.md (SDD-V0-007+).
//
// A session never renders a document (docs/specs/human-documentation-compiler-v0.md
// remains not-started) and never promotes generated prose to accepted intent
// (AGENTS.md invariant 8); it only refreshes source-pinned Markdown blocks that
// the same native corvint.docs_draft compiler
// (internal/doccompiler.DraftSources over internal/contextindex.BuildContext)
// already produces, inside explicit begin/end markers, leaving every other
// byte of the page untouched.
package docmaintain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
)

// Selector names one (owner Markdown source, Go package directory) pair, the
// same shape corvint.docs_draft accepts (docsbridge.draftInput).
type Selector struct {
	Source  string
	Package string
}

// Policy bounds one session. Enabled must be true or Run refuses outright.
// Apply authorizes writing; false always previews only, regardless of what
// eligible changes are found. MaxWrites and MaxWallClock bound how much of a
// multi-selector session executes before it stops cleanly.
type Policy struct {
	Enabled      bool
	Apply        bool
	MaxWrites    int
	MaxWallClock time.Duration
	pageLimit    int
}

// BlockOutcome reports what happened to one selector's generated block.
type BlockOutcome struct {
	Selector  Selector
	Existed   bool // a generated block for this selector was already in the page
	Eligible  bool // its content digest differs from the page's recorded digest (or it did not exist)
	Applied   bool // this session actually rewrote it
	Skipped   string
	OldSHA256 string
	NewSHA256 string
	Commit    string
	Tree      string
}

// Receipt is the durable session record: source identities, page digest
// before/after, and whether the session previewed or applied.
type Receipt struct {
	Page               string
	Selectors          []Selector
	StartedAt          time.Time
	FinishedAt         time.Time
	PageExistedAtStart bool
	PageDigestBefore   string
	PageDigestAfter    string
	Applied            bool
	PreviewOnly        bool
	Conflict           bool
	ConflictDetail     string
	Blocks             []BlockOutcome
	StoppedReason      string // "", "max-writes", "max-wall-clock"
}

// Result is what Run returns: the proposed page content, a unified diff
// against the bytes read at session start, and the session Receipt.
type Result struct {
	ProposedPage []byte
	Diff         string
	Receipt      Receipt
}

// Refusal is a closed, sanitized session refusal (disabled policy, invalid
// selector, unauthorized apply, or a write-time conflict). It never carries
// underlying process output.
type Refusal struct{ Code string }

func (r *Refusal) Error() string { return r.Code }

func refuse(code string) *Refusal { return &Refusal{Code: code} }

// Preview computes, but never writes, the proposed page content for page
// (repository-relative to root) over selectors, in order: it detects which
// generated blocks are eligible for refresh (missing, or whose recorded
// markdown_sha256 differs from a fresh redraft), skips ineligible/unchanged
// blocks untouched, and stops accepting further writes once policy.MaxWrites
// or policy.MaxWallClock is reached — remaining selectors are recorded as
// skipped, not silently dropped. It requires policy.Enabled but never checks
// or requires policy.Apply.
func Preview(ctx context.Context, root, page string, selectors []Selector, policy Policy) (*Result, error) {
	if !policy.Enabled {
		return nil, refuse("maintenance-session-disabled")
	}
	if len(selectors) == 0 {
		return nil, refuse("no-selectors")
	}
	if policy.MaxWrites <= 0 {
		policy.MaxWrites = len(selectors)
	}
	if policy.MaxWallClock <= 0 {
		policy.MaxWallClock = time.Minute
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, refuse("invalid-root")
	}
	absPage := filepath.Join(absRoot, filepath.FromSlash(page))
	if !contained(absRoot, absPage) {
		return nil, refuse("invalid-page-path")
	}

	started := time.Now()
	startBytes, existed, err := readSessionPage(absPage, policy.pageLimit)
	if err != nil {
		return nil, refuse("page-unreadable")
	}
	startDigest := digestHex(startBytes)

	index, err := contextindex.BuildContext(ctx, absRoot, "")
	if err != nil {
		return nil, refuse("repository-unavailable")
	}

	current := startBytes
	receipt := Receipt{
		Page: page, Selectors: selectors, StartedAt: started,
		PageExistedAtStart: existed, PageDigestBefore: startDigest,
		PreviewOnly: true, PageDigestAfter: startDigest,
	}
	writes := 0
	for _, selector := range selectors {
		if time.Since(started) > policy.MaxWallClock {
			receipt.StoppedReason = "max-wall-clock"
			receipt.Blocks = append(receipt.Blocks, BlockOutcome{Selector: selector, Skipped: "session-bounds"})
			continue
		}
		if !validSelector(selector) {
			return nil, refuse("invalid-selector")
		}
		draft, draftErr := doccompiler.DraftSources(index, selector.Source, selector.Package)
		if draftErr != nil {
			return nil, refuse(doccompilerCode(draftErr))
		}
		newDigest := sourceDigest(draft.Entries)
		if containsMarker(draft.Markdown) {
			return nil, refuse("marker-in-content")
		}
		block := renderBlock(selector, draft.Commit, draft.Tree, newDigest, draft.Markdown)

		existingBody, oldDigest, foundStart, foundEnd, hasBlock := findBlock(current, selector)
		if hasBlock && containsMarker(existingBody) {
			return nil, refuse("marker-in-content")
		}
		outcome := BlockOutcome{
			Selector: selector, Existed: hasBlock, OldSHA256: oldDigest,
			NewSHA256: newDigest, Commit: draft.Commit, Tree: draft.Tree,
		}
		outcome.Eligible = !hasBlock || oldDigest != newDigest
		if !outcome.Eligible {
			outcome.Skipped = "unchanged"
			receipt.Blocks = append(receipt.Blocks, outcome)
			continue
		}
		if writes >= policy.MaxWrites {
			receipt.StoppedReason = "max-writes"
			outcome.Skipped = "session-bounds"
			receipt.Blocks = append(receipt.Blocks, outcome)
			continue
		}
		if hasBlock {
			current = append(append(append([]byte{}, current[:foundStart]...), block...), current[foundEnd:]...)
		} else {
			current = insertBlock(current, block)
		}
		outcome.Applied = true
		writes++
		receipt.Blocks = append(receipt.Blocks, outcome)
	}

	if policy.pageLimit > 0 && len(current) > policy.pageLimit {
		return nil, refuse("page-too-large")
	}
	receipt.FinishedAt = time.Now()
	// Watch emits only bounded digest summaries, avoiding the preview diff
	// algorithm's quadratic line matrix for large human pages.
	diff := ""
	if policy.pageLimit == 0 {
		diff = unifiedDiff(page, startBytes, current)
	}
	return &Result{ProposedPage: current, Diff: diff, Receipt: receipt}, nil
}

// Apply authorizes and performs the write a prior Preview proposed. It
// refuses unless policy.Apply is true, and it refuses — writing nothing — if
// the page's bytes on disk no longer equal exactly what Preview started
// from: that is the session's whole conflict boundary, and it never applies
// part of a proposal. Applying a Preview that found nothing eligible is a
// harmless no-op (Applied stays false).
func Apply(root string, preview *Result, policy Policy) (*Result, error) {
	if preview == nil {
		return nil, refuse("no-preview")
	}
	if !policy.Apply {
		return nil, refuse("apply-not-authorized")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, refuse("invalid-root")
	}
	absPage := filepath.Join(absRoot, filepath.FromSlash(preview.Receipt.Page))
	if !contained(absRoot, absPage) {
		return nil, refuse("invalid-page-path")
	}

	receipt := preview.Receipt
	receipt.PreviewOnly = false
	writes := 0
	for _, block := range receipt.Blocks {
		if block.Applied {
			writes++
		}
	}
	if writes == 0 {
		receipt.PageDigestAfter = receipt.PageDigestBefore
		return &Result{ProposedPage: preview.ProposedPage, Diff: preview.Diff, Receipt: receipt}, nil
	}

	// Conflict check: the page on disk must still equal exactly what Preview
	// started from. No partial write happens on mismatch.
	nowBytes, nowExisted, err := readSessionPage(absPage, policy.pageLimit)
	if err != nil {
		return nil, refuse("page-unreadable")
	}
	if nowExisted != receipt.PageExistedAtStart || digestHex(nowBytes) != receipt.PageDigestBefore {
		receipt.Conflict = true
		receipt.ConflictDetail = "page changed on disk since the preview was computed"
		receipt.PageDigestAfter = digestHex(nowBytes)
		return &Result{ProposedPage: preview.ProposedPage, Diff: preview.Diff, Receipt: receipt}, refuse("maintenance-conflict")
	}
	if err := atomicWrite(absPage, preview.ProposedPage); err != nil {
		return nil, refuse("write-failed")
	}
	receipt.Applied = true
	receipt.PageDigestAfter = digestHex(preview.ProposedPage)
	return &Result{ProposedPage: preview.ProposedPage, Diff: preview.Diff, Receipt: receipt}, nil
}

// Run is the convenience path: Preview, then Apply when policy.Apply is
// true. Callers that need to inspect or gate the preview before authorizing
// the write (e.g. an operator confirmation step) should call Preview and
// Apply directly instead.
func Run(ctx context.Context, root, page string, selectors []Selector, policy Policy) (*Result, error) {
	preview, err := Preview(ctx, root, page, selectors, policy)
	if err != nil || !policy.Apply {
		return preview, err
	}
	return Apply(root, preview, policy)
}

func doccompilerCode(err error) string {
	var problem *doccompiler.Error
	if e, ok := err.(*doccompiler.Error); ok {
		problem = e
	}
	if problem != nil {
		return problem.Code
	}
	return "internal-error"
}

func readIfExists(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}

// contained reports whether path is safely inside root: string-contained,
// and free of any symlink component (the page itself, or any directory in
// its path down from root) that could read or write outside the repository
// through a link the string check cannot see. No component may name Git
// metadata in any letter case, which a case-insensitive volume would honor.
func contained(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if relative == ".." || relative == "." || hasDotDotPrefix(relative) {
		return false
	}
	if namesGitMetadata(relative) {
		return false
	}
	if filepath.Ext(relative) != ".md" {
		return false
	}
	return !crossesSymlink(root, relative)
}

func namesGitMetadata(relative string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(relative), "/") {
		if strings.EqualFold(segment, ".git") {
			return true
		}
	}
	return false
}

// crossesSymlink Lstats every path component from root down to the full
// relative path, refusing (returning true) as soon as any component is a
// symlink. A component that does not exist yet is not a symlink and stops
// the walk without refusing: a not-yet-created page is normal.
func crossesSymlink(root, relative string) bool {
	current := root
	for _, segment := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func hasDotDotPrefix(relative string) bool {
	return len(relative) >= 2 && relative[0] == '.' && relative[1] == '.' &&
		(len(relative) == 2 || relative[2] == filepath.Separator)
}

func atomicWrite(path string, data []byte) error {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".docmaintain-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if existing, statErr := os.Lstat(path); statErr == nil {
		if err := temp.Chmod(existing.Mode().Perm()); err != nil {
			temp.Close()
			os.Remove(tempName)
			return err
		}
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		os.Remove(tempName)
		return err
	}
	return nil
}

func digestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Summary is a small human-readable render used by callers (CLI, evidence)
// that want a one-line-per-block summary without re-deriving it.
func Summary(r Receipt) string {
	summary := fmt.Sprintf("page=%s applied=%v preview=%v conflict=%v stopped=%q\n", r.Page, r.Applied, r.PreviewOnly, r.Conflict, r.StoppedReason)
	for _, block := range r.Blocks {
		summary += fmt.Sprintf("  %s/%s eligible=%v applied=%v skipped=%q old=%s new=%s\n",
			block.Selector.Source, block.Selector.Package, block.Eligible, block.Applied, block.Skipped, short(block.OldSHA256), short(block.NewSHA256))
	}
	return summary
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
