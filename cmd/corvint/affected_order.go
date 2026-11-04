// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// affectedOrder is the opt-in fail-fast order of a compact plan's selection
// (TSP-V0-7305): the units a recent local `corvint test` receipt records as
// failing first, each group in plan.selected order. It only reorders: it
// names exactly the selected units and changes no other member.
type affectedOrder struct {
	Key      []string              `json:"key"`
	Units    []affectedOrderUnit   `json:"units"`
	Failures affectedOrderFailures `json:"failures"`
}

type affectedOrderUnit struct {
	UnitID        string `json:"unitId"`
	RecentFailure bool   `json:"recentFailure"`
	PlanIndex     int    `json:"planIndex"`
}

// affectedOrderFailures discloses the local state the order read: OBSERVED
// when at least one receipt was read, UNREADABLE (with Detail) when the
// receipt directory exists but cannot be listed, NOT_OBSERVED otherwise.
type affectedOrderFailures struct {
	State    string                 `json:"state"`
	Detail   string                 `json:"detail,omitempty"`
	Receipts []affectedOrderReceipt `json:"receipts"`
}

type affectedOrderReceipt struct {
	// Path is relative to the Git directory.
	Path     string   `json:"path"`
	State    string   `json:"state"`
	Detail   string   `json:"detail,omitempty"`
	SHA256   string   `json:"sha256,omitempty"`
	Revision string   `json:"revision,omitempty"`
	Failed   []string `json:"failed"`
}

const (
	affectedOrderObserved    = "OBSERVED"
	affectedOrderNotObserved = "NOT_OBSERVED"
	affectedOrderUnreadable  = "UNREADABLE"
	// maxFailFastReceipts bounds the receipts read; maxFailFastReceiptBytes
	// bounds each one.
	maxFailFastReceipts     = 64
	maxFailFastReceiptBytes = 64 << 20
)

var affectedOrderKey = []string{"recent-local-failure", "plan-order"}

// failFastReceiptName matches the receipt names `corvint test` writes:
// test-run.json and test-run.<shard>.json.
var failFastReceiptName = regexp.MustCompile(`^test-run(\.[0-9]+)?\.json$`)

// affectedFailFastOrder adds the fail-fast order to a compact receipt, reading
// the `corvint test` receipts under root's Git directory (TSP-V0-7305). Every
// other member is returned unchanged.
func affectedFailFastOrder(ctx context.Context, root string, receipt any) (any, error) {
	compact, ok := receipt.(affectedCompactReceipt)
	if !ok {
		return nil, argumentError("--fail-fast-order has no order for this receipt")
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	gitDir, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return nil, errors.New("--fail-fast-order cannot find the Git directory: " + err.Error())
	}
	receipts, unlisted := readFailFastReceipts(strings.TrimSpace(string(gitDir)))
	compact.Order = orderAffectedSelection(compact.Plan.Selected, receipts)
	if unlisted != "" {
		compact.Order.Failures.State, compact.Order.Failures.Detail = affectedOrderUnreadable, unlisted
	}
	return compact, nil
}

// orderAffectedSelection puts every unit some receipt records as failed first,
// keeping plan order within each group.
func orderAffectedSelection(selected []affectedCompactSelection, receipts []affectedOrderReceipt) *affectedOrder {
	failed := map[string]bool{}
	state := affectedOrderNotObserved
	for _, receipt := range receipts {
		if receipt.State == affectedOrderObserved {
			state = affectedOrderObserved
		}
		for _, unit := range receipt.Failed {
			failed[unit] = true
		}
	}
	order := &affectedOrder{
		Key:      slices.Clone(affectedOrderKey),
		Units:    make([]affectedOrderUnit, 0, len(selected)),
		Failures: affectedOrderFailures{State: state, Receipts: receipts},
	}
	if order.Failures.Receipts == nil {
		order.Failures.Receipts = []affectedOrderReceipt{}
	}
	for index, selection := range selected {
		order.Units = append(order.Units, affectedOrderUnit{UnitID: selection.UnitID, RecentFailure: failed[selection.UnitID], PlanIndex: index})
	}
	sort.SliceStable(order.Units, func(left, right int) bool {
		return order.Units[left].RecentFailure && !order.Units[right].RecentFailure
	})
	return order
}

