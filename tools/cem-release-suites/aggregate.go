package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

// runRecordProfile names one platform's release-run output (CEM-V1-611). A
// record is unauthenticated run output: it carries what one cem-release-gate
// run observed so another host's run of the same revision can be judged with
// it, and it never proves who ran it or how the binaries were built.
const runRecordProfile = "cem-release-run/1"

const runRecordLimit = 16 << 20

var (
	recordRevision = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	recordDigest   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type runRecord struct {
	Profile     string            `json:"profile"`
	Revision    string            `json:"revision"`
	Platform    string            `json:"platform"`
	Declaration recordFile        `json:"declaration"`
	Binaries    []recordBinary    `json:"binaries"`
	Runner      map[string]string `json:"runner"`
	Platforms   map[string]string `json:"platforms"`
	Accepted    []string          `json:"accepted"`
}

type recordFile struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
}

type recordBinary struct {
	Name   string `json:"name"`
	Main   string `json:"main"`
	Sha256 string `json:"sha256"`
}

// recordBinaries is the exact binary set every record must name, in order.
var recordBinaries = [2][2]string{{"corvint", corvintMain}, {"cem01-go", cem01Main}}

// readRunRecord reads raw as one cem-release-run/1 record: closed members,
// one JSON value, and the canonical encoding writeRunRecord produces.
func readRunRecord(raw []byte) (runRecord, error) {
	var rec runRecord
	if len(raw) == 0 || len(raw) > runRecordLimit {
		return rec, errors.New("record is empty or exceeds 16 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rec); err != nil {
		return rec, err
	}
	if decoder.More() {
		return rec, errors.New("trailing JSON value")
	}
	canonical, err := encodeRunRecord(rec)
	if err != nil || !bytes.Equal(canonical, raw) {
		return rec, fmt.Errorf("record is not the canonical %s encoding", runRecordProfile)
	}
	return rec, nil
}

func encodeRunRecord(rec runRecord) ([]byte, error) {
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// checkRecords refuses unless every record is one passing run of the same
// revision and declaration, names exactly the corvint and cem01-go binaries
// with digests, observes exactly its own platform PASSED, and declares the
// same runner rows; no platform and no binary digest may appear in two
// records (CEM-V1-612). It returns the merged observation: a runner row is
// PASSED only when every record observed it PASSED, and otherwise the first
// other outcome in record order.
func checkRecords(records []runRecord, declarationPath, declarationSha string) (map[string]string, map[string]string, [][]byte, error) {
	if len(records) == 0 {
		return nil, nil, nil, errors.New("no run record")
	}
	runner, platforms := map[string]string{}, map[string]string{}
	var accepted [][]byte
	binaries := map[string]int{}
	first := records[0]
	for i, rec := range records {
		n := i + 1
		switch {
		case rec.Profile != runRecordProfile:
			return nil, nil, nil, fmt.Errorf("record %d profile %q is not %s", n, rec.Profile, runRecordProfile)
		case !recordRevision.MatchString(rec.Revision) || rec.Revision != first.Revision:
			return nil, nil, nil, fmt.Errorf("record %d revision %s is not record 1's revision %s", n, rec.Revision, first.Revision)
		case rec.Declaration != (recordFile{Path: declarationPath, Sha256: declarationSha}):
			return nil, nil, nil, fmt.Errorf("record %d judged declaration %s sha256 %s, not %s sha256 %s at %s", n, rec.Declaration.Path, rec.Declaration.Sha256, declarationPath, declarationSha, first.Revision)
		case rec.Platform == "" || len(rec.Platforms) != 1 || rec.Platforms[rec.Platform] != "PASSED":
			return nil, nil, nil, fmt.Errorf("record %d does not observe exactly its platform %q PASSED", n, rec.Platform)
		case platforms[rec.Platform] != "":
			return nil, nil, nil, fmt.Errorf("record %d repeats platform %s", n, rec.Platform)
		case len(rec.Accepted) == 0:
			return nil, nil, nil, fmt.Errorf("record %d carries no accepted stable result", n)
		case !sameKeys(rec.Runner, first.Runner):
			return nil, nil, nil, fmt.Errorf("record %d runner rows %v are not record 1's %v", n, keys(rec.Runner), keys(first.Runner))
		}
		if len(rec.Binaries) != len(recordBinaries) {
			return nil, nil, nil, fmt.Errorf("record %d names %d binaries, not corvint and cem01-go", n, len(rec.Binaries))
		}
		for j, b := range rec.Binaries {
			if b.Name != recordBinaries[j][0] || b.Main != recordBinaries[j][1] || !recordDigest.MatchString(b.Sha256) {
				return nil, nil, nil, fmt.Errorf("record %d binary %d is %s %s, not %s %s with a sha256", n, j+1, b.Name, b.Main, recordBinaries[j][0], recordBinaries[j][1])
			}
			if prior, reused := binaries[b.Sha256]; reused {
				return nil, nil, nil, fmt.Errorf("record %d reuses binary sha256 %s from record %d", n, b.Sha256, prior)
			}
			binaries[b.Sha256] = n
		}
		platforms[rec.Platform] = "PASSED"
		for row, outcome := range rec.Runner {
			if prior, seen := runner[row]; !seen || prior == "PASSED" {
				runner[row] = outcome
			}
		}
		for _, line := range rec.Accepted {
			accepted = append(accepted, []byte(line))
		}
	}
	return runner, platforms, accepted, nil
}

func sameKeys(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// aggregateRuns judges release readiness over per-platform run records of one
// revision (CEM-V1-612): the declaration is read from that revision's
// committed objects at root, never from a record, and each record must have
// judged exactly those bytes. Exit 1 refuses the record set, 3 refuses
// readiness, 0 is READY. A non-empty ciUnobservable selects the CI scope of
// AFP-V0-432 (see ciScopeExit); it never changes the judgment or the outcome.
func aggregateRuns(root, readiness string, paths []string, outcome, ciUnobservable string, stdout, stderr io.Writer) int {
	if err := absentOutput(outcome); err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	records := make([]runRecord, 0, len(paths))
	digests := make([]string, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err == nil {
			var rec runRecord
			if rec, err = readRunRecord(raw); err == nil {
				records = append(records, rec)
				digests = append(digests, digest(raw))
			}
		}
		if err != nil {
			fmt.Fprintf(stderr, "cem-release-suites: refused: record %s: %v\n", path, err)
			return 1
		}
	}
	revision := records[0].Revision
	if !recordRevision.MatchString(revision) {
		fmt.Fprintf(stderr, "cem-release-suites: refused: record 1 revision %q is not a full object name\n", revision)
		return 1
	}
	declarationRaw, err := gitCommand(root, "cat-file", "blob", "--end-of-options", revision+":"+readiness).Output()
	var declaration companionrelease.StableReleaseDeclaration
	if err == nil {
		declaration, err = companionrelease.ReadStableReleaseDeclaration(declarationRaw)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cem-release-suites: refused: readiness declaration %s at %s: %v\n", readiness, revision, err)
		return 1
	}
	attested, err := ciUnobservableRows(ciUnobservable, declaration)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: refused:", err)
		return 1
	}
	runner, platforms, accepted, err := checkRecords(records, readiness, digest(declarationRaw))
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: refused:", err)
		return 1
	}
	fmt.Fprintf(stdout, "cem-release-suites: aggregate: %d run records of revision %s (unauthenticated run output)\n", len(records), revision)
	external := map[string]string{}
	code, reason := judgeReadiness(declaration, accepted, runner, platforms, external, stdout)
	code = writeOutcome(outcome, revision, digest(declarationRaw), digests, code, reason, stdout, stderr)
	if attested == nil {
		return code
	}
	return ciScopeExit(code, reason, attested, external, stdout)
}

// ciUnobservableRows reads the --ci-unobservable value: nil when it is empty
// (strict), and otherwise the named rows, which must be distinct and exactly
// the declaration's external rows, so the caller can never exclude a runner
// or platform row nor silently drop a newly declared external one.
func ciUnobservableRows(value string, d companionrelease.StableReleaseDeclaration) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	rows := strings.Split(value, ",")
	seen := map[string]bool{}
	for _, row := range rows {
		if row == "" || seen[row] {
			return nil, fmt.Errorf("--ci-unobservable %q names an empty or repeated row", value)
		}
		seen[row] = true
	}
	want := append([]string(nil), d.External...)
	got := append([]string(nil), rows...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		return nil, fmt.Errorf("--ci-unobservable %q is not exactly the declaration's external rows %q", value, strings.Join(want, ","))
	}
	return rows, nil
}

