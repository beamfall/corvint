package main

import (
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gateagreement"
	"github.com/Beamfall/corvint/internal/migrationratchet"
	"github.com/Beamfall/corvint/internal/testsource"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// gateAgreement holds the uniform --advisory, --compare FILE and --log FILE options of a
// gate-shaped command (GVA-V0-001).
type gateAgreement struct {
	advisory     bool
	compare, log string
	// label is the free-form --label copied to the record; baseline is set by a command that
	// compares against a baseline revision (GVA-V0-089).
	label, baseline string
	verdicts        *gateagreement.Verdicts
}

const gateAgreementUsage = "[--advisory] [--compare VERDICTS.json --log FILE.jsonl] [--log FILE.jsonl] [--label NAME]"

// splitGateAgreement removes the agreement options from args and returns the rest in order. Each
// option may be given once; --compare needs --log (GVA-V0-001).
func splitGateAgreement(args []string) (gateAgreement, []string, error) {
	g, rest := gateAgreement{}, []string{}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		switch name {
		case "--advisory":
			if inline || seen[name] {
				return g, nil, errors.New("--advisory takes no value and is given once")
			}
			seen[name], g.advisory = true, true
		case "--compare", "--log", "--label":
			if seen[name] {
				return g, nil, errors.New(name + " is given once")
			}
			if !inline {
				if i+1 >= len(args) {
					return g, nil, errors.New(name + " needs a file")
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return g, nil, errors.New(name + " needs a file")
			}
			seen[name] = true
			switch name {
			case "--compare":
				g.compare = value
			case "--label":
				g.label = value
			default:
				g.log = value
			}
		default:
			rest = append(rest, args[i])
		}
	}
	if g.compare != "" && g.log == "" {
		return g, nil, errors.New("--compare needs --log")
	}
	return g, rest, nil
}

// prepare reads the verdict file and refuses a log inside the analysed repository, which a read
// command never writes (GVA-V0-002, GVA-V0-005). root is "" for a command without a repository.
func (g *gateAgreement) prepare(root string) error {
	if g.log != "" && root != "" && pathInside(g.log, root) {
		return &gateagreement.Error{Code: "agreement-log-inside-repository", Message: "the --log file must be outside the analysed repository"}
	}
	if g.compare == "" {
		return nil
	}
	data, err := readBoundedFile(g.compare, gateagreement.MaxVerdictBytes)
	if err != nil {
		return &gateagreement.Error{Code: "agreement-verdicts-invalid", Message: "the verdict file is unreadable or larger than 4 MiB"}
	}
	verdicts, err := gateagreement.ParseVerdicts(g.compare, data)
	if err != nil {
		return err
	}
	g.verdicts = &verdicts
	return nil
}

// pathInside reports whether name, resolved through the symlinks of its existing parent, lies
// inside root.
func pathInside(name, root string) bool {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	absRoot, err := filepath.Abs(resolvedRoot)
	if err != nil {
		return true
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return true
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(parent, filepath.Base(abs))
	}
	relative, err := filepath.Rel(absRoot, abs)
	return err != nil || relative == "." || !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".."
}

// finish appends the run's record when --log is set and returns the status to exit with
// (GVA-V0-001, GVA-V0-004). A record that cannot be written refuses the run.
// rules, when set, are the rules the command decides.
func (g *gateAgreement) finish(command string, args []string, revision string, rules []string, observed gateagreement.Observed, code int) (int, error) {
	if g.log != "" {
		record := gateagreement.Compare(gateagreement.Run{Command: command, Arguments: args, Build: currentProducer(), Revision: revision, BaselineRevision: g.baseline, Label: g.label,
			ExitCode: code, Advisory: g.advisory, Rules: rules}, observed, g.verdicts)
		if err := gateagreement.Append(g.log, record); err != nil {
			return 2, err
		}
	}
	return gateagreement.AdvisoryExit(code, g.advisory), nil
}

// lintObserved maps a lint report to agreement keys: each finding, an UNKNOWN finding and an
// applied suppression as an abstention at its line, an uncertain file as a file abstention, and,
// in changed-lines scope, every file the run did not read (GVA-V0-003).
func lintObserved(report testsource.LintReport) gateagreement.Observed {
	observed := gateagreement.Observed{}
	for _, f := range report.Findings {
		key := gateagreement.Key{Rule: f.Rule, Path: f.Path, Line: f.Line}
		if f.Resolution != "" {
			observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: key, Reason: "finding-" + strings.ToLower(f.Resolution)})
			continue
		}
		observed.Findings = append(observed.Findings, key)
	}
	for _, f := range report.Suppressed {
		observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: gateagreement.Key{Rule: f.Rule, Path: f.Path, Line: f.Line}, Reason: "suppressed"})
	}
	for _, name := range report.Uncertain {
		observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: gateagreement.Key{Path: name}, Reason: "file-uncertain"})
	}
	if report.Scope == "changed-lines" {
		observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Reason: "outside-changed-lines"})
	}
	return observed
}

// lintRules are the rules a lint run decides: its closed rule set and any rule it reported.
func lintRules(report testsource.LintReport) []string {
	rules := append([]string{}, report.Rules...)
	for _, f := range report.Findings {
		rules = append(rules, f.Rule)
	}
	return rules
}

