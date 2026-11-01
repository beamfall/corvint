package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	registrationProfile = "corvint-cem-trial-registration/0"
	maxRegistrationSize = 64 << 10
	registeredMetric    = "miss"
	registeredAlpha     = 0.05
)

// scorerFiles are the non-test sources the scorer digest covers, in the order
// they are concatenated: `cat` of these files, in this order, piped through
// sha256 reproduces scorerDigest (CRT-V0-014).
var scorerFiles = []string{"gold.go", "main.go", "register.go", "score.go", "select.go"}

//go:embed gold.go main.go register.go score.go select.go
var scorerSources embed.FS

// scorerDigest is the sha256 of the scorer sources this binary was built
// from, so a registration can pin the scorer before selection and a later
// `score --report` under a changed scorer reads as a changed scorer.
func scorerDigest() string {
	hash := sha256.New()
	for _, name := range scorerFiles {
		data, err := scorerSources.ReadFile(name)
		if err != nil {
			panic("cem-trial: embedded scorer source missing: " + name)
		}
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// registration is the preregistered analysis plan (CRT-V0-014): a closed set
// of fields, committed before selection, that a run must match.
type registration struct {
	Profile       string   `json:"profile"`
	Hypotheses    []string `json:"hypotheses"`
	PrimaryMetric string   `json:"primary_metric"`
	Alpha         float64  `json:"alpha"`
	N             int      `json:"n"`
	ScorerSHA256  string   `json:"scorer_sha256"`
	Model         string   `json:"model"`
	Effort        string   `json:"effort"`
	Partition     string   `json:"partition"`
}

// registrationRecord is the registration as the report carries it: the file's
// repository-relative path and digest, the commit that last wrote it, and that
// commit's committer time, so `score --report` re-checks the order without Git.
type registrationRecord struct {
	Path        string       `json:"path"`
	SHA256      string       `json:"sha256"`
	Commit      string       `json:"commit"`
	CommittedAt string       `json:"committed_at"`
	Document    registration `json:"document"`
}

var lowerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// UnmarshalJSON decodes the record a report carries as strictly as
// parseRegistration decodes the file, so `score --report` refuses a carried
// registration with a key the closed object does not define (CRT-V0-014).
func (record *registrationRecord) UnmarshalJSON(data []byte) error {
	type plain registrationRecord
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("registration record: %w", err)
	}
	*record = registrationRecord(value)
	return nil
}

// parseRegistration decodes one registration object with no unknown field and
// checks each field's shape.
func parseRegistration(data []byte) (registration, error) {
	var document registration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("registration: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return document, errors.New("registration: trailing data after the object")
	}
	if problems := registrationProblems(document); len(problems) != 0 {
		return document, errors.New("registration: " + strings.Join(problems, "; "))
	}
	return document, nil
}

// registrationProblems checks each field's shape and fixed value, sorted, so
// loading a registration and rescoring a report that carries one agree.
func registrationProblems(document registration) []string {
	problems := []string{}
	if document.Profile != registrationProfile {
		problems = append(problems, "profile must be "+registrationProfile)
	}
	if len(document.Hypotheses) == 0 {
		problems = append(problems, "hypotheses must name at least one hypothesis")
	}
	for _, item := range document.Hypotheses {
		if strings.TrimSpace(item) == "" {
			problems = append(problems, "hypotheses must not hold a blank entry")
			break
		}
	}
	if document.PrimaryMetric != registeredMetric {
		problems = append(problems, "primary_metric must be "+registeredMetric+", the only primary metric this scorer estimates")
	}
	if document.Alpha != registeredAlpha {
		problems = append(problems, "alpha must be 0.05, the level of the scorer's fixed 95% interval")
	}
	if document.N < 1 {
		problems = append(problems, "n must be a positive pair count")
	}
	if !lowerHex64.MatchString(document.ScorerSHA256) {
		problems = append(problems, "scorer_sha256 must be 64 lowercase hex digits")
	}
	for name, value := range map[string]string{"model": document.Model, "effort": document.Effort, "partition": document.Partition} {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, name+" must not be empty")
		}
	}
	sort.Strings(problems)
	return problems
}

