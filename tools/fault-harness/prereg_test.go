// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const committedRule = "revert-rule.json"

func ruleSHA(t *testing.T) string {
	t.Helper()
	got, err := readHashFile(committedRule + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func basePrereg(t *testing.T) preregistration {
	t.Helper()
	var p preregistration
	p.Profile, p.Runner, p.Language = preregProfile, "go", "go"
	p.Repository.Name, p.Repository.SHA, p.Repository.PinDate = "example/go-repo", strings.Repeat("a", 40), "2026-11-01"
	p.SamplingFrame = "merged PRs of the 24 months before the pin that change Go source"
	p.RevertRule.Path, p.RevertRule.SHA256 = committedRule, ruleSHA(t)
	p.Operators = map[string][]string{"go": slices.Clone(operatorFamilies)}
	p.Seeds = map[string]int64{"mutants": 20261109}
	p.OverSample, p.ExclusionBudget, p.ExclusionRules = 110, 5, []string{"pin-unavailable"}
	return p
}

// writePrereg writes p and the hash file committed for it, and returns both
// paths.
func writePrereg(t *testing.T, p preregistration) (string, string) {
	t.Helper()
	dir := t.TempDir()
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path, hash := filepath.Join(dir, "prereg.json"), filepath.Join(dir, "prereg.json.sha256")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hash, []byte(sha(body)+"  prereg.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, hash
}

func writeJSON(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "file.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func codesOf(errs []error) []string {
	var codes []string
	for _, e := range errs {
		codes = append(codes, refusalCode(e))
	}
	return codes
}

func TestPreregistrationHash_FIH_V1_001(t *testing.T) {
	ctx := context.Background()
	path, hash := writePrereg(t, basePrereg(t))
	var w bytes.Buffer
	if code := run(ctx, []string{"check", "--preregistration", path, "--hash", hash}, &w); code != 0 {
		t.Fatalf("valid pre-registration: %d %s", code, w.String())
	}
	// Changed seeds: an edited seed changes the SHA-256, so the harness
	// refuses before it runs anything.
	body, _ := os.ReadFile(path)
	edited := bytes.Replace(body, []byte("20261109"), []byte("20261110"), 1)
	if bytes.Equal(edited, body) {
		t.Fatal("seed not found")
	}
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range [][]string{{"check"}, {"seal", "--rule", committedRule, "--candidates", "none.json", "--set", "none.json"}} {
		w.Reset()
		args := append(cmd, "--preregistration", path, "--hash", hash)
		if code := run(ctx, args, &w); code != 1 || !strings.Contains(w.String(), "refused: preregistration-changed") {
			t.Fatalf("%s with an edited seed: %d %s", cmd[0], code, w.String())
		}
	}
	if _, err := loadPreregistration(path, ""); refusalCode(err) != "preregistration-unhashed" {
		t.Fatalf("unhashed: %v", err)
	}
	// The pre-registration holds the rule file's hash; another rule file is
	// refused.
	p := basePrereg(t)
	other := writeJSON(t, map[string]any{"profile": ruleProfile})
	if _, _, err := loadRule(other, p.RevertRule.SHA256); refusalCode(err) != "revert-rule-changed" {
		t.Fatalf("changed rule: %v", err)
	}
	for name, mutate := range map[string]func(*preregistration){
		"unknown operator":    func(p *preregistration) { p.Operators["go"] = []string{"swap-everything"} },
		"no operators":        func(p *preregistration) { p.Operators = map[string][]string{"python": {"relational-swap"}} },
		"over-sample below N": func(p *preregistration) { p.OverSample = 99 },
		"no seed":             func(p *preregistration) { p.Seeds = map[string]int64{} },
		"short SHA":           func(p *preregistration) { p.Repository.SHA = "abc" },
		"no rule hash":        func(p *preregistration) { p.RevertRule.SHA256 = "" },
		"playwright without flow": func(p *preregistration) {
			p.Runner, p.Language = "playwright", "typescript"
			p.Operators = map[string][]string{"typescript": {"relational-swap"}}
		},
	} {
		p := basePrereg(t)
		mutate(&p)
		if _, err := loadPreregistration(writePrereg(t, p)); refusalCode(err) != "preregistration-invalid" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Held-out pre-registration before the draw: refused.
	held := basePrereg(t)
	held.HeldOut = true
	if _, err := loadPreregistration(writePrereg(t, held)); refusalCode(err) != "held-out-not-drawn" {
		t.Fatalf("held-out before the draw: %v", err)
	}
	draw := writeJSON(t, map[string]string{"draw": "fixture"})
	drawBody, _ := os.ReadFile(draw)
	held.DrawRecord = &struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}{draw, strings.Repeat("0", 64)}
	if _, err := loadPreregistration(writePrereg(t, held)); refusalCode(err) != "held-out-not-drawn" {
		t.Fatalf("held-out with a different draw record: %v", err)
	}
	held.DrawRecord.SHA256 = sha(drawBody)
	if _, err := loadPreregistration(writePrereg(t, held)); err != nil {
		t.Fatalf("held-out after the draw: %v", err)
	}
}

// candidate returns a qualifying fix PR merged daysBefore the 2026-11-01 pin.
func candidate(n, daysBefore int) candidatePR {
	return candidatePR{
		Number: n, MergedAt: fmt.Sprintf("2026-%02d-%02d", 10-daysBefore/30%10, 1+daysBefore%28), Title: fmt.Sprintf("fix: case %d", n),
		SourceFiles: []string{fmt.Sprintf("src/f%d.go", n)}, TestFiles: []string{fmt.Sprintf("src/f%d_test.go", n)},
		RevertApplies: true, RevertBuilds: true, RevertFailRuns: 3,
	}
}

func mergedMonthsBefore(c candidatePR, months int) candidatePR {
	c.MergedAt = fmt.Sprintf("%04d-%02d-15", 2026-(months+1)/12, 1+(10-months%12+12)%12)
	return c
}

func TestRevertRule_FIH_V1_002(t *testing.T) {
	r, sum, err := loadRule(committedRule, ruleSHA(t))
	if err != nil {
		t.Fatalf("committed rule: %v", err)
	}
	// Same rule, same thirty: two selections over one frame, in any input
	// order, name the same 30 PRs in the same order.
	var frame []candidatePR
	for n := 1; n <= 50; n++ {
		frame = append(frame, candidate(n, n%200))
	}
	a, err := selectReverts(r, sum, "2026-11-01", frame)
	if err != nil {
		t.Fatal(err)
	}
	shuffled := slices.Clone(frame)
	rand.New(rand.NewSource(1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	b, _ := selectReverts(r, sum, "2026-11-01", shuffled)
	if len(a.Reverts) != 30 || a.Shortfall != 0 || a.FrameMonths != 12 || !reflect.DeepEqual(a, b) {
		t.Fatalf("selections differ or are short: %+v %+v", a, b)
	}
	// Another rule hash orders the frame differently.
	if c, _ := selectReverts(r, strings.Repeat("0", 64), "2026-11-01", frame); slices.Equal(c.Reverts, a.Reverts) {
		t.Fatal("rule hash does not seed the order")
	}
	// At most two per source file.
	shared := slices.Clone(frame)
	for i := range shared {
		shared[i].SourceFiles = []string{"src/hot.go"}
	}
	if s, _ := selectReverts(r, sum, "2026-11-01", shared); len(s.Reverts) != 2 || s.Shortfall != 28 {
		t.Fatalf("per-file cap: %+v", s)
	}
	// What qualifies.
	pin := "2026-11-01"
	for name, c := range map[string]candidatePR{
		"two of three":  func() candidatePR { c := candidate(1, 3); c.RevertFailRuns = 2; return c }(),
		"no test":       func() candidatePR { c := candidate(1, 3); c.TestFiles = nil; return c }(),
		"no build":      func() candidatePR { c := candidate(1, 3); c.RevertBuilds = false; return c }(),
		"not a fix":     func() candidatePR { c := candidate(1, 3); c.Title = "fixture: rename"; return c }(),
		"after the pin": func() candidatePR { c := candidate(1, 3); c.MergedAt = "2026-11-02"; return c }(),
		"25 months":     mergedMonthsBefore(candidate(1, 3), 25),
	} {
		if s, _ := selectReverts(r, sum, pin, []candidatePR{c}); len(s.Reverts) != 0 {
			t.Fatalf("%s qualified: %+v", name, c)
		}
	}
	for name, c := range map[string]candidatePR{
		"bug label":    func() candidatePR { c := candidate(1, 3); c.Title, c.Labels = "Tidy parser", []string{"Bug"}; return c }(),
		"closing link": func() candidatePR { c := candidate(1, 3); c.Title, c.ClosesBug = "Tidy parser", true; return c }(),
		"fix(scope)":   func() candidatePR { c := candidate(1, 3); c.Title = "Fix(parser): off by one"; return c }(),
	} {
		if s, _ := selectReverts(r, sum, pin, []candidatePR{c}); len(s.Reverts) != 1 {
			t.Fatalf("%s did not qualify: %+v", name, c)
		}
	}
	// Fewer than thirty: 20 in 12 months extend the frame to 24 months, where
	// 24 qualify; the shortfall of 6 is reported.
	var short []candidatePR
	for n := 1; n <= 20; n++ {
		short = append(short, candidate(n, n))
	}
	for n := 21; n <= 24; n++ {
		short = append(short, mergedMonthsBefore(candidate(n, 1), 18))
	}
	short = append(short, mergedMonthsBefore(candidate(99, 1), 30))
	s, _ := selectReverts(r, sum, pin, short)
	if s.FrameMonths != 24 || s.Qualifying != 24 || len(s.Reverts) != 24 || s.Shortfall != 6 || slices.Contains(s.Reverts, 99) {
		t.Fatalf("shortfall selection: %+v", s)
	}
}

// sealFixture builds a set that seals: the rule's reverts, the shortfall
// fill, mutant PRs up to the over-sample and 30 non-code faults.
func sealFixture(t *testing.T, p loadedPreregistration, sel revertSelection, codePRs, nonCode int) faultSet {
	t.Helper()
	sha := func(i int) string { return fmt.Sprintf("%040x", i) }
	set := faultSet{Profile: faultSetProfile, PreregistrationSHA256: p.SHA256, Runner: p.Runner}
	for _, n := range sel.Reverts {
		set.Cases = append(set.Cases, harnessCase{ID: fmt.Sprintf("revert-%d", n), Kind: "revert", PR: n, BaseSHA: sha(1), HeadSHA: sha(1),
			Faults: []fault{{ID: fmt.Sprintf("revert-%d-f", n), Operator: "revert", Revert: &revertFault{FixBase: sha(n), FixHead: sha(n + 1), Paths: []string{"src/a.go"}}}}})
	}
	for i := 0; len(set.Cases) < codePRs; i++ {
		c := harnessCase{ID: fmt.Sprintf("pr%d", 1000+i), Kind: "mutants", PR: 1000 + i, BaseSHA: sha(2 * i), HeadSHA: sha(2*i + 1), FillsShortfall: i < sel.Shortfall}
		for m := 0; m < 3; m++ {
			c.Faults = append(c.Faults, fault{ID: fmt.Sprintf("%s-m%d", c.ID, m), Path: "src/a.go", Line: 10 + m, Offset: 100 + m, Before: "<", After: ">=", Operator: "relational-swap"})
		}
		set.Cases = append(set.Cases, c)
	}
	for i := 0; i < nonCode; i++ {
		set.Cases = append(set.Cases, harnessCase{ID: fmt.Sprintf("nc%d", i), Kind: "non-code", PR: 5000 + i, BaseSHA: sha(3), HeadSHA: sha(4),
			Faults: []fault{{ID: fmt.Sprintf("nc%d-f", i), Path: "config/app.yaml", Line: 1, Before: "a", After: "b", Operator: "non-code", Category: "configuration"}}})
	}
	return set
}

func TestSeal_FIH_V1_002_014(t *testing.T) {
	ctx := context.Background()
	prereg := basePrereg(t)
	path, hash := writePrereg(t, prereg)
	p, err := loadPreregistration(path, hash)
	if err != nil {
		t.Fatal(err)
	}
	var short []candidatePR
	for n := 1; n <= 20; n++ {
		short = append(short, candidate(n, n))
	}
	for n := 21; n <= 24; n++ {
		short = append(short, mergedMonthsBefore(candidate(n, 1), 18))
	}
	cands := writeJSON(t, candidates{Profile: candidatesProfile, PRs: short})
	sel, err := selectFromFiles(p, committedRule, cands)
	if err != nil {
		t.Fatal(err)
	}
	// Fewer than thirty qualifying fixes: 24 reverts, a shortfall of 6
	// filled with changed-line mutants and reported.
	set := sealFixture(t, p, sel, 110, 30)
	setPath := writeJSON(t, set)
	var w bytes.Buffer
	sealOut := filepath.Join(t.TempDir(), "seal.json")
	if code := run(ctx, []string{"seal", "--preregistration", path, "--hash", hash, "--rule", committedRule, "--candidates", cands, "--set", setPath, "--out", sealOut}, &w); code != 0 {
		t.Fatalf("seal: %d %s", code, w.String())
	}
	if !strings.Contains(w.String(), "24 reverts, shortfall 6 filled with changed-line mutants") {
		t.Fatalf("shortfall not reported: %s", w.String())
	}
	var rec sealRecord
	raw, _ := os.ReadFile(sealOut)
	if err := json.Unmarshal(raw, &rec); err != nil || rec.RevertCases != 24 || rec.Reverts.Shortfall != 6 || rec.CodeCases != 110 || rec.NonCodeCases != 30 {
		t.Fatalf("seal record %+v %v", rec, err)
	}
	setBody, _ := os.ReadFile(setPath)
	if err := checkSealed(p, set, sha(setBody), rec); err != nil {
		t.Fatalf("sealed set refused: %v", err)
	}
	if err := checkSealed(p, set, sha(append(setBody, ' ')), rec); refusalCode(err) != "fault-set-changed" {
		t.Fatalf("changed set: %v", err)
	}
	refusals := func(s faultSet, p loadedPreregistration) []string {
		_, errs := seal(p, sel, s, "x")
		return codesOf(errs)
	}
	// Too few non-code faults: 110 code PRs and 12 non-code faults.
	if got := refusals(sealFixture(t, p, sel, 110, 12), p); !slices.Equal(got, []string{"non-code-faults-below-minimum"}) {
		t.Fatalf("too few non-code: %v", got)
	}
	if got := refusals(sealFixture(t, p, sel, 109, 30), p); !slices.Equal(got, []string{"over-sample-not-met"}) {
		t.Fatalf("over-sample: %v", got)
	}
	bad := sealFixture(t, p, sel, 110, 30)
	bad.Cases[30].Faults = bad.Cases[30].Faults[:2]
	bad.Cases[31].Faults[0].Operator = "constant-replacement-everywhere"
	bad.Cases[24].FillsShortfall = false
	bad.Cases[0], bad.Cases[1] = bad.Cases[1], bad.Cases[0]
	bad.Cases[len(bad.Cases)-1].Faults[0].Line = 0
	bad.PreregistrationSHA256 = strings.Repeat("0", 64)
	if got := refusals(bad, p); !slices.Equal(got, []string{"preregistration-changed", "mutants-per-pr", "operator-not-registered", "fault-set-invalid", "reverts-not-rule", "shortfall-not-filled"}) {
		t.Fatalf("bad set: %v", got)
	}
	// Too few flow-reaching faults in a Playwright set: 12 against 20.
	pw := p
	pw.Runner = "playwright"
	pwSet := sealFixture(t, pw, sel, 110, 30)
	pwSet.Runner = "playwright"
	for i := 0; i < 12; i++ {
		pwSet.Cases[40+i].Faults[0].FlowReaching = true
	}
	_, errs := seal(pw, sel, pwSet, "x")
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "flow-reaching-faults-below-minimum: 12 against the minimum of 20") {
		t.Fatalf("flow-reaching: %v", errs)
	}
	for i := 0; i < 20; i++ {
		pwSet.Cases[40+i].Faults[1].FlowReaching = true
	}
	if _, errs := seal(pw, sel, pwSet, "x"); len(errs) != 0 {
		t.Fatalf("flow-reaching at the minimum: %v", errs)
	}
}