// duplicatesObserved maps a duplicates report to keys: each unresolved pair, including those
// beyond the listed bound only as a count, and an uncertain file as a file abstention. A
// truncated report abstains on every unlisted pair (GVA-V0-003).
func duplicatesObserved(report testsource.DuplicatesReport) gateagreement.Observed {
	observed := gateagreement.Observed{}
	for _, p := range report.Unresolved {
		observed.Findings = append(observed.Findings, gateagreement.Key{Rule: duplicatesAgreementRule, Test: p.Test, Other: p.Other})
	}
	for _, name := range report.Uncertain {
		observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: gateagreement.Key{Path: name}, Reason: "file-uncertain"})
	}
	if report.Truncated {
		observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: gateagreement.Key{Rule: duplicatesAgreementRule}, Reason: "output-truncated"})
	}
	return observed
}

// duplicatesAgreementRule is the rule an unresolved duplicate pair is compared under.
const duplicatesAgreementRule = "duplicate-pair"

// ratchetObserved maps a migration-ratchet receipt to keys: each policy failure CODE:ID as rule
// CODE on test ID, or rule CODE alone (GVA-V0-003).
func ratchetObserved(receipt migrationratchet.Receipt) gateagreement.Observed {
	observed := gateagreement.Observed{}
	// Identity deltas are keyed so a caller can compare its full regression set
	// (GVA-V0-088): removal:KIND on the baseline ID, state-regression:KIND on the candidate
	// ID, broken-reverse-link:KIND on the candidate (else baseline) ID. An excepted delta is an
	// abstention, not a finding.
	add := func(rule, id, excepted string) {
		key := gateagreement.Key{Rule: rule, Test: id}
		if excepted != "" {
			observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: key, Reason: "excepted"})
			return
		}
		observed.Findings = append(observed.Findings, key)
	}
	for _, d := range receipt.Removals {
		add("removal:"+d.Kind, d.BaselineID, d.ExceptedBy)
	}
	for _, d := range receipt.StateChanges {
		if d.Direction == "regression" {
			add("state-regression:"+d.Kind, d.CandidateID, d.ExceptedBy)
		}
	}
	for _, d := range receipt.BrokenReverseLinks {
		id := d.CandidateID
		if id == "" {
			id = d.BaselineID
		}
		add("broken-reverse-link:"+d.Kind, id, d.ExceptedBy)
	}
	for _, failure := range receipt.PolicyFailures {
		code, id, _ := strings.Cut(failure, ":")
		key := gateagreement.Key{Rule: code, Test: id}
		if id == "" {
			key.Path = "."
		}
		observed.Findings = append(observed.Findings, key)
	}
	return observed
}

// validityObserved maps a tests validity report to keys: each finding as its rule at the path and
// line of the test it names, an abstained finding (`finding-abstained`) and a cross-file reading
// withheld (`cross-file`) as abstentions there, an uncertain file as a file abstention
// (GVA-V0-094), and each seed- and assertion-parity finding the same way (GVA-V0-095).
func validityObserved(report testsource.SourceValidityReport) gateagreement.Observed {
	observed := gateagreement.Observed{}
	at := func(rule, test string) gateagreement.Key {
		key := gateagreement.Key{Rule: rule, Path: test}
		if i := strings.LastIndex(test, ":"); i > 0 {
			if line, err := strconv.Atoi(test[i+1:]); err == nil && line > 0 {
				key.Path, key.Line = test[:i], line
			}
		}
		return key
	}
	for _, file := range report.Files {
		for _, f := range file.Findings {
			if f.Severity == testvalidity.SeverityAbstained {
				observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: at(f.Rule, f.Test), Reason: "finding-abstained"})
				continue
			}
			observed.Findings = append(observed.Findings, at(f.Rule, f.Test))
		}
		for _, a := range file.Abstentions {
			observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: at(a.Rule, a.Test), Reason: "cross-file"})
		}
		if file.Uncertain {
			observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: gateagreement.Key{Path: file.Path}, Reason: "file-uncertain"})
		}
	}
	// Each --legacy link's parity findings are keyed like a file's (GVA-V0-095); a link that was
	// not analysed, or whose seed parity is uncertain, abstains on its family's other rules.
	parity := func(test string, findings []testvalidity.SourceFinding, unread bool, family ...string) {
		for _, f := range findings {
			observed.Findings = append(observed.Findings, at(f.Rule, f.Test))
		}
		if !unread {
			return
		}
		for _, rule := range family {
			observed.Abstentions = append(observed.Abstentions, gateagreement.Abstention{Key: at(rule, test), Reason: "parity-not-analysed"})
		}
	}
	for _, p := range report.SeedParity {
		parity(p.New, p.Findings, p.Status == testvalidity.SeedParityStatusNotAnalysed || p.Status == testvalidity.SeedParityStatusUncertain,
			testvalidity.RuleSeedParityDiverges, testvalidity.RuleSeedParityUncertain)
	}
	for _, p := range report.AssertionParity {
		parity(p.New, p.Findings, p.Status == testvalidity.AssertionParityStatusNotAnalysed, testvalidity.RuleAssertionParityUnmatchedLegacy,
			testvalidity.RuleAssertionParityNewWithoutOrigin, testvalidity.RuleAssertionParityWeakened)
	}
	return observed
}

// openAgreementLog opens one log for the summary.
func openAgreementLog(name string) (io.ReadCloser, error) {
	file, err := openBoundedRegularFile(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}