// loadRegistration reads a registration file that must be committed, byte for
// byte, at HEAD of the Git repository holding it, and records the commit that
// last wrote it with that commit's committer time.
func loadRegistration(ctx context.Context, path string) (*registrationRecord, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if absolute, err = filepath.EvalSymlinks(absolute); err != nil {
		return nil, err
	}
	data, err := readBounded(absolute, maxRegistrationSize)
	if err != nil {
		return nil, err
	}
	document, err := parseRegistration(data)
	if err != nil {
		return nil, err
	}
	top, err := gitOutput(ctx, filepath.Dir(absolute), "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("registration %s is not inside a Git repository: %w", path, err)
	}
	top = strings.TrimSpace(top)
	relative, err := filepath.Rel(top, absolute)
	if err != nil {
		return nil, err
	}
	relative = filepath.ToSlash(relative)
	// Both lookups run from the repository root, where the root-relative
	// path names the file whatever directory holds it.
	committed, err := gitBytes(ctx, top, "cat-file", "blob", "HEAD:"+relative)
	if err != nil || !bytes.Equal(committed, data) {
		return nil, fmt.Errorf("registration %s must be committed at HEAD exactly as it is on disk", path)
	}
	line, err := gitOutput(ctx, top, "log", "-1", "--format=%H %cI", "HEAD", "--", relative)
	fields := strings.Fields(line)
	if err != nil || len(fields) != 2 {
		return nil, fmt.Errorf("registration %s: cannot read the commit that wrote it", path)
	}
	when, err := time.Parse(time.RFC3339, fields[1])
	if err != nil {
		return nil, fmt.Errorf("registration %s: unreadable commit time %q", path, fields[1])
	}
	return &registrationRecord{
		Path: relative, SHA256: digestOf(string(data)), Commit: fields[0],
		CommittedAt: when.UTC().Format(time.RFC3339), Document: document,
	}, nil
}

// registrationMismatches lists every way the report's run disagrees with its
// registration: a malformed or unfixed field (a report is decoded without
// parseRegistration), the registered values, the running scorer, and the arms,
// (CRT-V0-014). Since 2026-10-24 the cem/1.0 map arms may be registered (CRT-V0-015).
func registrationMismatches(document *report, record *registrationRecord) []string {
	plan := record.Document
	problems := registrationProblems(plan)
	compare := func(name, registered, observed string) {
		if registered != observed {
			problems = append(problems, fmt.Sprintf("registered %s %q, run %q", name, registered, observed))
		}
	}
	compare("partition", plan.Partition, document.Partition)
	compare("model", plan.Model, document.Model)
	compare("effort", plan.Effort, document.Effort)
	compare("scorer_sha256", plan.ScorerSHA256, scorerDigest())
	if plan.N != document.Pairs {
		problems = append(problems, fmt.Sprintf("registered n %d, run %d pairs", plan.N, document.Pairs))
	}
	return problems
}

// registrationInvalid is the CRT-V0-010 reading of the registration: a held-out
// run without one, a registration the run does not match, or a registration
// committed after the run's first lane started makes the run invalid.
func registrationInvalid(document *report) []string {
	record := document.Registration
	if record == nil {
		// A stray re_registered block on an unregistered report is still
		// read, so it cannot pass unchecked (CRT-V0-016).
		problems := reRegistrationInvalid(document)
		if document.Partition == "heldout" {
			problems = append([]string{"a heldout run carries no registration (CRT-V0-014)"}, problems...)
		}
		return problems
	}
	problems := registrationMismatches(document, record)
	committed, err := time.Parse(time.RFC3339, record.CommittedAt)
	if err != nil {
		return append(problems, "the registration commit time is unreadable")
	}
	problems = append(problems, reRegistrationInvalid(document)...)
	first, observed := firstLaneStart(document)
	if document.ReRegistered != nil {
		// Lanes reused across the re-registration answer to the earlier one;
		// with none run fresh there is no lane for this one to precede.
		started := observed
		if first, observed = firstFreshLaneStart(document); !observed && started {
			first, observed = committed, true
		}
	}
	switch {
	case unstampedLanes(document) != 0:
		problems = append(problems, fmt.Sprintf("%d completed lane(s) recorded no start, so the registration order is unobserved",
			unstampedLanes(document)))
	case !observed:
		problems = append(problems, "no lane recorded its start, so the registration order is unobserved")
	case committed.After(first):
		problems = append(problems, "the registration commit "+record.Commit+" ("+record.CommittedAt+
			") is newer than the first lane ("+first.UTC().Format(time.RFC3339Nano)+")")
	}
	return problems
}

