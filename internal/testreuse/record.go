// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"bufio"
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
)

// MaxRecordBytes bounds one record document.
const MaxRecordBytes = 16 << 20

// Source names the run a record comes from (AFP-V0-2323).
type Source struct {
	Kind        string `json:"kind"`
	PullRequest int    `json:"pullRequest,omitempty"`
	RunID       int64  `json:"runId,omitempty"`
	RunAttempt  int    `json:"runAttempt,omitempty"`
	Commit      string `json:"commit"`
	Tree        string `json:"tree"`
}

// RecordedPackage is one passed package and its digest.
type RecordedPackage struct {
	Package string `json:"package"`
	Digest  string `json:"digest"`
	Outcome string `json:"outcome"`
}

// Skipped is one package of the run the record does not carry, and why.
type Skipped struct {
	Package string `json:"package"`
	Reason  string `json:"reason"`
	Detail  string `json:"detail,omitempty"`
}

// Record is one corvint-test-reuse-record/0 document.
type Record struct {
	Profile        string            `json:"profile"`
	Source         Source            `json:"source"`
	Identity       Identity          `json:"identity"`
	IdentityDigest string            `json:"identityDigest"`
	Packages       []RecordedPackage `json:"packages"`
	Skipped        []Skipped         `json:"skipped"`
}

// OutcomesProfile is the CI shard outcomes document (AFP-V0-047).
const OutcomesProfile = "corvint-ci-outcomes/0"

// Outcomes is the part of one corvint-ci-outcomes/0 document a record reads:
// the first run's outcome per package. The bounded rerun's outcomes are
// never read, so a package that passed only on its rerun is not passed.
type Outcomes struct {
	Profile  string            `json:"profile"`
	Event    string            `json:"event"`
	RunID    int64             `json:"runId"`
	Attempt  int               `json:"runAttempt"`
	Target   string            `json:"target"`
	Tree     *string           `json:"tree"`
	Outcomes map[string]string `json:"outcomes"`
}

// ParseOutcomes reads one shard outcomes document.
func ParseOutcomes(body []byte) (Outcomes, error) {
	var doc Outcomes
	if err := json.Unmarshal(body, &doc); err != nil {
		return doc, err
	}
	if doc.Profile != OutcomesProfile || !hexObject.MatchString(doc.Target) || doc.Outcomes == nil {
		return doc, fmt.Errorf("not a %s document with a target commit and outcomes", OutcomesProfile)
	}
	return doc, nil
}

// MergeOutcomes joins shard documents of one run at commit and tree: a
// package passes only when every document naming it says pass.
func MergeOutcomes(documents []Outcomes, commit, tree string) (map[string]string, error) {
	merged := map[string]string{}
	for _, doc := range documents {
		if doc.Target != commit {
			return nil, fmt.Errorf("outcomes of run %d name target %s, not HEAD %s", doc.RunID, doc.Target, commit)
		}
		if doc.Tree != nil && *doc.Tree != "" && *doc.Tree != tree {
			return nil, fmt.Errorf("outcomes of run %d name tree %s, not HEAD's tree %s", doc.RunID, *doc.Tree, tree)
		}
		for name, outcome := range doc.Outcomes {
			if previous, seen := merged[name]; seen && previous != outcome {
				merged[name] = "conflict"
				continue
			}
			merged[name] = outcome
		}
	}
	return merged, nil
}

