// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/Beamfall/corvint/internal/testid"
)

// The reasons a reader prints (TIV-V1-006 to TIV-V1-011).
const (
	// ReasonAbsent: no inventory, or no record for a declared instance; the
	// instance runs its declared full command (TIV-V1-006).
	ReasonAbsent = "inventory-absent"
	// ReasonStale: the instance's config, lockfile, lister or profile moved
	// since refresh; every test of the instance runs (TIV-V1-007).
	ReasonStale = "inventory-stale"
	// ReasonUnassigned: a listed test file changed, or a new one appeared;
	// that file's tests run (TIV-V1-008).
	ReasonUnassigned = "unassigned"
	// ReasonInvalid: the file cannot be read as this profile; every test of
	// every declared instance runs (TIV-V1-009).
	ReasonInvalid = "inventory-invalid"
	// ReasonDrift: HEAD or the sources moved while refresh listed
	// (TIV-V1-010).
	ReasonDrift = "inventory-refresh-drift"
	// ReasonTooLarge: the refreshed inventory would exceed MaxBytes
	// (TIV-V1-011).
	ReasonTooLarge = "inventory-too-large"
	// ReasonSelectionFull: the caller forced every instance of a runner to
	// run in full (TIV-V1-003).
	ReasonSelectionFull = "selection-full"
)

// VerdictProfile names the reader's verdict document.
const VerdictProfile = "corvint-test-inventory-verdict/0"

// The file-level states of a verdict.
const (
	StateRead    = "read"
	StateAbsent  = "absent"
	StateInvalid = "invalid"
)

// Verdict is what a reader concludes from the inventory and the tree it
// plans: one InstanceVerdict per declared instance, sorted by key. A
// verdict never narrows: an instance is either Full (every test runs, with
// its reason) or planned from its record with every Unassigned file's tests
// added. Undeclared names records the tree declares no instance for; they
// are ignored for selection.
type Verdict struct {
	Profile    string            `json:"profile"`
	Path       string            `json:"path"`
	State      string            `json:"state"`
	Reason     string            `json:"reason,omitzero"`
	Detail     string            `json:"detail,omitzero"`
	Instances  []InstanceVerdict `json:"instances"`
	Undeclared []string          `json:"undeclared,omitzero"`
}

// InstanceVerdict is one declared instance's state. Full instances run
// their declared full command; Reason names why. Otherwise Tests counts the
// listed tests whose file is unchanged, Unassigned lists the changed or new
// test files whose tests run as unassigned, and Removed lists listed files
// the tree no longer holds.
type InstanceVerdict struct {
	Key        string   `json:"key"`
	Runner     string   `json:"runner"`
	Root       string   `json:"root"`
	Config     string   `json:"config"`
	Full       bool     `json:"full"`
	Reason     string   `json:"reason,omitzero"`
	Cause      string   `json:"cause,omitzero"`
	Tests      int      `json:"tests"`
	Unassigned []string `json:"unassigned,omitzero"`
	Removed    []string `json:"removed,omitzero"`
}

// Judge reads the inventory from tree and judges it against the instances
// tree declares (TIV-V1-006 to TIV-V1-009). It reads only; it runs no lister
// and writes nothing. A tree it cannot list judges every instance it can
// still declare, which is none, invalid.
func Judge(tree Tree) Verdict {
	files, err := tree.Files()
	if err != nil {
		return Verdict{Profile: VerdictProfile, Path: Path, State: StateInvalid, Reason: ReasonInvalid, Detail: "cannot list the tree: " + err.Error(), Instances: []InstanceVerdict{}}
	}
	declared := Detect(files, tree.Read)
	data, err := tree.Read(Path, MaxBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return JudgeData(nil, false, nil, files, declared)
	}
	return JudgeData(data, true, err, files, declared)
}

// JudgeData is Judge over data already read: present false means no
// inventory file, readErr a file that exists but could not be read.
func JudgeData(data []byte, present bool, readErr error, files map[string]string, declared []Declared) Verdict {
	verdict := Verdict{Profile: VerdictProfile, Path: Path, State: StateRead, Instances: []InstanceVerdict{}}
	whole := func(state, reason, detail string) Verdict {
		verdict.State, verdict.Reason, verdict.Detail = state, reason, detail
		for _, instance := range declared {
			verdict.Instances = append(verdict.Instances, fullInstance(instance, reason, detail))
		}
		return verdict
	}
	if !present {
		return whole(StateAbsent, ReasonAbsent, "no "+Path+" in the planned tree; run corvint inventory refresh and commit it")
	}
	if readErr != nil {
		return whole(StateInvalid, ReasonInvalid, "cannot read "+Path+": "+readErr.Error())
	}
	inventory, err := Decode(data)
	if err != nil {
		return whole(StateInvalid, ReasonInvalid, err.Error())
	}
	records := map[string]Instance{}
	for _, instance := range inventory.Instances {
		records[instance.Key] = instance
	}
	failures := map[string]Failure{}
	for _, failure := range inventory.Failures {
		failures[failure.Key] = failure
	}
	keys := map[string]bool{}
	for _, instance := range declared {
		keys[instance.Key] = true
		record, ok := records[instance.Key]
		if !ok {
			cause := "the inventory has no record for this instance"
			if failure, failed := failures[instance.Key]; failed {
				cause = "its lister failed at refresh: " + failure.Error
			}
			verdict.Instances = append(verdict.Instances, fullInstance(instance, ReasonAbsent, cause))
			continue
		}
		verdict.Instances = append(verdict.Instances, judgeInstance(instance, record, files, declared))
	}
	for _, instance := range inventory.Instances {
		if !keys[instance.Key] {
			verdict.Undeclared = append(verdict.Undeclared, instance.Key)
		}
	}
	return verdict
}

