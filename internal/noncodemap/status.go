// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"fmt"
	"slices"
)

// Entry statuses. Only StatusProven narrows (NCM-V1-004).
const (
	// StatusProposed is an entry missing its proof or an acceptance member
	// (NCM-V1-002, NCM-V1-003).
	StatusProposed = "proposed"
	// StatusIgnored is an entry whose key is on the never-narrow list
	// (NCM-V1-005).
	StatusIgnored = "ignored"
	// StatusStale is an entry citing a test that is gone, or pinned at a
	// commit that is not an ancestor of the head (NCM-V1-008).
	StatusStale = "stale"
	// StatusUnproven is an accepted entry whose proof is not a confirmed
	// FIH-V1 non-code fault case killed by one of its tests (NCM-V1-004).
	StatusUnproven = "unproven"
	// StatusProven is an entry that narrows.
	StatusProven = "proven"
)

// FaultCase is what an FIH-V1 resolver knows of one proof identifier: the
// mapped file the case injected a fault into, whether a test killed it, and
// the killing tests.
type FaultCase struct {
	Path    string
	Killed  bool
	Killers []string
}

// Env holds the facts an entry's status needs. A nil member is a fact that
// cannot be read, which never narrows: a nil TestExists makes every cited
// test unconfirmed (stale), a nil IsAncestor every pinned commit (stale), and
// a nil FaultCase every proof (unproven), the state until FIH-V1 is linked
// (V1-5405).
type Env struct {
	TestExists func(test string) bool
	IsAncestor func(commit string) (bool, error)
	FaultCase  func(proof string) (FaultCase, bool)
}

// unlinkedProof is the detail of a proof no resolver confirms while FIH-V1 is
// not linked.
const unlinkedProof = "FIH-V1 is not linked (V1-5405)"

// Evaluate returns an entry's status and the reason for it, first match wins:
// proposed, ignored, stale, unproven, proven.
func Evaluate(entry Entry, env Env) (string, string) {
	var missing []string
	for _, member := range []struct{ name, value string }{
		{"proof", entry.Proof}, {"accepted_by", entry.AcceptedBy}, {"accepted_at", entry.AcceptedAt}, {"pinned_commit", entry.PinnedCommit},
	} {
		if member.value == "" {
			missing = append(missing, member.name)
		}
	}
	if len(missing) != 0 {
		return StatusProposed, fmt.Sprintf("no %s; a person must accept it and an FIH-V1 fault case must prove it", joinNames(missing))
	}
	if category, never := NeverNarrowed(entry.Key()); never {
		return StatusIgnored, fmt.Sprintf("%s is never narrowed (%s)", entry.Key(), category)
	}
	for _, test := range entry.Tests {
		if env.TestExists == nil || !env.TestExists(test) {
			return StatusStale, fmt.Sprintf("cited test %s is gone or cannot be confirmed", test)
		}
	}
	if env.IsAncestor == nil {
		return StatusStale, fmt.Sprintf("pinned commit %s cannot be checked against the head", entry.PinnedCommit)
	}
	ancestor, err := env.IsAncestor(entry.PinnedCommit)
	if err != nil {
		return StatusStale, fmt.Sprintf("pinned commit %s cannot be checked against the head: %v", entry.PinnedCommit, err)
	}
	if !ancestor {
		return StatusStale, fmt.Sprintf("pinned commit %s is not an ancestor of the head", entry.PinnedCommit)
	}
	if env.FaultCase == nil {
		return StatusUnproven, fmt.Sprintf("fault case %s is unknown: %s", entry.Proof, unlinkedProof)
	}
	fault, known := env.FaultCase(entry.Proof)
	switch {
	case !known:
		return StatusUnproven, fmt.Sprintf("fault case %s is not in the FIH-V1 non-code set", entry.Proof)
	case !entry.Covers(fault.Path):
		return StatusUnproven, fmt.Sprintf("fault case %s injected %s, which the entry does not map", entry.Proof, fault.Path)
	case !fault.Killed:
		return StatusUnproven, fmt.Sprintf("fault case %s killed no test", entry.Proof)
	}
	for _, killer := range fault.Killers {
		if slices.Contains(entry.Tests, killer) {
			return StatusProven, fmt.Sprintf("fault case %s was killed by %s", entry.Proof, killer)
		}
	}
	return StatusUnproven, fmt.Sprintf("fault case %s was killed by no test the entry lists", entry.Proof)
}

func joinNames(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	}
	out := ""
	for index, name := range names {
		switch {
		case index == len(names)-1:
			out += " or " + name
		case index > 0:
			out += ", " + name
		default:
			out += name
		}
	}
	return out
}
