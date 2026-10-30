package console

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SnapshotProfile is the schema the console consumes from
// `corvint-dashboard-snapshot`, and ErrorProfile is what that tool answers with
// when it refuses.
const (
	SnapshotProfile = "corvint-dashboard-snapshot/0"
	ErrorProfile    = "corvint-dashboard-error/0"
)

// benchmarkResultsDir and agentMemoryDir are the two committed directories the
// S3 panes list. They are repository conventions, not console state: a
// directory the repository does not have lists as empty at that revision.
const (
	benchmarkResultsDir = "benchmarks/results"
	agentMemoryDir      = "docs/agent-memory"
)

// maxSnapshotBytes bounds one snapshot reply. A larger reply is reported as an
// unreadable source rather than buffered without limit.
const maxSnapshotBytes = 64 << 20

// metricGroups are the snapshot's metric families, in the order the pane
// renders them. The console does not invent a family: a key the snapshot stops
// emitting renders as a family the tool did not report.
var metricGroups = []string{"data", "usage", "verification", "frontier", "beamfall"}

// rawMetric is one snapshot metric as the tool wrote it. The six axes are
// decoded as plain strings and admitted through Axes.Set, so a value outside an
// axis's closed set stays unstated rather than reaching the page as an axis
// (LAC-V0-007).
type rawMetric struct {
	Name           string   `json:"name"`
	Unit           string   `json:"unit"`
	Value          *string  `json:"value"`
	Numerator      *string  `json:"numerator"`
	Denominator    *string  `json:"denominator"`
	ScopeClass     string   `json:"scopeClass"`
	SourceIDs      []string `json:"sourceIds"`
	Exclusions     []string `json:"exclusions"`
	Validity       string   `json:"validity"`
	EpistemicClass string   `json:"epistemicClass"`
	AuthorityClass string   `json:"authorityClass"`
	Completeness   string   `json:"completeness"`
	Currency       string   `json:"currency"`
	DeliveryStage  string   `json:"deliveryStage"`
	Dimensions     []struct {
		Name  string  `json:"name"`
		Value *string `json:"value"`
	} `json:"dimensions"`
}

// rawSnapshot is the part of `corvint-dashboard-snapshot/0` this pane renders.
type rawSnapshot struct {
	Schema         string              `json:"schema"`
	GeneratedAt    string              `json:"generatedAt"`
	SnapshotSHA256 string              `json:"snapshotSha256"`
	Observation    map[string]any      `json:"observation"`
	Repository     map[string]any      `json:"repository"`
	Privacy        map[string]any      `json:"privacy"`
	Sources        []rawSnapshotSource `json:"sources"`
	Issues         []rawIssue          `json:"issues"`
	Code           string              `json:"code"`
	Profile        string              `json:"profile"`
}

type rawSnapshotSource struct {
	ID             string   `json:"id"`
	DisplayLabel   string   `json:"displayLabel"`
	AdapterID      string   `json:"adapterId"`
	Profile        string   `json:"profile"`
	VerifierID     string   `json:"verifierId"`
	ContentSHA256  *string  `json:"contentSha256"`
	ByteCount      *string  `json:"byteCount"`
	Validity       string   `json:"validity"`
	EpistemicClass string   `json:"epistemicClass"`
	AuthorityClass string   `json:"authorityClass"`
	Completeness   string   `json:"completeness"`
	Currency       string   `json:"currency"`
	DeliveryStage  string   `json:"deliveryStage"`
	Exclusions     []string `json:"exclusions"`
}

type rawIssue struct {
	Code     string  `json:"code"`
	Severity string  `json:"severity"`
	SourceID *string `json:"sourceId"`
	Observed *string `json:"observed"`
	Limit    *string `json:"limit"`
}

// Metric is one rendered metric: the value the snapshot measured, the axes the
// snapshot stated for it, and every source that contributed to it.
type Metric struct {
	Name       string
	Unit       string
	Value      string
	Scope      string
	Dimensions string
	SourceIDs  []string
	Exclusions []string
	Axes       Axes
	Unmeasured bool
}

// MetricFamily is one metric family and its rows.
type MetricFamily struct {
	Name    string
	Metrics []Metric
}

// SnapshotSource is one contributing source of the snapshot, with the axes the
// snapshot stated for it.
type SnapshotSource struct {
	ID         string
	Label      string
	AdapterID  string
	Profile    string
	VerifierID string
	Digest     string
	Bytes      string
	Exclusions []string
	Axes       Axes
}

// Issue is one issue the snapshot raised about its own inputs.
type Issue struct {
	Code     string
	Severity string
	SourceID string
	Observed string
	Limit    string
}

