// SPDX-License-Identifier: Apache-2.0

package docmaintain

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Block states of a check (DMF-V1-003). Every state but BlockFresh is stale: a block that cannot be
// compared is never reported fresh.
const (
	BlockFresh     = "fresh"
	BlockStale     = "stale"
	BlockMissing   = "missing"
	BlockUnchecked = "unchecked"
)

const maxCheckPageBytes = 16 << 20

// CheckPage names one page and the selectors to check in it; no selectors checks every generated
// block the committed page carries.
type CheckPage struct {
	Page      string
	Selectors []Selector
}

// CheckBlock is one generated block's freshness at HEAD. Line is the 1-based line of its begin
// marker in the committed page, 0 when the block is missing or the page cannot be scanned.
type CheckBlock struct {
	Page            string `json:"page"`
	Line            int    `json:"line"`
	Source          string `json:"source"`
	Package         string `json:"package"`
	State           string `json:"state"`
	Reason          string `json:"reason,omitempty"`
	OldSourceDigest string `json:"old_source_digest,omitempty"`
	NewSourceDigest string `json:"new_source_digest,omitempty"`
}

// CheckReport is the read-only `docs maintain --check` answer about committed content (DMF-V1-003).
type CheckReport struct {
	Commit string       `json:"commit"`
	Blocks []CheckBlock `json:"blocks"`
	Stale  int          `json:"stale"`
}

// Check compares every selected generated block of each page, as committed at HEAD, with a fresh
// draft of its cited sources at HEAD. It writes nothing and never reads worktree bytes: the page is
// the committed blob and the draft reads the context index's pinned tree. A missing block, a page
// absent at HEAD, an unscannable page or a refused draft is stale with a reason.
func Check(ctx context.Context, root string, pages []CheckPage) (*CheckReport, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, refuse("invalid-root")
	}
	index, err := contextindex.BuildContext(ctx, absRoot, "")
	if err != nil {
		return nil, refuse("repository-unavailable")
	}
	if index.CommitRevision == "" {
		return nil, refuse("repository-unavailable")
	}
	report := &CheckReport{Commit: index.CommitRevision, Blocks: []CheckBlock{}}
	budget := gitrun.NewBudget(2*len(pages)+1, 2*time.Minute)
	for _, p := range pages {
		if !contained(absRoot, filepath.Join(absRoot, filepath.FromSlash(p.Page))) || p.Page == "" {
			return nil, refuse("invalid-page-path")
		}
		page, present, err := committedPage(ctx, budget, absRoot, report.Commit, p.Page)
		if err != nil {
			return nil, err
		}
		report.Blocks = append(report.Blocks, checkPage(index, p, page, present)...)
	}
	for _, b := range report.Blocks {
		if b.State != BlockFresh {
			report.Stale++
		}
	}
	return report, nil
}

func checkPage(index *contextindex.Index, p CheckPage, page []byte, present bool) []CheckBlock {
	unchecked := func(s Selector, reason string) CheckBlock {
		return CheckBlock{Page: p.Page, Source: s.Source, Package: s.Package, State: BlockUnchecked, Reason: reason}
	}
	layout, code := scanPage(page)
	selectors := p.Selectors
	if len(selectors) == 0 {
		if !present {
			return []CheckBlock{unchecked(Selector{}, "page-absent-at-head")}
		}
		if code != "" {
			return []CheckBlock{unchecked(Selector{}, code)}
		}
		for _, b := range layout.blocks {
			selectors = append(selectors, Selector{Source: b.source, Package: b.pkg})
		}
	}
	blocks := []CheckBlock{}
	for _, s := range selectors {
		switch {
		case !present:
			blocks = append(blocks, CheckBlock{Page: p.Page, Source: s.Source, Package: s.Package, State: BlockMissing, Reason: "page-absent-at-head"})
			continue
		case code != "":
			blocks = append(blocks, unchecked(s, code))
			continue
		case !validSelector(s):
			blocks = append(blocks, unchecked(s, "invalid-selector"))
			continue
		}
		block := CheckBlock{Page: p.Page, Source: s.Source, Package: s.Package}
		draft, err := doccompiler.DraftSources(index, s.Source, s.Package)
		if err != nil {
			blocks = append(blocks, unchecked(s, "draft-refused: "+doccompilerRefusal(err).Code))
			continue
		}
		block.NewSourceDigest = sourceDigest(draft)
		_, oldDigest, start, _, found := findBlock(page, s)
		switch {
		case !found:
			block.State, block.Reason = BlockMissing, "no generated block for this selector"
		case oldDigest != block.NewSourceDigest:
			block.State, block.Reason = BlockStale, "cited source digest changed"
			block.OldSourceDigest, block.Line = oldDigest, bytes.Count(page[:start], []byte("\n"))+1
		default:
			block.State, block.OldSourceDigest, block.Line = BlockFresh, oldDigest, bytes.Count(page[:start], []byte("\n"))+1
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// committedPage reads page as committed at commit; present is false when the commit has no regular
// file there.
func committedPage(ctx context.Context, budget *gitrun.Budget, root, commit, page string) ([]byte, bool, error) {
	run := func(limit int, args ...string) ([]byte, error) {
		return gitrun.Run(ctx, budget, gitrun.Options{Dir: root, Env: gokernel.SanitizedGitEnvironment(), StdoutLimit: limit},
			append([]string{"--no-replace-objects", "--literal-pathspecs", "-c", "core.quotePath=false"}, args...)...)
	}
	listing, err := run(1<<16, "ls-tree", "-z", "--full-tree", commit, "--", page)
	if err != nil {
		return nil, false, refuse("repository-unavailable")
	}
	meta, name, ok := strings.Cut(string(bytes.TrimSuffix(listing, []byte{0})), "\t")
	fields := strings.Fields(meta)
	if !ok || name != page || len(fields) != 3 || fields[1] != "blob" || !strings.HasPrefix(fields[0], "100") {
		return nil, false, nil
	}
	raw, err := run(maxCheckPageBytes, "cat-file", "blob", fields[2])
	if err != nil {
		return nil, false, &Refusal{Code: "page-unreadable", Message: fmt.Sprintf("cannot read %s at %s", page, commit)}
	}
	return raw, true, nil
}
