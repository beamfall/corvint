// Package gateagreement compares a gate-shaped command's findings with a caller's verdict file and
// keeps an append-only JSONL agreement log with a per-rule summary (docs/specs/
// gate-verdict-agreement-v0.md, GVA-V0-001..012). It never changes a finding or a verdict; it only counts
// where the two agree.
package gateagreement

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jsonend"
)

const (
	// VerdictSchema names the caller's verdict file (GVA-V0-002).
	VerdictSchema = "corvint-gate-verdicts/0"
	// RecordSchema names one agreement log line (GVA-V0-004).
	RecordSchema = "corvint-gate-agreement/0"
	// SummarySchema names the summary report (GVA-V0-007).
	SummarySchema = "corvint-gate-agreement-summary/0"
	// MaxVerdictBytes and MaxVerdicts bound one verdict file.
	MaxVerdictBytes = 4 << 20
	MaxVerdicts     = 100000
	// MaxRecordBytes bounds one log line; MaxLogBytes one log file the summary reads.
	MaxRecordBytes = 8 << 20
	MaxLogBytes    = 256 << 20
	// MaxLogs bounds the logs one summary reads.
	MaxLogs = 64
)

// Comparison statuses (GVA-V0-004).
const (
	StatusCompared    = "COMPARED"
	StatusNotProduced = "NOT_PRODUCED"
)