// Evidence is the Corvint evidence pane: the compiled snapshot, its families,
// its sources and its issues. It is the first console surface whose source
// states all six axes for every value, so nothing here is Unstated by default
// (LAC-V0-007).
type Evidence struct {
	Schema      string
	GeneratedAt string
	Digest      string
	ScanState   string
	Worktree    string
	HeadRev     string
	TreeRev     string
	DirtyCount  string
	Privacy     []KeyValue
	Families    []MetricFamily
	Sources     []SnapshotSource
	Issues      []Issue
	Total       int
	Source      Source
	Err         string
	Refused     bool
	Code        string
}

// KeyValue is one declared field rendered as the tool stated it.
type KeyValue struct{ Key, Value string }

// Dashboard invokes `corvint-dashboard-snapshot` in one repository. The console
// compiles no snapshot of its own: it runs the owning tool and renders what
// that tool wrote, exactly as the board runs `atm` (LAC-V0-004, LAC-V0-012).
type Dashboard struct {
	Binary  string
	Root    string
	Timeout time.Duration
}

// Read compiles one snapshot and decodes it. The tool's own refusal is
// rendered as a refusal carrying its code, never as an empty evidence pane
// (LAC-V0-009).
func (d Dashboard) Read(ctx context.Context) *Evidence {
	argv := []string{d.Binary, "snapshot", "--root", d.Root}
	evidence := &Evidence{Source: Source{
		Argv: argv, Tool: d.Binary, Worktree: d.Root,
		ObservedAt: time.Now().UTC(), Axes: UnstatedAxes(),
	}}

	raw, stderr, runErr := runTool(ctx, d.Root, nil, d.Timeout, maxSnapshotBytes, argv...)
	if boundaryFailure(runErr) {
		evidence.Err = runErr.Error()
		return evidence
	}
	if len(raw) > maxSnapshotBytes {
		evidence.Err = fmt.Sprintf("%s replied with more than %d bytes", d.Binary, maxSnapshotBytes)
		return evidence
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		// The compiler refuses by writing its error object to stderr, with no
		// stdout, and exiting 2. Only that profile is admitted from stderr.
		var refusal rawSnapshot
		if runErr != nil && json.Unmarshal(stderr, &refusal) == nil && refusal.Profile == ErrorProfile && refusal.Code != "" {
			evidence.refuse(refusal.Code)
			return evidence
		}
		evidence.Err = describeExec(d.Binary, runErr, string(stderr))
		return evidence
	}
	// The snapshot is a JSON object on stdout, and the families are sibling
	// keys of the fixed fields, so the document is decoded twice: once into
	// the known shape and once as a map for them.
	var decoded rawSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		evidence.Err = fmt.Sprintf("%s did not reply with a %s snapshot: %v", d.Binary, SnapshotProfile, err)
		return evidence
	}
	if runErr != nil && decoded.Profile != ErrorProfile {
		evidence.Err = describeExec(d.Binary, runErr, "")
		return evidence
	}
	if decoded.Profile == ErrorProfile && decoded.Code == "" {
		evidence.Err = fmt.Sprintf("%s replied with a %s object without a code", d.Binary, ErrorProfile)
		return evidence
	}
	if decoded.Profile == ErrorProfile {
		evidence.refuse(decoded.Code)
		return evidence
	}
	if decoded.Schema != SnapshotProfile {
		evidence.Err = fmt.Sprintf("%s replied with schema %q, want %q", d.Binary, decoded.Schema, SnapshotProfile)
		return evidence
	}

	var families map[string]json.RawMessage
	if err := json.Unmarshal(raw, &families); err != nil {
		evidence.Err = fmt.Sprintf("%s replied with an undecodable snapshot: %v", d.Binary, err)
		return evidence
	}
	if err := evidence.fill(decoded, families); err != nil {
		evidence.Err = fmt.Sprintf("%s replied with an %v", d.Binary, err)
		return evidence
	}
	evidence.Source.Revision = evidence.HeadRev
	evidence.Source.Outcome = "OK"
	return evidence
}

// refuse records the compiler's own refusal and its code (LAC-V0-009).
func (e *Evidence) refuse(code string) {
	e.Refused = true
	e.Code = code
	e.Source.Outcome = "REFUSED"
	e.Source.Codes = []string{code}
}

