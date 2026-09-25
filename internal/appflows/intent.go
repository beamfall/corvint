package appflows

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Wire identities and bounds of the AFU-V1 intent model (AFU-V1-001, AFU-V1-002, AFU-V1-037).
const (
	FlowIntentSchema = "application-flow-intent/1"
	RetiredSchema    = "application-flow-retired/1"
	RetiredFile      = "retired.json"
	OriginsFile      = "origins.json"
	MaxFlows         = 512
	maxFlowList      = 256
	maxFlowLinks     = 1024
	maxFlowText      = 4096
)

var (
	flowIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	memberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	oidPattern    = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// FlowIntent is one closed application-flow-intent/1 document (AFU-V1-001).
type FlowIntent struct {
	Schema        string          `json:"schema"`
	FlowID        string          `json:"flow_id"`
	Revision      int             `json:"revision"`
	Proposed      bool            `json:"proposed,omitempty"`
	Kind          string          `json:"kind"`
	Actor         string          `json:"actor"`
	Preconditions []string        `json:"preconditions"`
	Steps         []FlowStep      `json:"steps"`
	Outcomes      []FlowOutcome   `json:"outcomes"`
	Variations    []FlowVariation `json:"variations"`
	Links         []FlowLink      `json:"links"`
	Adapter       *FlowAdapter    `json:"adapter,omitempty"`
}

type FlowStep struct {
	StepID string `json:"step_id"`
	Action string `json:"action"`
}

type FlowOutcome struct {
	OutcomeID string `json:"outcome_id"`
	Behavior  string `json:"behavior"`
	Matcher   string `json:"matcher"`
	Locator   string `json:"locator"`
	Value     string `json:"value"`
}

type FlowVariation struct {
	VariationID     string   `json:"variation_id"`
	Preconditions   []string `json:"preconditions"`
	Steps           []string `json:"steps"`
	ObservableFacts []string `json:"observable_facts"`
	Outcomes        []string `json:"outcomes"`
	Projects        []string `json:"projects"`
}

// FlowLink is one stored forward link; `reviewed` is never stored, only evaluated (AFU-V1-008).
type FlowLink struct {
	From       string     `json:"from"`
	Basis      string     `json:"basis"`
	Target     LinkTarget `json:"target"`
	ReviewedAt string     `json:"reviewed_at,omitempty"`
}

// LinkTarget is a link's target identity (AFU-V1-007).
type LinkTarget struct {
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	TestKey   string `json:"test_key,omitempty"`
	Assertion string `json:"assertion,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

// FlowAdapter carries the DCP-V1-027 flow-record fields the intent model does not otherwise own.
type FlowAdapter struct {
	Derivation       string                    `json:"derivation"`
	Evidence         doccorpus.Anchor          `json:"evidence"`
	RequiredPages    []string                  `json:"required_pages"`
	NegativeControls []string                  `json:"negative_controls"`
	OrderedEvents    []doccorpus.BehaviorEvent `json:"ordered_events"`
	MissingReview    *doccorpus.Anchor         `json:"missing_e2e_review,omitempty"`
}

// Retired reserves deleted flow and variation IDs (AFU-V1-002).
type Retired struct {
	Schema       string   `json:"schema"`
	FlowIDs      []string `json:"flow_ids"`
	VariationIDs []string `json:"variation_ids"`
}

// IntentSet is the validated intent directory, flows sorted by ID.
type IntentSet struct {
	Dir     string
	Flows   []FlowIntent
	Retired Retired
	// Revision is the commit the set was read from by LoadIntentsAt; it is empty for a working-tree read.
	Revision string
}

// IntentPath is the repository-relative path of a flow's intent file.
func (s IntentSet) IntentPath(flowID string) string { return path.Join(s.Dir, flowID+".json") }

// FlowsDir checks that dir is a repository-relative real directory under root, following no symlink.
func FlowsDir(root, dir string) (string, error) {
	dir = filepath.ToSlash(dir)
	if !safePath(dir) || dir == "." {
		return "", errors.New("--flows must name a directory inside the repository root")
	}
	current := root
	for _, part := range strings.Split(dir, "/") {
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if err != nil || !st.IsDir() {
			return "", errors.New("--flows must name a real directory inside the repository root")
		}
	}
	return dir, nil
}

// LoadIntents reads and validates every intent in dir (AFU-V1-001, AFU-V1-002).
func LoadIntents(root, dir string) (IntentSet, error) {
	dir, err := FlowsDir(root, dir)
	if err != nil {
		return IntentSet{}, err
	}
	set := emptySet(dir)
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return set, errors.New("flows directory unreadable")
	}
	names := []string{}
	for _, entry := range entries {
		if intentCandidate(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if err = candidateBound(len(names)); err != nil {
		return set, err
	}
	for _, name := range names {
		raw, err := ReadFile(filepath.Join(root, dir, name))
		if err != nil {
			return set, fmt.Errorf("%s: %v", name, err)
		}
		if err = set.add(name, raw); err != nil {
			return set, err
		}
	}
	return set, set.validate()
}

func emptySet(dir string) IntentSet {
	return IntentSet{Dir: dir, Retired: Retired{Schema: RetiredSchema, FlowIDs: []string{}, VariationIDs: []string{}}}
}

// intentCandidate treats every name ending in .json in any letter case as an intent, so a case
// variant of an intent name is refused rather than skipped on a case-insensitive file system.
func intentCandidate(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".json") && name != OriginsFile
}

// candidateBound refuses more candidates than MaxFlows intents plus retired.json before any is read (AFU-V1-037).
func candidateBound(n int) error {
	if n > MaxFlows+1 {
		return fmt.Errorf("flows directory exceeds %d intents", MaxFlows)
	}
	return nil
}

func (s *IntentSet) add(name string, raw []byte) error {
	if name == RetiredFile {
		return decodeRetired(raw, &s.Retired)
	}
	var intent FlowIntent
	if err := Decode(raw, &intent); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	if intent.FlowID+".json" != name {
		return fmt.Errorf("%s: flow_id must equal the file name", name)
	}
	s.Flows = append(s.Flows, intent)
	return nil
}

func decodeRetired(raw []byte, out *Retired) error {
	if err := Decode(raw, out); err != nil {
		return fmt.Errorf("%s: %v", RetiredFile, err)
	}
	if out.Schema != RetiredSchema || !uniqueMatching(out.FlowIDs, flowIDPattern) || !uniqueMatching(out.VariationIDs, memberPattern) {
		return fmt.Errorf("%s: invalid retired ID list", RetiredFile)
	}
	return nil
}

func (s *IntentSet) validate() error {
	slices.SortFunc(s.Flows, func(a, b FlowIntent) int { return strings.Compare(a.FlowID, b.FlowID) })
	variations := map[string]string{}
	for _, intent := range s.Flows {
		if err := ValidateIntent(intent); err != nil {
			return fmt.Errorf("flow %s: %v", intent.FlowID, err)
		}
		if slices.Contains(s.Retired.FlowIDs, intent.FlowID) {
			return fmt.Errorf("flow %s: flow_id is retired and cannot be reused", intent.FlowID)
		}
		for _, v := range intent.Variations {
			if slices.Contains(s.Retired.VariationIDs, v.VariationID) {
				return fmt.Errorf("flow %s: variation_id %s is retired and cannot be reused", intent.FlowID, v.VariationID)
			}
			if owner, ok := variations[v.VariationID]; ok {
				return fmt.Errorf("variation_id %s is declared by flows %s and %s", v.VariationID, owner, intent.FlowID)
			}
			variations[v.VariationID] = intent.FlowID
		}
	}
	return nil
}

// ValidateIntent checks one intent's closed shape and internal references.
func ValidateIntent(f FlowIntent) error {
	if f.Schema != FlowIntentSchema {
		return errors.New("schema must be " + FlowIntentSchema)
	}
	if !validFlowID(f.FlowID) {
		return errors.New("invalid flow_id")
	}
	if f.Revision < 1 {
		return errors.New("revision must be a positive integer")
	}
	if f.Kind != "ui" && f.Kind != "api" {
		return errors.New("kind must be ui or api")
	}
	if !flowText(f.Actor) || !uniqueTexts(f.Preconditions) {
		return errors.New("invalid actor or preconditions")
	}
	if len(f.Steps) > maxFlowList || len(f.Outcomes) > maxFlowList || len(f.Variations) > maxFlowList || len(f.Links) > maxFlowLinks {
		return errors.New("intent list bound exceeded")
	}
	members, err := intentMembers(f)
	if err != nil {
		return err
	}
	for _, v := range f.Variations {
		if err = validateVariation(v, members); err != nil {
			return fmt.Errorf("variation %s: %v", v.VariationID, err)
		}
	}
	for i, l := range f.Links {
		if err = validateLink(l, members, f.Proposed); err != nil {
			return fmt.Errorf("links[%d]: %v", i, err)
		}
	}
	return validateAdapter(f.Adapter)
}

func validFlowID(id string) bool {
	return flowIDPattern.MatchString(id) && id+".json" != RetiredFile && id+".json" != OriginsFile
}

// intentMembers maps every step, outcome and variation ID, which share one namespace per flow.
func intentMembers(f FlowIntent) (map[string]string, error) {
	members := map[string]string{}
	add := func(id, kind string) error {
		if !memberPattern.MatchString(id) {
			return fmt.Errorf("invalid %s id %q", kind, id)
		}
		if _, dup := members[id]; dup {
			return fmt.Errorf("duplicate step, outcome or variation id %q", id)
		}
		members[id] = kind
		return nil
	}
	for _, s := range f.Steps {
		if err := add(s.StepID, "step"); err != nil {
			return nil, err
		}
		if !flowText(s.Action) {
			return nil, fmt.Errorf("step %s: invalid action", s.StepID)
		}
	}
	for _, o := range f.Outcomes {
		if err := add(o.OutcomeID, "outcome"); err != nil {
			return nil, err
		}
		if !flowText(o.Behavior) || !flowText(o.Matcher) || !flowText(o.Locator) || !flowText(o.Value) {
			return nil, fmt.Errorf("outcome %s: invalid behavior, matcher, locator or value", o.OutcomeID)
		}
	}
	for _, v := range f.Variations {
		if err := add(v.VariationID, "variation"); err != nil {
			return nil, err
		}
	}
	return members, nil
}

func validateVariation(v FlowVariation, members map[string]string) error {
	if !uniqueTexts(v.Preconditions) || !uniqueTexts(v.ObservableFacts) || !uniqueTexts(v.Projects) {
		return errors.New("invalid preconditions, observable_facts or projects")
	}
	if !uniqueTexts(v.Steps) || !uniqueTexts(v.Outcomes) {
		return errors.New("invalid step or outcome references")
	}
	for _, s := range v.Steps {
		if members[s] != "step" {
			return fmt.Errorf("unknown step %q", s)
		}
	}
	for _, o := range v.Outcomes {
		if members[o] != "outcome" {
			return fmt.Errorf("unknown outcome %q", o)
		}
	}
	return nil
}

func validateLink(l FlowLink, members map[string]string, proposed bool) error {
	if members[l.From] == "" {
		return fmt.Errorf("from %q names no step, outcome or variation", l.From)
	}
	if l.Basis != "declared" && l.Basis != "inferred" {
		return errors.New("stored basis must be declared or inferred")
	}
	if proposed && l.Basis != "inferred" {
		return errors.New("a proposed intent carries only inferred links")
	}
	if l.ReviewedAt != "" && (l.Basis != "declared" || !oidPattern.MatchString(l.ReviewedAt)) {
		return errors.New("reviewed_at must be a full commit ID on a declared link")
	}
	return validateTarget(l.Target)
}

func validateTarget(t LinkTarget) error {
	bad := fmt.Errorf("invalid %s target", t.Type)
	if t.Path != "" && !safePath(t.Path) {
		return bad
	}
	if (t.StartLine == 0) != (t.EndLine == 0) || t.StartLine < 0 || t.EndLine < t.StartLine {
		return bad
	}
	switch t.Type {
	case "source":
		if t.Path == "" || t.TestKey != "" || t.Assertion != "" || t.Digest != "" {
			return bad
		}
	case "test":
		if !flowText(t.TestKey) || t.Assertion != "" || t.Digest != "" {
			return bad
		}
	case "assertion":
		if !flowText(t.Assertion) || t.Path == "" || t.Digest != "" {
			return bad
		}
	case "evidence":
		if !sha256Pattern.MatchString(t.Digest) || t.Path != "" || t.StartLine != 0 || t.TestKey != "" || t.Assertion != "" {
			return bad
		}
	default:
		return errors.New("target type must be source, test, assertion or evidence")
	}
	return nil
}

func validateAdapter(a *FlowAdapter) error {
	if a == nil {
		return nil
	}
	if !slices.Contains([]string{"generated", "source-derived", "declared", "imported"}, a.Derivation) {
		return errors.New("adapter derivation is invalid")
	}
	if !uniqueTexts(a.RequiredPages) || !uniqueTexts(a.NegativeControls) || len(a.OrderedEvents) > maxFlowLinks {
		return errors.New("adapter pages, controls or events are invalid")
	}
	return nil
}

func flowText(s string) bool {
	if s == "" || len(s) > maxFlowText || !utf8.ValidString(s) {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

func uniqueTexts(values []string) bool {
	if len(values) > maxFlowList {
		return false
	}
	seen := map[string]bool{}
	for _, v := range values {
		if !flowText(v) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func uniqueMatching(values []string, pattern *regexp.Regexp) bool {
	if len(values) > MaxFlows*maxFlowList {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if !pattern.MatchString(v) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
