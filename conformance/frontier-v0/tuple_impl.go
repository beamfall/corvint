// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// The CF-V0-003 dynamic tuple, built for real.
//
// A complete tuple is three caller-supplied artifacts: a canonical
// `test-command/0.1-experimental`, a canonical
// `test-observation/0.1-experimental` projecting one JUnit report under that
// command, and the report bytes themselves. Frontier accepts none of them as
// authority — CF-V0-002 makes it recompute TCQ over them — so the tuple has to
// be one a REAL `tcq.Evaluate` accepts and matches against the committed test
// blob, not three plausible blobs.
//
// The command and the observation are therefore produced by TCQ's own
// producers (`tcq.MakeTestCommand`, `tcq.MakeTestObservation`) rather than
// hand-written here: a hand-written artifact would prove only that this file
// agrees with itself, and both producers re-parse what they emit. The report is
// authored here, because the report IS the caller's raw evidence and has no
// producer; it names the exact execution identity TCQ derives from the
// committed `tests/claims_test.go` blob, which is what makes the row MATCH
// rather than merely exist.

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/tcq"
)

// testRow is one selected claim edge's execution identity: the committed blob
// its claim lives in and the Go table-case runtime name TCQ derives for it.
// TCQ-V0-028 keys a report row only when its `file` resolves to a tracked
// target-tree blob, and TCQ-V0-034 matches on the derived key alone.
type testRow struct {
	Path string
	Name string
}

// tupleRepository is the bounded target-tree seam TCQ reads report rows and cwd
// through. It is the generated repository itself, read through Git object
// identity exactly as internal/frontierrepo does — a fake tree would let a row
// key against a blob the verified target does not contain.
type tupleRepository struct{ git gitRunner }

func (r tupleRepository) Blob(oid string) ([]byte, error) {
	return r.git.raw("cat-file", "blob", oid)
}

func (r tupleRepository) TreeEntry(revision, path string) (tcq.TreeEntry, error) {
	out, err := r.git.run("ls-tree", "--full-tree", revision, "--", path)
	if err != nil {
		return tcq.TreeEntry{}, err
	}
	// `<mode> SP <type> SP <oid> TAB <path>`, or nothing when the path is
	// absent. TCQ draws that distinction itself: a missing object is an outcome
	// it reasons about, never an error.
	head, _, found := strings.Cut(out, "\t")
	if !found {
		return tcq.TreeEntry{}, nil
	}
	fields := strings.Fields(head)
	if len(fields) < 3 {
		return tcq.TreeEntry{}, nil
	}
	return tcq.TreeEntry{Mode: fields[0], Type: fields[1], Found: true}, nil
}

// tupleFor builds the declared CF-V0-003 tuple for one case.
//
// A PARTIAL combination is refused at cascade stage 1, before any Git work, so
// its bytes need only be present: their content is never read. A COMPLETE one
// is the opposite — every byte is parsed, digested, and matched — so it is
// built against this universe's real target revision and real claim blob.
func (u *builtUniverse) tupleFor(c Case) (command, observation, report []byte, err error) {
	switch c.Declared.DynamicTuple {
	case "absent":
		return nil, nil, nil, nil
	case "complete":
		return u.completeTuple(c)
	}
	command, observation, report = partialTuple(c.Declared)
	return command, observation, report, nil
}

// partialTuple builds the placeholder bytes a partial combination needs.
func partialTuple(declared Declared) (command, observation, report []byte) {
	body := func(name string) []byte {
		return []byte("{\"" + name + "\":\"" + canaryFor(declared, name) + "\"}\n")
	}
	for _, part := range strings.Split(declared.DynamicTuple, "+") {
		switch part {
		case "command", "command-only":
			command = body("command-body")
		case "observation", "observation-only":
			observation = body("observation-body")
		case "report", "report-only":
			report = body("report-body")
		}
	}
	return command, observation, report
}

func (u *builtUniverse) completeTuple(c Case) (command, observation, report []byte, err error) {
	// CF-V0-024 names argv as a body whose contents must never reach output,
	// and TCQ-V0-025 forbids persisting or echoing argv at all, so a declared
	// command-body canary belongs in argv and nowhere else.
	argv := []string{"go", "test", "./..."}
	if canary := canaryFor(c.Declared, "command-body"); canary != "none" {
		argv = append(argv, "-run="+canary)
	}
	command, err = tcq.MakeTestCommand(argv, ".", "corvint-frontier-conformance", "0",
		u.target, tcq.CleanTargetAttested)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("make test command: %w", err)
	}
	if c.Declared.TCQ == "invalid-junit" {
		report = malformedJUnit(c)
		observation, err = u.declaredObservation(command, report)
		if err != nil {
			return nil, nil, nil, err
		}
		return command, observation, report, nil
	}
	report = passingJUnit(c, u.testRows)
	observation, err = tcq.MakeTestObservation(tupleRepository{git: u.git}, command, report, u.target, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("make test observation: %w", err)
	}
	return command, observation, report, nil
}