func fullInstance(instance Declared, reason, cause string) InstanceVerdict {
	return InstanceVerdict{Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config, Full: true, Reason: reason, Cause: cause}
}

// judgeInstance applies TIV-V1-007 then TIV-V1-008 to one record.
func judgeInstance(instance Declared, record Instance, files map[string]string, declared []Declared) InstanceVerdict {
	stale := func(cause string) InstanceVerdict { return fullInstance(instance, ReasonStale, cause) }
	if record.Config != instance.Config {
		return stale(fmt.Sprintf("config is %s, the record lists %s", instance.Config, record.Config))
	}
	if files[instance.Config] != record.ConfigDigest {
		return stale("config " + instance.Config + " changed since refresh")
	}
	if record.Lockfile != instance.Lockfile {
		return stale(fmt.Sprintf("lockfile is %q, the record lists %q", instance.Lockfile, record.Lockfile))
	}
	if instance.Lockfile != "" && files[instance.Lockfile] != record.LockfileDigest {
		return stale("lockfile " + instance.Lockfile + " changed since refresh")
	}
	if current, ok := CurrentLister(instance.Runner); !ok || record.Lister.Name != current.Name || record.Lister.Version != current.Version || record.Lister.Kind != current.Kind {
		return stale(fmt.Sprintf("lister %s %s (%s) is not this binary's %s %s (%s)", record.Lister.Name, record.Lister.Version, record.Lister.Kind, current.Name, current.Version, current.Kind))
	}
	verdict := InstanceVerdict{Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config}
	recorded := map[string]string{}
	for _, file := range record.Files {
		recorded[file.Path] = file.Blob
	}
	changed := map[string]bool{}
	for _, file := range record.Files {
		blob, ok := files[file.Path]
		switch {
		case !ok:
			verdict.Removed = append(verdict.Removed, file.Path)
		case blob != file.Blob:
			changed[file.Path] = true
		}
	}
	for _, name := range Candidates(files, instance, declared) {
		if _, ok := recorded[name]; !ok {
			changed[name] = true
		}
	}
	for name := range changed {
		verdict.Unassigned = append(verdict.Unassigned, name)
	}
	sort.Strings(verdict.Unassigned)
	for _, test := range record.Tests {
		if _, present := files[test.File]; present && !changed[test.File] {
			verdict.Tests++
		}
	}
	return verdict
}

// ForceFull makes every instance of runner run in full with
// ReasonSelectionFull (TIV-V1-003): `CORVINT_SELECTION=full:<runner>` names a
// runner, never one instance. An instance already full keeps its reason.
func (verdict *Verdict) ForceFull(runner string) {
	for index := range verdict.Instances {
		instance := &verdict.Instances[index]
		if instance.Runner == runner && !instance.Full {
			instance.Full, instance.Reason, instance.Cause = true, ReasonSelectionFull, "every instance of "+runner+" runs in full"
			instance.Tests, instance.Unassigned, instance.Removed = 0, nil, nil
		}
	}
}

// Reasons lists each distinct reason the verdict prints, sorted: the file's
// own and every instance's, plus ReasonUnassigned when a file is unassigned.
func (verdict Verdict) Reasons() []string {
	seen := map[string]bool{}
	if verdict.Reason != "" {
		seen[verdict.Reason] = true
	}
	for _, instance := range verdict.Instances {
		if instance.Reason != "" {
			seen[instance.Reason] = true
		}
		if len(instance.Unassigned) != 0 {
			seen[ReasonUnassigned] = true
		}
	}
	reasons := make([]string, 0, len(seen))
	for reason := range seen {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

// ListerIdentity is a lister's name, version and kind (TIV-V1-004).
type ListerIdentity struct {
	Name    string
	Version string
	Kind    string
}

// listers is this binary's lister per runner. Bumping a version makes every
// record of that runner stale (TIV-V1-007).
var listers = map[string]ListerIdentity{
	testid.Go:         {Name: "go-ast", Version: "1", Kind: KindStatic},
	testid.Vitest:     {Name: "vitest-list", Version: "1", Kind: KindExecuting},
	testid.Jest:       {Name: "jest-list-tests", Version: "1", Kind: KindExecuting},
	testid.Playwright: {Name: "playwright-list", Version: "1", Kind: KindExecuting},
	testid.Pytest:     {Name: "pytest-collect", Version: "1", Kind: KindExecuting},
	testid.RSpec:      {Name: "rspec-static", Version: "1", Kind: KindStatic},
	testid.Minitest:   {Name: "minitest-static", Version: "1", Kind: KindStatic},
}

// CurrentLister is this binary's lister for runner.
func CurrentLister(runner string) (ListerIdentity, bool) {
	identity, ok := listers[runner]
	return identity, ok
}