// ParseTestJSON reads a `go test -json` stream: a package passes when its
// package-level action is pass and no action of it failed.
func ParseTestJSON(stream []byte) (map[string]string, error) {
	outcomes := map[string]string{}
	failed := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(stream))
	scanner.Buffer(make([]byte, 0, 1<<16), 16<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var event struct {
			Action  string `json:"Action"`
			Package string `json:"Package"`
			Test    string `json:"Test"`
		}
		if err := json.Unmarshal(line, &event); err != nil || event.Package == "" {
			continue
		}
		switch event.Action {
		case "fail":
			failed[event.Package] = true
		case "pass", "skip":
			if event.Test == "" {
				outcomes[event.Package] = event.Action
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for name := range failed {
		outcomes[name] = "fail"
	}
	return outcomes, nil
}

// BuildRecord digests every passed package of outcomes over snapshot
// (AFP-V0-2323). Only an eligible package whose outcome is pass is carried;
// every other package of the run is listed in Skipped with its reason.
func BuildRecord(snapshot *Snapshot, identity Identity, source Source, outcomes map[string]string) (Record, error) {
	identityDigest, err := identity.Digest()
	if err != nil {
		return Record{}, err
	}
	if err := validSource(source); err != nil {
		return Record{}, err
	}
	if source.Commit != snapshot.Head || source.Tree != snapshot.Tree {
		return Record{}, fmt.Errorf("the record source names %s/%s, but the checkout is %s/%s", source.Commit, source.Tree, snapshot.Head, snapshot.Tree)
	}
	record := Record{Profile: RecordProfile, Source: source, Identity: identity, IdentityDigest: identityDigest, Packages: []RecordedPackage{}, Skipped: []Skipped{}}
	for _, name := range sortedKeys(outcomes) {
		if outcomes[name] != "pass" {
			record.Skipped = append(record.Skipped, Skipped{Package: name, Reason: ReasonNotPassed, Detail: outcomes[name]})
			continue
		}
		digest := DigestPackage(snapshot, name, identityDigest, identity.Confine)
		if digest.Digest == "" {
			record.Skipped = append(record.Skipped, Skipped{Package: name, Reason: digest.Reason, Detail: digest.Detail})
			continue
		}
		record.Packages = append(record.Packages, RecordedPackage{Package: name, Digest: digest.Digest, Outcome: "pass"})
	}
	return record, nil
}

var sourceKinds = []string{SourceMain, SourcePullRequest, SourceLocal}

func validSource(source Source) error {
	switch {
	case !slices.Contains(sourceKinds, source.Kind):
		return fmt.Errorf("source kind %q is not main, pull_request or local", source.Kind)
	case source.Kind == SourcePullRequest && source.PullRequest <= 0:
		return errors.New("a pull_request source needs its pull request number")
	case source.Kind != SourcePullRequest && source.PullRequest != 0:
		return errors.New("only a pull_request source names a pull request")
	case !hexObject.MatchString(source.Commit) || !hexObject.MatchString(source.Tree):
		return errors.New("the source needs a full commit and tree id")
	case source.RunID < 0 || source.RunAttempt < 0:
		return errors.New("the source run id and attempt are not negative")
	}
	return nil
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ParseRecord reads and validates one record. Its identity digest must equal
// the digest of its own identity, so an edited identity is refused.
func ParseRecord(body []byte) (Record, string, error) {
	if len(body) > MaxRecordBytes {
		return Record{}, RecordInvalid, fmt.Errorf("record exceeds %d bytes", MaxRecordBytes)
	}
	var record Record
	if err := json.Unmarshal(body, &record, json.RejectUnknownMembers(true)); err != nil {
		return Record{}, RecordInvalid, err
	}
	if record.Profile != RecordProfile || record.Identity.Profile != IdentityProfile || record.Packages == nil {
		return Record{}, RecordInvalid, fmt.Errorf("not a %s document", RecordProfile)
	}
	if err := validSource(record.Source); err != nil {
		return Record{}, RecordInvalid, err
	}
	if err := CheckArgv(record.Identity.Argv); err != nil {
		return Record{}, RecordInvalid, err
	}
	seen := map[string]bool{}
	for _, p := range record.Packages {
		if p.Package == "" || seen[p.Package] || p.Outcome != "pass" || !digestPattern.MatchString(p.Digest) {
			return Record{}, RecordInvalid, fmt.Errorf("record package %q is invalid or repeated", p.Package)
		}
		seen[p.Package] = true
	}
	digest, err := record.Identity.Digest()
	if err != nil {
		return Record{}, RecordInvalid, err
	}
	if digest != record.IdentityDigest {
		return Record{}, RecordIdentityForged, errors.New("the record's identity digest is not the digest of its identity")
	}
	sort.Slice(record.Packages, func(i, j int) bool { return record.Packages[i].Package < record.Packages[j].Package })
	return record, RecordAdmitted, nil
}
