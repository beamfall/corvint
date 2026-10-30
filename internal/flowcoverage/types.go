// Package flowcoverage binds accepted variation inventories to retained /3 observations.
package flowcoverage

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
)

const DenominatorSchema = "application-flow-coverage-denominator/1"
const RunsSchema = "application-flow-coverage-runs/1"
const ReportSchema = "application-flow-coverage/1"
const WritebackSchema = "application-flow-coverage-writeback/1"
const MaxReportBytes = 64 << 20
const MaxPageBytes = 300 << 10
const MaxRows = 8192

type Ref struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	SHA256   string `json:"sha256"`
}
type Inventory struct {
	Kind string `json:"kind"`
	Ref
}
type IntentDirectory struct {
	Directory string `json:"directory"`
	Revision  string `json:"revision"`
}
type Mapping struct {
	DocumentedID  string   `json:"documented_id"`
	IntentFlowIDs []string `json:"intent_flow_ids"`
}
type Citation struct {
	Anchors     []doccorpus.Anchor `json:"anchors"`
	IDs         []string           `json:"ids"`
	Unavailable string             `json:"unavailable"`
}
type Citations struct {
	Docs            Citation `json:"docs"`
	Claims          Citation `json:"claims"`
	SourceBranches  Citation `json:"source_branches"`
	LegacyTests     Citation `json:"legacy_tests"`
	DownstreamFlows Citation `json:"downstream_flows"`
}
type Entry struct {
	Locator    appflows.NavLocator `json:"locator"`
	Acceptance doccorpus.Anchor    `json:"acceptance"`
}
type ExactTest struct {
	File       string             `json:"file"`
	FullTitle  string             `json:"full_title"`
	Project    *string            `json:"project"`
	Anchor     doccorpus.Anchor   `json:"anchor"`
	Assertions []doccorpus.Anchor `json:"assertions"`
}
type Test struct {
	ExactTest
	Mode        string             `json:"mode"`
	Outcomes    []string           `json:"outcomes"`
	Controls    []ExactTest        `json:"controls"`
	Fixtures    []doccorpus.Anchor `json:"fixtures"`
	PageObjects []doccorpus.Anchor `json:"page_objects"`
	Acceptance  doccorpus.Anchor   `json:"acceptance"`
}
type Variation struct {
	FlowID    string    `json:"intent_flow_id"`
	ID        string    `json:"variation_id"`
	Entry     *Entry    `json:"entry,omitempty"`
	Citations Citations `json:"citations"`
	Tests     []Test    `json:"tests"`
	Exclusion *Ref      `json:"exclusion,omitempty"`
}
type Denominator struct {
	Schema     string               `json:"schema"`
	Source     doccorpus.Repository `json:"source"`
	Intents    IntentDirectory      `json:"intents"`
	Inventory  Inventory            `json:"documented_inventory"`
	Flows      []Mapping            `json:"flows"`
	Variations []Variation          `json:"variations"`
}
type Run struct {
	Ref
	Kind                string            `json:"kind"`
	SourceRevision      string            `json:"source_revision"`
	TestRevision        string            `json:"test_revision"`
	Paths               map[string]string `json:"paths"`
	PackagePaths        []string          `json:"package_paths"`
	BuildScope          string            `json:"build_scope"`
	ApplicationIdentity string            `json:"application_identity"`
	Environment         map[string]string `json:"environment"`
}
type Runs struct {
	Schema    string `json:"schema"`
	Directory string `json:"directory"`
	Revision  string `json:"revision"`
	Runs      []Run  `json:"runs"`
}
type Observation struct {
	HistoryGroup      string    `json:"history_group"`
	Historical        bool      `json:"historical"`
	BindingGroup      string    `json:"binding_group"`
	Receipt           string    `json:"receipt_sha256"`
	Kind              string    `json:"kind"`
	Test              ExactTest `json:"test"`
	State             string    `json:"state"`
	Attempts          int       `json:"attempts"`
	ConfiguredRetries []int     `json:"configured_retries"`
	Qualified         bool      `json:"qualified"`
	Reasons           []string  `json:"reasons"`
	SourceRevision    string    `json:"source_revision"`
}
type Skeleton struct {
	Entry       *appflows.NavLocator   `json:"entry"`
	Outcomes    []appflows.FlowOutcome `json:"outcomes"`
	Fixtures    []doccorpus.Anchor     `json:"fixtures"`
	PageObjects []doccorpus.Anchor     `json:"page_objects"`
	Unknown     []string               `json:"unknown"`
}
type Row struct {
	DocumentedID         string                 `json:"documented_id"`
	FlowID               string                 `json:"intent_flow_id"`
	VariationID          string                 `json:"variation_id"`
	Status               string                 `json:"status"`
	Actor                string                 `json:"actor"`
	Preconditions        []string               `json:"preconditions"`
	Entry                *appflows.NavLocator   `json:"entry"`
	Actions              []appflows.FlowStep    `json:"actions"`
	Facts                []string               `json:"observable_facts"`
	Outcomes             []appflows.FlowOutcome `json:"outcomes"`
	Citations            Citations              `json:"citations"`
	Tests                []Test                 `json:"tests"`
	Observations         []Observation          `json:"observations"`
	Reasons              []string               `json:"reasons"`
	LastVerifiedRevision string                 `json:"last_verified_revision"`
	Skeleton             *Skeleton              `json:"skeleton,omitempty"`
}
type Report struct {
	Schema            string   `json:"schema"`
	Revision          string   `json:"revision"`
	DenominatorSHA256 string   `json:"denominator_sha256"`
	RunsSHA256        string   `json:"runs_sha256"`
	Passed            bool     `json:"passed"`
	Gaps              int      `json:"gaps"`
	Rows              []Row    `json:"rows"`
	Limitations       []string `json:"limitations"`
}
type Result struct {
	Report     Report
	Generation *flowdocs.Result
	Documented []string
}
type Page struct {
	Schema       string   `json:"schema"`
	Revision     string   `json:"revision"`
	ReportSHA256 string   `json:"report_sha256"`
	Passed       bool     `json:"passed"`
	Gaps         int      `json:"gaps"`
	Total        int      `json:"total"`
	Offset       int      `json:"offset"`
	Next         *int     `json:"next_offset"`
	Rows         []Row    `json:"rows"`
	Limitations  []string `json:"limitations"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Encode(v any) ([]byte, error) {
	b, e := json.Marshal(v, json.Deterministic(true))
	if len(b) > MaxReportBytes {
		return nil, fmt.Errorf("flow coverage encoding bound exceeded")
	}
	return b, e
}
func decode(b []byte, v any) error {
	if len(b) > MaxReportBytes {
		return fmt.Errorf("flow coverage input bound exceeded")
	}
	return json.Unmarshal(b, v, json.RejectUnknownMembers(true))
}
func (r *Result) Page(offset, limit int) ([]byte, error) {
	if offset < 0 || offset > len(r.Report.Rows) || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid coverage page")
	}
	full, e := Encode(r.Report)
	if e != nil {
		return nil, e
	}
	end := min(offset+limit, len(r.Report.Rows))
	p := Page{Schema: ReportSchema, Revision: r.Report.Revision, ReportSHA256: digest(full), Passed: r.Report.Passed, Gaps: r.Report.Gaps, Total: len(r.Report.Rows), Offset: offset, Rows: r.Report.Rows[offset:end], Limitations: r.Report.Limitations}
	if end < len(r.Report.Rows) {
		p.Next = &end
	}
	b, e := Encode(p)
	if len(b) > MaxPageBytes {
		return nil, fmt.Errorf("coverage page exceeds byte bound; reduce limit")
	}
	return b, e
}