// Error is a typed refusal.
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func refuse(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Key identifies one finding or verdict: a rule and where it applies. Line 0 in a verdict matches
// any line of the path; Other names the second test of an unordered pair (GVA-V0-002).
type Key struct {
	Rule  string `json:"rule"`
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Test  string `json:"test,omitempty"`
	Other string `json:"other,omitempty"`
}

// normal orders a pair so that the comparison is unordered.
func (k Key) normal() Key {
	if k.Other != "" && k.Other < k.Test {
		k.Test, k.Other = k.Other, k.Test
	}
	return k
}

// path is the key's file: Path, else the path part of a `path :: title` or `path:line` test.
func (k Key) path() string {
	if k.Path != "" {
		return k.Path
	}
	if p, _, ok := strings.Cut(k.Test, " :: "); ok {
		return p
	}
	return ""
}

// Abstention is a scope where the command did not decide: an uncertain file, or a finding it
// reported as unknown. A Rule, Line or Test left empty covers every value (GVA-V0-003).
type Abstention struct {
	Key
	Reason string `json:"reason"`
}

// Observed is what one command run decided.
type Observed struct {
	Findings    []Key
	Abstentions []Abstention
}

// Verdicts is a parsed caller verdict file with its digest.
type Verdicts struct {
	Name     string
	SHA256   string
	Verdicts []Key
}

// ParseVerdicts strictly decodes a caller verdict file (GVA-V0-002).
func ParseVerdicts(name string, data []byte) (Verdicts, error) {
	if len(data) > MaxVerdictBytes {
		return Verdicts{}, refuse("agreement-verdicts-invalid", "%s is larger than %d bytes", name, MaxVerdictBytes)
	}
	var file struct {
		Schema   string `json:"schema"`
		Verdicts []Key  `json:"verdicts"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil || !jsonend.Done(decoder) {
		return Verdicts{}, refuse("agreement-verdicts-invalid", "%s is not a valid verdict file", name)
	}
	if file.Schema != VerdictSchema {
		return Verdicts{}, refuse("agreement-verdicts-invalid", "%s must declare schema %s", name, VerdictSchema)
	}
	if len(file.Verdicts) > MaxVerdicts {
		return Verdicts{}, refuse("agreement-verdicts-invalid", "%s holds more than %d verdicts", name, MaxVerdicts)
	}
	for i, v := range file.Verdicts {
		if strings.TrimSpace(v.Rule) == "" || v.Line < 0 || v.Path == "" && v.Test == "" {
			return Verdicts{}, refuse("agreement-verdicts-invalid", "%s verdict %d needs a rule and a path or test", name, i+1)
		}
	}
	sum := sha256.Sum256(data)
	return Verdicts{Name: name, SHA256: "sha256:" + hex.EncodeToString(sum[:]), Verdicts: file.Verdicts}, nil
}

// RuleCounts are one rule's counts in one run or over a log.
type RuleCounts struct {
	Agreements  int `json:"agreements"`
	CorvintOnly int `json:"corvintOnly"`
	CallerOnly  int `json:"callerOnly"`
	Abstentions int `json:"abstentions"`
}

// VerdictSource names the verdict file a record compared against.
type VerdictSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Record is one agreement log line (GVA-V0-004).
type Record struct {
	Schema    string   `json:"schema"`
	At        string   `json:"at"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	Build     any      `json:"build"`
	Revision  string   `json:"revision,omitempty"`
	// BaselineRevision is the compared baseline of a command that has one (GVA-V0-089); Label is
	// the caller's free-form --label that splits logs of several callers.
	BaselineRevision string `json:"baselineRevision,omitempty"`
	Label            string `json:"label,omitempty"`
	// ExitCode is the command's own exit status; AdvisoryExitCode the status it returned.
	ExitCode         int            `json:"exitCode"`
	Advisory         bool           `json:"advisory"`
	AdvisoryExitCode int            `json:"advisoryExitCode"`
	Comparison       string         `json:"comparison"`
	Reason           string         `json:"reason,omitempty"`
	Verdicts         *VerdictSource `json:"verdicts,omitempty"`
	Findings         int            `json:"findings"`
	Agreements       []Key          `json:"agreements"`
	CorvintOnly      []Key          `json:"corvintOnly"`
	CallerOnly       []Key          `json:"callerOnly"`
	Abstentions      []Abstention   `json:"abstentions"`
	// IgnoredVerdicts are caller verdicts for a rule the command does not decide.
	IgnoredVerdicts []Key `json:"ignoredVerdicts"`
	// Rules counts each rule's outcomes in this run.
	Rules map[string]RuleCounts `json:"rules"`
}

// Now is the record clock; a test may replace it.
var Now = func() time.Time { return time.Now().UTC() }

// Run identifies one command invocation for its record.
type Run struct {
	Command   string
	Arguments []string
	Build     any
	Revision  string
	// BaselineRevision and Label are copied to the record unchanged.
	BaselineRevision string
	Label            string
	ExitCode         int
	Advisory         bool
	// Rules, when set, are the rules the command decides; a verdict for another rule is listed
	// under ignoredVerdicts and counted nowhere (GVA-V0-003).
	Rules []string
}

// AdvisoryExit is the status an advisory run returns: 0 unless the command failed (exit 2 or
// any status other than 0 and 1), which is returned unchanged (GVA-V0-001).
func AdvisoryExit(code int, advisory bool) int {
	if advisory && code == 1 {
		return 0
	}
	return code
}

// Compare builds the record of one run (GVA-V0-003, GVA-V0-004). Without verdicts the comparison
// is NOT_PRODUCED and only the findings are counted as Corvint-only for the record, never in a
// summary rate.
func Compare(run Run, observed Observed, verdicts *Verdicts) Record {
	record := Record{Schema: RecordSchema, At: Now().Format(time.RFC3339), Command: run.Command, Arguments: append([]string{}, run.Arguments...),
		Build: run.Build, Revision: run.Revision, BaselineRevision: run.BaselineRevision, Label: run.Label, ExitCode: run.ExitCode, Advisory: run.Advisory,
		AdvisoryExitCode: AdvisoryExit(run.ExitCode, run.Advisory), Findings: len(observed.Findings),
		Agreements: []Key{}, CorvintOnly: []Key{}, CallerOnly: []Key{}, Abstentions: []Abstention{}, IgnoredVerdicts: []Key{},
		Rules: map[string]RuleCounts{}}
	findings := make([]Key, len(observed.Findings))
	for i, f := range observed.Findings {
		findings[i] = f.normal()
	}
	sortKeys(findings)
	if verdicts == nil {
		record.Comparison, record.Reason = StatusNotProduced, "no-verdict-file"
		record.CorvintOnly = findings
		return record
	}
	record.Comparison = StatusCompared
	record.Verdicts = &VerdictSource{Path: verdicts.Name, SHA256: verdicts.SHA256}
	decided := map[string]bool{}
	for _, rule := range run.Rules {
		decided[rule] = true
	}
	callers := make([]Key, 0, len(verdicts.Verdicts))
	for _, v := range verdicts.Verdicts {
		if len(decided) > 0 && !decided[v.Rule] {
			record.IgnoredVerdicts = append(record.IgnoredVerdicts, v.normal())
			continue
		}
		callers = append(callers, v.normal())
	}
	sortKeys(record.IgnoredVerdicts)
	sortKeys(callers)
	count := func(rule string, add func(*RuleCounts)) {
		c := record.Rules[rule]
		add(&c)
		record.Rules[rule] = c
	}
	for _, f := range findings {
		agreed := false
		for _, v := range callers {
			if matches(v, f) {
				agreed = true
				break
			}
		}
		if agreed {
			record.Agreements = append(record.Agreements, f)
			count(f.Rule, func(c *RuleCounts) { c.Agreements++ })
		} else {
			record.CorvintOnly = append(record.CorvintOnly, f)
			count(f.Rule, func(c *RuleCounts) { c.CorvintOnly++ })
		}
	}
	for _, v := range callers {
		matched := false
		for _, f := range findings {
			if matches(v, f) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if a, ok := abstained(v, observed.Abstentions); ok {
			record.Abstentions = append(record.Abstentions, Abstention{Key: v, Reason: a.Reason})
			count(v.Rule, func(c *RuleCounts) { c.Abstentions++ })
			continue
		}
		record.CallerOnly = append(record.CallerOnly, v)
		count(v.Rule, func(c *RuleCounts) { c.CallerOnly++ })
	}
	return record
}

// matches reports a verdict naming a finding: the same rule, path, test and pair, and the same
// line unless the verdict gives none.
func matches(v, f Key) bool {
	return v.Rule == f.Rule && v.path() == f.path() && v.Test == f.Test && v.Other == f.Other && (v.Line == 0 || v.Line == f.Line)
}

// abstained finds the abstention whose scope covers a verdict.
func abstained(v Key, abstentions []Abstention) (Abstention, bool) {
	for _, a := range abstentions {
		if a.path() != "" && a.path() != v.path() || a.Rule != "" && a.Rule != v.Rule || a.Line != 0 && a.Line != v.Line ||
			a.Test != "" && a.Test != v.Test {
			continue
		}
		return a, true
	}
	return Abstention{}, false
}

func sortKeys(keys []Key) {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		switch {
		case a.Rule != b.Rule:
			return a.Rule < b.Rule
		case a.path() != b.path():
			return a.path() < b.path()
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Test != b.Test:
			return a.Test < b.Test
		}
		return a.Other < b.Other
	})
}

// Append writes one record as one JSONL line to the log, creating it with mode 0600. The line is
// written with one call on an O_APPEND descriptor (GVA-V0-005).
func Append(path string, record Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return refuse("agreement-log-unavailable", "the record could not be encoded")
	}
	if len(encoded)+1 > MaxRecordBytes {
		return refuse("agreement-log-unavailable", "the record is larger than %d bytes", MaxRecordBytes)
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return refuse("agreement-log-unavailable", "%s is not a regular file", path)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return refuse("agreement-log-unavailable", "%s cannot be opened for append", path)
	}
	_, werr := file.Write(append(encoded, '\n'))
	cerr := file.Close()
	if werr != nil || cerr != nil {
		return refuse("agreement-log-unavailable", "%s could not be written", path)
	}
	return nil
}

