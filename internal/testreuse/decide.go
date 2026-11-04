// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"fmt"
	"slices"
)

// Policy names the record sources a decision admits (AFP-V0-2324). The
// caller that chose which records to read is the trust boundary: in CI the
// workflow lists only runs of this repository by event, branch and pull
// request; Policy then refuses any record whose own source disagrees.
type Policy struct {
	Trust       map[string]bool
	PullRequest int
}

// RecordInput is one record file as read, before admission.
type RecordInput struct {
	Name string
	Body []byte
	// Err is a read failure; the record is then refused as invalid.
	Err error
}

// RecordVerdict is one record's admission.
type RecordVerdict struct {
	Name     string  `json:"name"`
	Verdict  string  `json:"verdict"`
	Detail   string  `json:"detail,omitempty"`
	Source   *Source `json:"source,omitempty"`
	Packages int     `json:"packages"`
}

// Witness names the record a REUSE decision rests on.
type Witness struct {
	Record      string `json:"record"`
	Kind        string `json:"kind"`
	PullRequest int    `json:"pullRequest,omitempty"`
	RunID       int64  `json:"runId,omitempty"`
	RunAttempt  int    `json:"runAttempt,omitempty"`
	Commit      string `json:"commit"`
}

// Unit is one package's decision.
type Unit struct {
	Package  string   `json:"package"`
	Decision string   `json:"decision"`
	Reason   string   `json:"reason"`
	Detail   string   `json:"detail,omitempty"`
	Digest   string   `json:"digest,omitempty"`
	Witness  *Witness `json:"witness,omitempty"`
}

// Decision is one corvint-test-reuse-decision/0 document (AFP-V0-2325).
type Decision struct {
	Profile        string          `json:"profile"`
	Head           string          `json:"head"`
	Tree           string          `json:"tree"`
	Identity       Identity        `json:"identity"`
	IdentityDigest string          `json:"identityDigest"`
	Records        []RecordVerdict `json:"records"`
	Units          []Unit          `json:"units"`
	Reused         []string        `json:"reused"`
	Run            []string        `json:"run"`
}

// Decide decides each package of packages over snapshot (AFP-V0-2325). A
// package is REUSE only when it is eligible, its digest resolves, and an
// admitted record passed it under the same identity with the same digest.
func Decide(snapshot *Snapshot, identity Identity, policy Policy, records []RecordInput, packages []string) (Decision, error) {
	identityDigest, err := identity.Digest()
	if err != nil {
		return Decision{}, err
	}
	if len(packages) == 0 {
		return Decision{}, fmt.Errorf("no packages to decide")
	}
	decision := Decision{Profile: DecisionProfile, Head: snapshot.Head, Tree: snapshot.Tree, Identity: identity, IdentityDigest: identityDigest,
		Records: []RecordVerdict{}, Units: []Unit{}, Reused: []string{}, Run: []string{}}
	type admitted struct {
		name   string
		record Record
		passed map[string]string
	}
	var trusted []admitted
	for _, input := range records {
		verdict := RecordVerdict{Name: input.Name}
		record, code, err := Record{}, RecordInvalid, input.Err
		if err == nil {
			record, code, err = ParseRecord(input.Body)
		}
		switch {
		case err != nil:
			verdict.Verdict, verdict.Detail = code, err.Error()
		case !policy.Trust[record.Source.Kind]:
			verdict.Verdict, verdict.Detail = RecordUntrusted, "source kind "+record.Source.Kind+" is not trusted here"
		case record.Source.Kind == SourcePullRequest && record.Source.PullRequest != policy.PullRequest:
			verdict.Verdict, verdict.Detail = RecordUntrusted, fmt.Sprintf("pull request %d is not this pull request", record.Source.PullRequest)
		default:
			verdict.Verdict = RecordAdmitted
			passed := map[string]string{}
			for _, p := range record.Packages {
				passed[p.Package] = p.Digest
			}
			trusted = append(trusted, admitted{name: input.Name, record: record, passed: passed})
		}
		if err == nil {
			source := record.Source
			verdict.Source, verdict.Packages = &source, len(record.Packages)
		}
		decision.Records = append(decision.Records, verdict)
	}
	seen := map[string]bool{}
	for _, name := range packages {
		if seen[name] {
			continue
		}
		seen[name] = true
		unit := Unit{Package: name, Decision: DecisionRun}
		digest := DigestPackage(snapshot, name, identityDigest, identity.Confine)
		switch {
		case digest.Digest == "":
			unit.Reason, unit.Detail = digest.Reason, digest.Detail
		default:
			unit.Digest, unit.Reason = digest.Digest, ReasonNoRecord
			for _, candidate := range trusted {
				recorded, ok := candidate.passed[name]
				if !ok {
					continue
				}
				switch {
				case candidate.record.IdentityDigest != identityDigest:
					if unit.Reason == ReasonNoRecord {
						unit.Reason = ReasonIdentityChanged
					}
				case recorded != digest.Digest:
					if unit.Reason != ReasonDigestChanged {
						unit.Reason = ReasonDigestChanged
					}
				default:
					source := candidate.record.Source
					unit.Decision, unit.Reason = DecisionReuse, ReasonRecordDigestEqual
					unit.Witness = &Witness{Record: candidate.name, Kind: source.Kind, PullRequest: source.PullRequest, RunID: source.RunID, RunAttempt: source.RunAttempt, Commit: source.Commit}
				}
				if unit.Decision == DecisionReuse {
					break
				}
			}
		}
		if unit.Decision == DecisionReuse {
			decision.Reused = append(decision.Reused, name)
		} else {
			decision.Run = append(decision.Run, name)
		}
		decision.Units = append(decision.Units, unit)
	}
	slices.Sort(decision.Reused)
	return decision, nil
}