// passingJUnit writes one PASSED row per selected claim edge, keyed by the
// execution identity TCQ derives from the committed claims blob: the row's
// `file` is that blob's path and its `name` is the Go table-case runtime name
// `TestClaimX/<obligation>`. Matching is equality of that derived key and
// nothing else (TCQ-V0-034), so a row named anything else is a row TCQ
// correctly refuses to match — which is why the names come from the universe
// the runner just built rather than from the fixture.
func passingJUnit(c Case, rows []testRow) []byte {
	var out strings.Builder
	out.WriteString(`<testsuite name="corvint-frontier-conformance">`)
	if planted := plantedReportText(c); planted != "" {
		out.WriteString(`<system-out>` + planted + `</system-out>`)
	}
	for _, row := range rows {
		out.WriteString(`<testcase file="` + row.Path + `" name="` + row.Name + `"></testcase>`)
	}
	out.WriteString(`</testsuite>` + "\n")
	return []byte(out.String())
}

// malformedJUnit is a report that violates the TCQ-V0-026 byte preflight: a
// DOCTYPE is refused as bytes, before any parser exists. It carries the case's
// canaries as XML attribute values, which is what the CF-V0-024 assertion on
// that row is about — a run that refuses a report must echo no value from it.
func malformedJUnit(c Case) []byte {
	return []byte(`<!DOCTYPE testsuite><testsuite name="` + plantedReportText(c) +
		`"><testcase file="tests/claims_test.go" name="x"></testcase></testsuite>` + "\n")
}

// plantedReportText collects every string this case forbids in output that the
// report is the natural body for: the declared CF-V0-024 `report-body` canary,
// plus any forbidden substring the fixture names without locating in a body of
// its own. A canary the suite never plants makes its own check vacuous, so an
// unlocated one goes into the one caller-controlled report body this harness
// owns rather than nowhere.
func plantedReportText(c Case) string {
	located := map[string]bool{}
	for _, canary := range c.Declared.Canaries {
		if canary.Where != "report-body" {
			located[canary.Value] = true
		}
	}
	planted := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		if value == "" || value == "none" || seen[value] {
			return
		}
		seen[value] = true
		planted = append(planted, value)
	}
	add(canaryFor(c.Declared, "report-body"))
	for _, forbidden := range c.ForbiddenSubstrings {
		if !located[forbidden] {
			add(forbidden)
		}
	}
	return strings.Join(planted, " ")
}

// declaredObservation is the caller's own projection of a report TCQ will
// refuse. `tcq.MakeTestObservation` cannot produce it — the producer parses the
// report it is projecting, so it fails on exactly the reports this case is
// about — but an observation is a caller-supplied artifact (TCQ-V0-044), and a
// real runner that emitted a malformed report would still record its length and
// digest. So the artifact is built here in the canonical shape TCQ-V0-031
// fixes, and TCQ re-derives its identity and refuses the report on its own: the
// case is reached through the public entry point, not through a seam.
func (u *builtUniverse) declaredObservation(command, report []byte) ([]byte, error) {
	commandID, err := memberString(command, "id")
	if err != nil {
		return nil, fmt.Errorf("read command id: %w", err)
	}
	body := fmt.Sprintf(
		`{"commandId":%q,"exitCode":0,"report":{"bytes":%d,"format":%q,"sha256":%q},`+
			`"rows":[],"spec":%q,"targetRevision":%q,"unkeyedRows":0}`,
		commandID, len(report), tcq.ReportFormat, digestHex(report), tcq.ObservationSpec, u.target)
	preimage, err := canonicalArtifact(body)
	if err != nil {
		return nil, err
	}
	identity := observationIDPrefix + DomainHash(observationIDDomain, preimage)
	whole, err := canonicalArtifact(strings.TrimSuffix(body, "}") + fmt.Sprintf(`,"id":%q}`, identity))
	if err != nil {
		return nil, err
	}
	return append(whole, '\n'), nil
}

// The observation identity derivation, transcribed from TCQ-V0-031: the prefix
// and the domain label are the artifact's own constants, and the digest is the
// same domain-separated SHA-256 shape CF-V0-006 and CF-V0-007 use, which this
// suite already implements independently in DomainHash.
const (
	observationIDPrefix = "test-observation:sha256:"
	observationIDDomain = "corvint-test-observation/0.1-experimental"
)

// canonicalArtifact reserializes one TCQ artifact through the CEM 0.2 wire
// codec, which is the codec those artifacts are canonical under. It is NOT this
// suite's refcodec: CF-V0-019 forbids JSON numbers, and an observation's
// `exitCode`, `bytes`, and `unkeyedRows` are numbers.
func canonicalArtifact(body string) ([]byte, error) {
	value, err := wire.Parse([]byte(body))
	if err != nil {
		return nil, fmt.Errorf("build observation: %w", err)
	}
	return wire.CanonicalValue(value), nil
}

func memberString(raw []byte, key string) (string, error) {
	value, err := wire.Parse(raw)
	if err != nil {
		return "", err
	}
	member, found := value.Obj.Get(key)
	if !found || member.Kind != wire.KindString {
		return "", fmt.Errorf("no string member %q", key)
	}
	return member.Str, nil
}

// raw runs a Git command and returns its stdout unaltered. gitRunner.run trims
// and merges stderr, which is right for a revision and wrong for blob bytes.
func (g gitRunner) raw(args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = g.root
	command.Env = gitEnvironment(g.home)
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w", args, err)
	}
	return out, nil
}