// fill renders one decoded snapshot. Every axis is admitted from the value the
// snapshot stated; nothing here supplies one. A family that is present but
// undecodable fails the read: dropping it would render a source the console
// did not successfully read as if the tool had not reported it (LAC-V0-008).
func (e *Evidence) fill(decoded rawSnapshot, families map[string]json.RawMessage) error {
	e.Schema = decoded.Schema
	e.GeneratedAt = decoded.GeneratedAt
	e.Digest = decoded.SnapshotSHA256
	e.ScanState = declaredString(decoded.Observation, "scanState")
	e.Worktree = declaredString(decoded.Repository, "worktreeState")
	e.HeadRev = declaredString(decoded.Repository, "headRevision")
	e.TreeRev = declaredString(decoded.Repository, "treeRevision")
	e.DirtyCount = declaredString(decoded.Repository, "dirtyPathCount")
	e.Privacy = sortedPairs(decoded.Privacy)

	for _, group := range metricGroups {
		encoded, present := families[group]
		if !present {
			continue
		}
		var metrics []*rawMetric
		if err := json.Unmarshal(encoded, &metrics); err != nil {
			return fmt.Errorf("undecodable %q metric family: %v", group, err)
		}
		// JSON null decodes without error: a null family would render as an
		// empty one and a null metric as a nameless unmeasured row. The
		// snapshot states an empty group as [] and never a null metric.
		if metrics == nil {
			return fmt.Errorf("undecodable %q metric family: stated as null, not an array", group)
		}
		family := MetricFamily{Name: group}
		for _, metric := range metrics {
			if metric == nil {
				return fmt.Errorf("undecodable %q metric family: a metric stated as null", group)
			}
			family.Metrics = append(family.Metrics, renderMetric(*metric))
		}
		e.Total += len(family.Metrics)
		e.Families = append(e.Families, family)
	}

	for _, source := range decoded.Sources {
		e.Sources = append(e.Sources, SnapshotSource{
			ID: source.ID, Label: source.DisplayLabel, AdapterID: source.AdapterID,
			Profile: source.Profile, VerifierID: source.VerifierID,
			Digest: deref(source.ContentSHA256), Bytes: deref(source.ByteCount),
			Exclusions: source.Exclusions,
			Axes: statedAxes(source.Validity, source.EpistemicClass, source.AuthorityClass,
				source.Completeness, source.Currency, source.DeliveryStage),
		})
	}
	for _, issue := range decoded.Issues {
		e.Issues = append(e.Issues, Issue{
			Code: issue.Code, Severity: issue.Severity, SourceID: deref(issue.SourceID),
			Observed: deref(issue.Observed), Limit: deref(issue.Limit),
		})
	}
	return nil
}

// renderMetric turns one snapshot metric into a rendered row. A metric with no
// value is marked unmeasured rather than printed as a zero, which is the whole
// point of LAC-V0-008.
func renderMetric(metric rawMetric) Metric {
	row := Metric{
		Name: metric.Name, Unit: metric.Unit, Scope: metric.ScopeClass,
		SourceIDs: metric.SourceIDs, Exclusions: metric.Exclusions,
		Axes: statedAxes(metric.Validity, metric.EpistemicClass, metric.AuthorityClass,
			metric.Completeness, metric.Currency, metric.DeliveryStage),
	}
	switch {
	case metric.Value != nil:
		row.Value = *metric.Value
	case metric.Numerator != nil && metric.Denominator != nil:
		row.Value = *metric.Numerator + " / " + *metric.Denominator
	default:
		row.Unmeasured = true
	}
	var dimensions []string
	for _, dimension := range metric.Dimensions {
		if dimension.Value == nil {
			continue
		}
		dimensions = append(dimensions, dimension.Name+"="+*dimension.Value)
	}
	row.Dimensions = strings.Join(dimensions, " · ")
	return row
}

// statedAxes admits the six values the source stated. Admission is the whole
// mechanism: Axes.Set refuses a value outside the axis's closed set, so a
// vocabulary the console does not recognize leaves the axis unstated instead
// of being renamed into one (LAC-V0-007).
func statedAxes(validity, epistemic, authority, completeness, currency, delivery string) Axes {
	axes := UnstatedAxes()
	axes.Set(AxisValidity, validity)
	axes.Set(AxisEpistemicClass, epistemic)
	axes.Set(AxisAuthorityClass, authority)
	axes.Set(AxisCompleteness, completeness)
	axes.Set(AxisCurrency, currency)
	axes.Set(AxisDeliveryStage, delivery)
	return axes
}

func declaredString(fields map[string]any, key string) string {
	if value, ok := fields[key].(string); ok {
		return value
	}
	return ""
}

func sortedPairs(fields map[string]any) []KeyValue {
	pairs := make([]KeyValue, 0, len(fields))
	for key, value := range fields {
		pairs = append(pairs, KeyValue{Key: key, Value: renderValue(value)})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	return pairs
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
