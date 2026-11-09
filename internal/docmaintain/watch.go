package docmaintain

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

const watchPageLimit = 1 << 20
const watchSummaryLimit = 32
const watchInterval = time.Second

// WatchSummary retains source identity, never a tick's page or diff.
type WatchSummary struct {
	Commit       string `json:"commit"`
	Tree         string `json:"tree"`
	SourceDigest string `json:"source_digest"`
	PageDigest   string `json:"page_digest"`
	Status       string `json:"status"`
}

// WatchReceipt is a bounded aggregate for the explicit foreground profile.
type WatchReceipt struct {
	Profile          string         `json:"profile"`
	Cycles           int            `json:"cycles"`
	Writes           int            `json:"writes"`
	StoppedReason    string         `json:"stopped_reason"`
	Complete         bool           `json:"complete"`
	Summaries        []WatchSummary `json:"summaries"`
	OmittedSummaries int            `json:"omitted_summaries"`
}

type sourceIdentity struct{ commit, tree string }
type pageIdentity struct {
	digest  string
	existed bool
}
type watchOperations struct {
	identity func(context.Context, string) (sourceIdentity, error)
	preview  func(context.Context, string, string, []Selector, Policy) (*Result, error)
	apply    func(string, *Result, Policy) (*Result, error)
}

// Watch refreshes one selector until its explicit bounds or a conflict stop it.
// It never adopts externally changed page bytes as the next cycle's baseline.
func Watch(ctx context.Context, root, page string, selector Selector, policy Policy) (*WatchReceipt, error) {
	receipt, err := watch(ctx, root, page, selector, policy, watchInterval, watchOperations{readSourceIdentity, Preview, Apply})
	if err != nil && receipt.StoppedReason == "" {
		receipt.StoppedReason = err.Error()
	}
	return receipt, err
}

func watch(ctx context.Context, root, page string, selector Selector, policy Policy, interval time.Duration, ops watchOperations) (*WatchReceipt, error) {
	receipt := &WatchReceipt{Profile: "corvint-docmaintain-watch/0", Summaries: []WatchSummary{}}
	if !watchPlatformSupported {
		return receipt, refuse("unsupported-watch-platform")
	}
	if !policy.Enabled {
		return receipt, refuse("maintenance-session-disabled")
	}
	if !policy.Apply {
		return receipt, refuse("apply-not-authorized")
	}
	if policy.MaxWrites <= 0 || policy.MaxWrites > 1024 {
		return receipt, refuse("invalid-watch-bounds")
	}
	if policy.MaxWallClock <= 0 || policy.MaxWallClock > 24*time.Hour {
		return receipt, refuse("invalid-watch-bounds")
	}
	ctx, cancel := context.WithTimeout(ctx, policy.MaxWallClock)
	defer cancel()
	root, err := filepath.Abs(root)
	if err != nil {
		return receipt, refuse("invalid-root")
	}
	path := filepath.Join(root, filepath.FromSlash(page))
	if !contained(root, path) {
		return receipt, refuse("invalid-page-path")
	}
	policy.pageLimit = watchPageLimit
	expected, err := observePage(root, path)
	if err != nil {
		return receipt, err
	}
	maxCycles := min(int((policy.MaxWallClock+interval-1)/interval), 86400)
	previous := sourceIdentity{}
	for receipt.Cycles < maxCycles {
		if ctx.Err() != nil {
			return stopContext(receipt, ctx)
		}
		receipt.Cycles++
		current, cycleErr := watchCycle(ctx, root, page, path, selector, policy, expected, previous, ops, receipt)
		if cycleErr != nil {
			return stopWatchError(receipt, ctx, cycleErr)
		}
		expected, previous = current.page, current.source
		if receipt.Writes >= policy.MaxWrites {
			receipt.StoppedReason = "max-writes"
			receipt.Complete = true
			return receipt, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return stopContext(receipt, ctx)
		case <-timer.C:
		}
	}
	receipt.StoppedReason, receipt.Complete = "max-cycles", true
	return receipt, nil
}

type cycleState struct {
	page   pageIdentity
	source sourceIdentity
}