// ciScopeExit is the PR and main-push exit of AFP-V0-432 (V1-2736). The strict
// verdict was already printed and written; this returns 0 only when it refused
// with the external class alone and every attested row is unobserved.
// StableReleaseReadiness checks the accepted results, then the runner rows,
// then the platform rows, and the external rows last, so an external-class
// refusal means every earlier row is PASSED. Any other refusal, or any other
// nonzero exit, is returned unchanged. Each attested row is printed as a known
// blocker and stays NOT_OBSERVED; nothing here marks a row PASSED.
func ciScopeExit(code int, reason string, attested []string, external map[string]string, stdout io.Writer) int {
	if code != 3 || !strings.HasPrefix(reason, companionrelease.StableRefusalExternal+":") {
		return code
	}
	for _, row := range attested {
		if _, observed := external[row]; observed {
			return code
		}
	}
	for _, row := range attested {
		fmt.Fprintf(stdout, "cem-release-suites: readiness: known blocker: external %s NOT_OBSERVED (caller-attested not CI-observable)\n", row)
	}
	fmt.Fprintf(stdout, "cem-release-suites: readiness: CI-OBSERVABLE PASSED (every runner and platform row PASSED); strict readiness stays REFUSED on %d known blocker(s)\n", len(attested))
	return 0
}

// writeOutcome writes the cem-release-gate-outcome/1 verdict (CEM-V1-613) to
// path when one is named and returns code, or 2 when the write fails. A run
// that refuses before judging readiness writes none, so a missing outcome
// never reads as a verdict.
func writeOutcome(path, revision, declarationSha string, records []string, code int, reason string, stdout, stderr io.Writer) int {
	if path == "" {
		return code
	}
	verdict := companionrelease.StableGateReady
	if code != 0 {
		verdict = companionrelease.StableGateRefused
	}
	raw, err := companionrelease.EncodeStableGateOutcome(companionrelease.StableGateOutcome{Profile: companionrelease.StableGateOutcomeProfile, Revision: revision, DeclarationSha256: declarationSha, Records: records, Verdict: verdict, Reason: reason})
	if err == nil {
		err = writeNew(path, raw)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: outcome:", err)
		return 2
	}
	fmt.Fprintf(stdout, "cem-release-suites: outcome: %s %s sha256 %s\n", path, verdict, digest(raw))
	return code
}

// absentOutput refuses an output path that already exists, so a stale record
// or verdict from an earlier run is never taken for this run's.
func absentOutput(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("output %s already exists or cannot be checked", path)
	}
	return nil
}

func writeNew(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