// readFailFastReceipts reads at most maxFailFastReceipts `corvint test`
// receipts under gitDir/corvint: newest completedAt, or mtime when absent, ties broken by
// name, listed in name order. A receipt that cannot be read, is not canonical,
// or changes while read is UNREADABLE with its reason and contributes no
// failure. An absent directory reads nothing; a directory that exists but
// cannot be listed returns no receipt and the reason as unlisted.
func readFailFastReceipts(gitDir string) (receipts []affectedOrderReceipt, unlisted string) {
	directory := filepath.Join(gitDir, "corvint")
	entries, err := readFailFastDirectory(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ""
	}
	if err != nil {
		return nil, "the receipt directory corvint/ cannot be listed"
	}
	type candidate struct {
		name     string
		modified time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !failFastReceiptName.MatchString(entry.Name()) {
			continue
		}
		// A metadata failure is disclosed even if the receipt is outside the
		// retained set (TSP-V0-7305); it contributes no recency claim.
		var modified time.Time
		if info, err := entry.Info(); err == nil {
			modified = info.ModTime()
			if completed, ok := failFastContentTime(filepath.Join(directory, entry.Name())); ok {
				modified = completed
			}
		} else {
			if unlisted != "" {
				unlisted += "; "
			}
			if len(unlisted) < 4096 {
				unlisted += "receipt metadata unreadable: corvint/" + entry.Name()
			}
		}
		candidates = append(candidates, candidate{entry.Name(), modified})
	}
	slices.SortFunc(candidates, func(left, right candidate) int {
		if order := right.modified.Compare(left.modified); order != 0 {
			return order
		}
		return strings.Compare(left.name, right.name)
	})
	if len(candidates) > maxFailFastReceipts {
		candidates = candidates[:maxFailFastReceipts]
	}
	names := make([]string, 0, len(candidates))
	for _, kept := range candidates {
		names = append(names, kept.name)
	}
	sort.Strings(names)
	receipts = make([]affectedOrderReceipt, 0, len(names))
	for _, name := range names {
		receipts = append(receipts, readFailFastReceipt(directory, name))
	}
	return receipts, unlisted
}

func readFailFastReceipt(directory, name string) affectedOrderReceipt {
	result := affectedOrderReceipt{Path: "corvint/" + name, State: affectedOrderUnreadable, Failed: []string{}}
	full := filepath.Join(directory, name)
	file, err := os.Open(full)
	if err != nil {
		result.Detail = "cannot be opened"
		return result
	}
	body, err := io.ReadAll(io.LimitReader(file, maxFailFastReceiptBytes+1))
	_ = file.Close()
	if err != nil || len(body) > maxFailFastReceiptBytes {
		result.Detail = "cannot be read within the size bound"
		return result
	}
	receipt, err := sessiontest.ReadReceipt(full)
	if err != nil {
		result.Detail = "is not a canonical corvint-test-run receipt"
		return result
	}
	if encoded, err := sessiontest.Encode(receipt); err != nil || !bytes.Equal(encoded, body) {
		result.Detail = "changed while read"
		return result
	}
	sum := sha256.Sum256(body)
	failed := map[string]bool{}
	for _, invocation := range receipt.Invocations {
		for _, outcome := range invocation.Outcomes {
			switch outcome.Outcome {
			case sessiontest.OutcomeFail, sessiontest.OutcomeBuildFailed, sessiontest.OutcomeTimedOut:
				failed[outcome.Unit] = true
			}
		}
	}
	result.State, result.SHA256, result.Revision = affectedOrderObserved, hex.EncodeToString(sum[:]), receipt.Revision
	for unit := range failed {
		result.Failed = append(result.Failed, unit)
	}
	sort.Strings(result.Failed)
	return result
}

var readFailFastDirectory = os.ReadDir

// failFastContentTime reads only a bounded header for ordering; a missing,
// invalid or over-bound timestamp falls back to mtime. Strict receipt admission
// still happens after retention. completedAt precedes invocations in canonical JSON.
func failFastContentTime(name string) (time.Time, bool) {
	file, err := os.Open(name)
	if err != nil {
		return time.Time{}, false
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return time.Time{}, false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			break
		}
		if key == "completedAt" {
			var stamp string
			if decoder.Decode(&stamp) != nil {
				break
			}
			parsed, err := time.Parse(time.RFC3339Nano, stamp)
			return parsed, err == nil && !parsed.IsZero()
		}
		var skip json.RawMessage
		if decoder.Decode(&skip) != nil {
			break
		}
	}
	return time.Time{}, false
}