// firstLaneStart is the earliest recorded lane start; a lane with no readable
// start (an older harness, or a lane that never launched) is skipped.
func firstLaneStart(document *report) (time.Time, bool) {
	var first time.Time
	observed := false
	for _, item := range document.Lanes {
		when, err := time.Parse(time.RFC3339Nano, item.StartedAt)
		if err != nil {
			continue
		}
		if !observed || when.Before(first) {
			first, observed = when, true
		}
	}
	return first, observed
}

// registeredPrior refuses, for a registered run, a reused or resumed report
// whose lanes ran under another model or effort: its replies would enter a
// report registered for this run's settings (CRT-V0-014).
func registeredPrior(run, prior *report, path string) error {
	if run.Registration == nil || (prior.Model == run.Model && prior.Effort == run.Effort) {
		return nil
	}
	return fmt.Errorf("%s: a registered run reuses only lanes run under its model %q and effort %q, not %q and %q",
		path, run.Model, run.Effort, prior.Model, prior.Effort)
}

// unstampedLanes counts completed lanes (an observed wall time, no error)
// that carry no readable start, such as a lane reused from a report written
// before lanes recorded one: the earliest start would then miss a lane that
// may have run first. A never-started pending lane ran nothing, so it is not
// counted; an errored lane already makes the run invalid (CRT-V0-010).
func unstampedLanes(document *report) int {
	count := 0
	for _, item := range document.Lanes {
		if _, err := time.Parse(time.RFC3339Nano, item.StartedAt); err != nil && item.Error == "" && observedNumber(item.WallMs) {
			count++
		}
	}
	return count
}

// checkRegistration refuses, before any lane runs, a run its registration
// does not describe, and a heldout run with no registration at all.
func checkRegistration(ctx context.Context, configuration options, document *report) error {
	if configuration.registration == "" {
		if document.Partition == "heldout" {
			return errors.New("a heldout run needs --registration (CRT-V0-014)")
		}
		return nil
	}
	record, err := loadRegistration(ctx, configuration.registration)
	if err != nil {
		return err
	}
	if problems := registrationMismatches(document, record); len(problems) != 0 {
		return errors.New("the run does not match its registration: " + strings.Join(problems, "; "))
	}
	document.Registration = record
	return nil
}

// reRegistration is the earlier registration a re-declared run reuses lanes
// under, and the manifest those lanes ran from (CRT-V0-016).
type reRegistration struct {
	ManifestSHA256 string             `json:"manifest_sha256"`
	Registration   registrationRecord `json:"registration"`
}

// reRegister admits, for a registered run, a --reuse report written under an
// earlier registration that differs from this run's in the scorer digest
// alone, and records it so each reused lane is checked against the
// registration it ran under (CRT-V0-016). A report under the same
// registration, or a run with none, is unchanged.
func reRegister(run, prior *report, path string) error {
	// Two registration files committed together share a commit, so the same
	// registration is the same commit and an identical document.
	if run.Registration == nil || prior.Registration == nil ||
		prior.Registration.Commit == run.Registration.Commit && reflect.DeepEqual(prior.Registration.Document, run.Registration.Document) {
		return nil
	}
	if problems := reRegistrationProblems(run.Registration, prior.Registration); len(problems) != 0 {
		return fmt.Errorf("%s: %s", path, strings.Join(problems, "; "))
	}
	first, observed := firstLaneStart(prior)
	committed, err := time.Parse(time.RFC3339, prior.Registration.CommittedAt)
	switch {
	case err != nil:
		return fmt.Errorf("%s: the earlier registration commit time is unreadable", path)
	case !observed || unstampedLanes(prior) != 0:
		return fmt.Errorf("%s: the earlier run's registration order is unobserved", path)
	case committed.After(first):
		return fmt.Errorf("%s: the earlier registration is newer than that run's first lane", path)
	}
	return adoptReRegistration(run, &reRegistration{ManifestSHA256: prior.ManifestSHA256, Registration: *prior.Registration}, path)
}