func watchCycle(ctx context.Context, root, page, path string, selector Selector, policy Policy, expected pageIdentity, previous sourceIdentity, ops watchOperations, receipt *WatchReceipt) (cycleState, error) {
	state := cycleState{expected, previous}
	if err := requirePage(root, path, expected); err != nil {
		return state, err
	}
	identity, err := ops.identity(ctx, root)
	if err != nil {
		return state, err
	}
	if identity == previous {
		return state, requirePage(root, path, expected)
	}
	preview, err := ops.preview(ctx, root, page, []Selector{selector}, policy)
	if err != nil {
		return state, err
	}
	if len(preview.Receipt.Blocks) != 1 {
		return state, refuse("maintenance-incomplete")
	}
	block := preview.Receipt.Blocks[0]
	if (sourceIdentity{block.Commit, block.Tree}) != identity {
		return state, refuse("source-drift")
	}
	if (pageIdentity{preview.Receipt.PageDigestBefore, preview.Receipt.PageExistedAtStart}) != expected {
		return state, refuse("maintenance-conflict")
	}
	if err := requirePage(root, path, expected); err != nil {
		return state, err
	}
	before, err := ops.identity(ctx, root)
	if err != nil {
		return state, err
	}
	if before != identity {
		return state, refuse("source-drift")
	}
	if ctx.Err() != nil {
		return state, ctx.Err()
	}
	applied, err := ops.apply(root, preview, policy)
	if err != nil {
		return state, err
	}
	state.source = identity
	status := "unchanged"
	if applied.Receipt.Applied {
		receipt.Writes++
		state.page = pageIdentity{digestHex(applied.ProposedPage), true}
		status = "applied"
	}
	after, err := ops.identity(ctx, root)
	if err != nil {
		receipt.add(identity, block.NewSHA256, state.page.digest, "incomplete")
		return state, refuse("source-incomplete")
	}
	if after != identity {
		receipt.add(identity, block.NewSHA256, state.page.digest, "superseded")
		return state, refuse("source-superseded")
	}
	receipt.add(identity, block.NewSHA256, state.page.digest, status)
	if err := requirePage(root, path, state.page); err != nil {
		return state, err
	}
	return state, nil
}

func (r *WatchReceipt) add(identity sourceIdentity, source, page, status string) {
	if len(r.Summaries) >= watchSummaryLimit {
		r.OmittedSummaries++
		return
	}
	r.Summaries = append(r.Summaries, WatchSummary{identity.commit, identity.tree, source, page, status})
}

func stopContext(receipt *WatchReceipt, ctx context.Context) (*WatchReceipt, error) {
	receipt.StoppedReason = "interrupted"
	if ctx.Err() == context.DeadlineExceeded {
		receipt.StoppedReason = "max-wall-clock"
		receipt.Complete = true
	}
	return receipt, nil
}

func stopWatchError(receipt *WatchReceipt, ctx context.Context, err error) (*WatchReceipt, error) {
	if ctx.Err() != nil && (err.Error() == "repository-unavailable" || err == context.Canceled || err == context.DeadlineExceeded) {
		return stopContext(receipt, ctx)
	}
	receipt.StoppedReason = err.Error()
	return receipt, err
}

func readSourceIdentity(ctx context.Context, root string) (sourceIdentity, error) {
	// Fresh admission and no object session/memo: mutable HEAD must never be cached.
	budget := gitrun.NewBudget(4, 10*time.Second)
	repository, err := gitauth.Open(root, budget)
	if err != nil {
		return sourceIdentity{}, refuse("repository-unavailable")
	}
	commit, err := repository.Resolve(ctx, "HEAD")
	if err != nil {
		return sourceIdentity{}, refuse("repository-unavailable")
	}
	tree, err := repository.CommitTree(ctx, commit)
	if err != nil {
		return sourceIdentity{}, refuse("repository-unavailable")
	}
	return sourceIdentity{commit, tree}, nil
}

func observePage(root, path string) (pageIdentity, error) {
	if !contained(root, path) {
		return pageIdentity{}, refuse("invalid-page-path")
	}
	data, existed, err := readSessionPage(path, watchPageLimit)
	return pageIdentity{digestHex(data), existed}, err
}

func requirePage(root, path string, expected pageIdentity) error {
	actual, err := observePage(root, path)
	if err != nil {
		return err
	}
	if actual != expected {
		return refuse("maintenance-conflict")
	}
	return nil
}

func readSessionPage(path string, limit int) ([]byte, bool, error) {
	if limit == 0 {
		return readIfExists(path)
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, refuse("page-unreadable")
	}
	if !info.Mode().IsRegular() {
		return nil, false, refuse("page-unreadable")
	}
	if info.Size() > int64(limit) {
		return nil, false, refuse("page-too-large")
	}
	file, err := openWatchPage(path)
	if err != nil {
		return nil, false, refuse("page-unreadable")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return nil, false, refuse("page-unreadable")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, false, refuse("page-unreadable")
	}
	if len(data) > limit {
		return nil, false, refuse("page-too-large")
	}
	return data, true, nil
}