// RuleSummary is one rule's counts over the summarised runs (GVA-V0-007). AgreementRate is
// agreements over agreements, Corvint-only and caller-only outcomes; it is null when there were
// none. Abstentions are counted apart and never enter the rate.
type RuleSummary struct {
	Rule string `json:"rule"`
	// Runs is the number of compared runs in which the rule had an outcome.
	Runs int `json:"runs"`
	RuleCounts
	AgreementRate *float64 `json:"agreementRate"`
}

// IgnoredLine is a log line the summary did not count, with why.
type IgnoredLine struct {
	Log     string `json:"log"`
	Line    int    `json:"line"`
	Problem string `json:"problem"`
}

// Summary is the corvint-gate-agreement-summary/0 report.
type Summary struct {
	Schema       string         `json:"schema"`
	Logs         []string       `json:"logs"`
	Command      string         `json:"command,omitempty"`
	Runs         int            `json:"runs"`
	ComparedRuns int            `json:"comparedRuns"`
	Commands     map[string]int `json:"commands"`
	Rules        []RuleSummary  `json:"rules"`
	IgnoredLines []IgnoredLine  `json:"ignoredLines"`
}

// Summarise reads agreement logs and reports per-rule agreement over their runs, optionally only
// one command's (GVA-V0-007, GVA-V0-008). A line that is not a record is listed, never counted.
func Summarise(logs []string, command string, open func(string) (io.ReadCloser, error)) (Summary, error) {
	if len(logs) == 0 || len(logs) > MaxLogs {
		return Summary{}, refuse("agreement-log-unavailable", "name between 1 and %d logs", MaxLogs)
	}
	summary := Summary{Schema: SummarySchema, Logs: logs, Command: command, Commands: map[string]int{}, Rules: []RuleSummary{}, IgnoredLines: []IgnoredLine{}}
	totals := map[string]*RuleSummary{}
	for _, name := range logs {
		file, err := open(name)
		if err != nil {
			return Summary{}, refuse("agreement-log-unavailable", "%s cannot be read", name)
		}
		err = summariseOne(name, command, io.LimitReader(file, MaxLogBytes+1), &summary, totals)
		_ = file.Close()
		if err != nil {
			return Summary{}, err
		}
	}
	rules := make([]string, 0, len(totals))
	for rule := range totals {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	for _, rule := range rules {
		r := *totals[rule]
		if denominator := r.Agreements + r.CorvintOnly + r.CallerOnly; denominator > 0 {
			rate := float64(r.Agreements) / float64(denominator)
			rate = float64(int(rate*10000+0.5)) / 10000
			r.AgreementRate = &rate
		}
		summary.Rules = append(summary.Rules, r)
	}
	return summary, nil
}

func summariseOne(name, command string, reader io.Reader, summary *Summary, totals map[string]*RuleSummary) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64<<10), MaxRecordBytes)
	read, line := 0, 0
	for scanner.Scan() {
		line++
		read += len(scanner.Bytes()) + 1
		if read > MaxLogBytes {
			return refuse("agreement-log-unavailable", "%s is larger than %d bytes", name, MaxLogBytes)
		}
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil || record.Schema != RecordSchema || record.Command == "" {
			summary.IgnoredLines = append(summary.IgnoredLines, IgnoredLine{Log: name, Line: line, Problem: "not-a-record"})
			continue
		}
		if command != "" && record.Command != command {
			continue
		}
		summary.Runs++
		summary.Commands[record.Command]++
		if record.Comparison != StatusCompared {
			continue
		}
		summary.ComparedRuns++
		for rule, c := range record.Rules {
			t := totals[rule]
			if t == nil {
				t = &RuleSummary{Rule: rule}
				totals[rule] = t
			}
			t.Runs++
			t.Agreements += c.Agreements
			t.CorvintOnly += c.CorvintOnly
			t.CallerOnly += c.CallerOnly
			t.Abstentions += c.Abstentions
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return refuse("agreement-log-unavailable", "%s line %d is longer than %d bytes", name, line+1, MaxRecordBytes)
		}
		return refuse("agreement-log-unavailable", "%s cannot be read", name)
	}
	return nil
}