// adoptReRegistration records the earlier registration on the run, and
// refuses a second, different one: a run reuses across one re-registration.
// A --resume checkpoint's record is restored through it too (CRT-V0-016).
func adoptReRegistration(run *report, carried *reRegistration, path string) error {
	if run.ReRegistered != nil {
		if run.ReRegistered.ManifestSHA256 != carried.ManifestSHA256 || run.ReRegistered.Registration.SHA256 != carried.Registration.SHA256 {
			return fmt.Errorf("%s: carries a different earlier registration than this run already reuses under", path)
		}
		return nil
	}
	run.ReRegistered = carried
	return nil
}

// resumeReRegistration restores a checkpoint's re_registered record, so lanes
// the interrupted run reused across the re-registration keep the registration
// they answer to (CRT-V0-016).
func resumeReRegistration(run, checkpoint *report, path string) error {
	if checkpoint.ReRegistered == nil {
		return nil
	}
	if run.Registration == nil {
		return fmt.Errorf("%s: the checkpoint reused lanes across a re-registration, so this run needs --registration", path)
	}
	if problems := reRegistrationProblems(run.Registration, &checkpoint.ReRegistered.Registration); len(problems) != 0 {
		return fmt.Errorf("%s: %s", path, strings.Join(problems, "; "))
	}
	carried := *checkpoint.ReRegistered
	return adoptReRegistration(run, &carried, path)
}

// reRegistrationProblems allows the two registrations to differ only in the
// scorer digest, and requires that they differ there.
func reRegistrationProblems(current, earlier *registrationRecord) []string {
	problems := registrationProblems(earlier.Document)
	same := earlier.Document
	same.ScorerSHA256 = current.Document.ScorerSHA256
	left, _ := json.Marshal(same)
	right, _ := json.Marshal(current.Document)
	if !bytes.Equal(left, right) {
		problems = append(problems, "the earlier registration differs from this run's beyond the scorer digest")
	}
	if earlier.Document.ScorerSHA256 == current.Document.ScorerSHA256 {
		problems = append(problems, "the earlier registration has the same scorer digest, so it is not a re-registration")
	}
	return problems
}

// reRegistrationInvalid is the rescore reading of CRT-V0-016: the carried
// earlier registration still differs only in the scorer digest, predates
// every lane reused from it, and only lanes from its manifest were reused.
func reRegistrationInvalid(document *report) []string {
	carried := document.ReRegistered
	if carried == nil {
		return nil
	}
	if document.Registration == nil {
		return []string{"a re-registered run carries no registration (CRT-V0-016)"}
	}
	problems := reRegistrationProblems(document.Registration, &carried.Registration)
	committed, err := time.Parse(time.RFC3339, carried.Registration.CommittedAt)
	if err != nil {
		return append(problems, "the earlier registration commit time is unreadable")
	}
	for _, item := range document.Lanes {
		if ownLane(document, item) {
			continue
		}
		if item.ReusedFrom != carried.ManifestSHA256 || (item.ReusedRegistration != "" && item.ReusedRegistration != carried.Registration.SHA256) {
			problems = append(problems, "lane "+laneKey(item)+" was reused from a report other than the re-registered one")
			continue
		}
		when, err := time.Parse(time.RFC3339Nano, item.StartedAt)
		if err == nil && committed.After(when) {
			problems = append(problems, "the earlier registration is newer than reused lane "+item.ID+"/"+item.Arm)
		}
	}
	return problems
}

// firstFreshLaneStart is the earliest start among lanes run under this
// registration, including a resumed checkpoint of the same manifest; lanes
// reused across the re-registration answer to the earlier one (CRT-V0-016).
func firstFreshLaneStart(document *report) (time.Time, bool) {
	fresh := &report{}
	for _, item := range document.Lanes {
		if ownLane(document, item) {
			fresh.Lanes = append(fresh.Lanes, item)
		}
	}
	return firstLaneStart(fresh)
}

// ownLane reports whether a lane ran under this run's registration: run
// fresh, or reused from a report of the same manifest under the same
// registration (a --resume checkpoint). A lane reused across a scorer-only
// re-registration keeps the earlier registration's digest in
// reused_registration, so it is not the run's own even when the manifest is
// unchanged (CRT-V0-016). A lane with no recorded registration falls back to
// the manifest comparison, as reports written before the field did.
func ownLane(document *report, item *lane) bool {
	if item.ReusedFrom == "" {
		return true
	}
	if item.ReusedFrom != document.ManifestSHA256 {
		return false
	}
	if item.ReusedRegistration == "" || document.Registration == nil {
		return true
	}
	return item.ReusedRegistration == document.Registration.SHA256
}
