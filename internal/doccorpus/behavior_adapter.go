package doccorpus

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const (
	BehaviorAdapterRequestSchema = "corvint-behavior-adapter-request/1"
	BehaviorAdapterResultSchema  = "corvint-behavior-adapter-result/1"
)

type BehaviorAdapterRequest struct {
	Schema                string                   `json:"schema"`
	ProviderID            string                   `json:"provider_id"`
	ProviderVersion       string                   `json:"provider_version"`
	ContractID            string                   `json:"contract_id"`
	Source                Repository               `json:"source"`
	Revisions             BehaviorRevisions        `json:"revisions"`
	SourceRevision        string                   `json:"source_revision"`
	DocumentationRevision string                   `json:"documentation_revision"`
	MigrationInput        string                   `json:"migration_input"`
	DiscoveryInput        string                   `json:"discovery_input"`
	Inputs                []BehaviorAdapterInput   `json:"inputs"`
	Mappings              []BehaviorAdapterMapping `json:"mappings"`
	Observations          []ObservationLink        `json:"observations"`
}

type BehaviorAdapterInput struct {
	ID       string `json:"id"`
	Anchor   Anchor `json:"anchor"`
	Document string `json:"document"`
}

type BehaviorAdapterMapping struct {
	Kind    string            `json:"kind"`
	Input   string            `json:"input"`
	Records string            `json:"records"`
	Fields  map[string]string `json:"fields"`
}

type BehaviorAdapterArtifact struct {
	Role     string `json:"role"`
	Input    string `json:"input"`
	Anchor   Anchor `json:"anchor"`
	SHA256   string `json:"sha256"`
	Document string `json:"document"`
}

type BehaviorAdapterCoverage struct {
	Metric      string   `json:"metric"`
	Value       int      `json:"value"`
	Denominator int      `json:"denominator"`
	Defined     bool     `json:"defined"`
	State       string   `json:"state"`
	Revision    string   `json:"revision"`
	Rule        string   `json:"rule"`
	Limitations []string `json:"limitations"`
}

type BehaviorAdapterOutcome struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
	Matcher  string `json:"matcher"`
	Locator  string `json:"locator"`
	Value    string `json:"value"`
}

type BehaviorAdapterVariation struct {
	ID               string                   `json:"variation_id"`
	Flow             string                   `json:"flow"`
	Preconditions    []string                 `json:"preconditions"`
	Actions          []string                 `json:"actions"`
	ObservableFacts  []string                 `json:"observable_facts"`
	ExpectedOutcomes []BehaviorAdapterOutcome `json:"expected_outcomes"`
	Projects         []string                 `json:"projects"`
	Tests            []string                 `json:"tests"`
}

type BehaviorAdapterTestClaim struct {
	VariationID     string   `json:"variation_id"`
	Preconditions   []string `json:"preconditions"`
	Actions         []string `json:"actions"`
	ObservableFacts []string `json:"observable_facts"`
}

type BehaviorAdapterClaimRecord struct {
	TestID string                   `json:"test_id"`
	Claim  BehaviorAdapterTestClaim `json:"claim"`
}

type BehaviorAdapterDiagnostic struct {
	Kind       string `json:"kind"`
	Subject    string `json:"subject"`
	Input      string `json:"input"`
	Field      string `json:"field"`
	Revision   string `json:"revision"`
	Digest     string `json:"digest"`
	Detail     string `json:"detail"`
	Correction string `json:"correction"`
}

type BehaviorAdapterDelta struct {
	AddedCriteria     []string `json:"added_criteria"`
	RemovedCriteria   []string `json:"removed_criteria"`
	ChangedCriteria   []string `json:"changed_criteria"`
	LostReverseLinks  []string `json:"lost_reverse_links"`
	PreviousAvailable bool     `json:"previous_available"`
}

type BehaviorAdapterResult struct {
	Schema      string                       `json:"schema"`
	Provider    ProviderRecord               `json:"provider"`
	Variations  []BehaviorAdapterVariation   `json:"variations"`
	Claims      []BehaviorAdapterClaimRecord `json:"claims"`
	Artifacts   []BehaviorAdapterArtifact    `json:"artifacts"`
	Coverage    []BehaviorAdapterCoverage    `json:"coverage"`
	Frontier    []BehaviorAdapterDiagnostic  `json:"frontier"`
	Delta       BehaviorAdapterDelta         `json:"delta"`
	Fallback    string                       `json:"fallback"`
	Limitations []string                     `json:"limitations"`
}

type behaviorAdapterOrigin struct {
	Input    BehaviorAdapterInput
	Record   string
	Field    string
	Revision string
	Digest   string
}

type behaviorAdapter struct {
	request               BehaviorAdapterRequest
	inputs                map[string]BehaviorAdapterInput
	inputKeys             map[string]bool
	mappings              map[string]BehaviorAdapterMapping
	origins               map[string]behaviorAdapterOrigin
	frontier              []BehaviorAdapterDiagnostic
	testClaims            map[string][]BehaviorAdapterTestClaim
	testReady             map[string]bool
	runtimeReady          map[string]bool
	linkedReady           map[string]bool
	variationRuntimeReady map[string]bool
}

var behaviorAdapterFields = map[string]map[string]bool{
	"flows": {
		"id": true, "derivation": true, "evidence": true, "required_pages": true,
		"negative_controls": true, "ordered_events": true, "missing_e2e_review": true,
	},
	"variations": {
		"id": true, "flow": true, "preconditions": true, "actions": true,
		"observable_facts": true, "expected_outcomes": true, "projects": true, "tests": true,
	},
	"candidates": {"id": true, "evidence": true, "flows": true},
	"tests": {
		"id": true, "project": true, "title": true, "evidence": true, "flows": true,
		"criteria": true, "assertions": true, "variation_claims": true,
		"runtime": true, "fixtures": true, "roles": true,
	},
}

var behaviorAdapterRequired = map[string][]string{
	"flows":      {"id", "derivation", "evidence", "required_pages", "negative_controls", "ordered_events"},
	"variations": {"id", "flow", "preconditions", "actions", "observable_facts", "expected_outcomes", "projects", "tests"},
	"candidates": {"id", "evidence", "flows"},
	"tests":      {"id", "project", "title", "evidence", "flows", "criteria", "assertions", "variation_claims"},
}

func BuildBehaviorAdapter(requestRaw, previousRaw []byte) (BehaviorAdapterResult, error) {
	var request BehaviorAdapterRequest
	if err := decode(requestRaw, &request); err != nil {
		return BehaviorAdapterResult{}, err
	}
	var previous *BehaviorAdapterResult
	if len(bytes.TrimSpace(previousRaw)) > 0 {
		var decoded BehaviorAdapterResult
		if err := decode(previousRaw, &decoded); err != nil {
			return BehaviorAdapterResult{}, fail("invalid previous behavior adapter result")
		}
		previous = &decoded
	}
	adapter, err := newBehaviorAdapter(request)
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	if previous != nil {
		if err := validatePreviousBehaviorAdapterResult(request, *previous); err != nil {
			return BehaviorAdapterResult{}, err
		}
	}
	return adapter.build(previous)
}

func validatePreviousBehaviorAdapterResult(request BehaviorAdapterRequest, result BehaviorAdapterResult) error {
	invalid := func(reason string) error { return fail("invalid previous behavior adapter result: " + reason) }
	if result.Schema != BehaviorAdapterResultSchema || result.Fallback != "full-relevant-suite" || result.Provider.Schema != BehaviorProviderSchema || result.Provider.ID != request.ProviderID || result.Provider.Version != request.ProviderVersion || result.Provider.Source.ID != request.Source.ID || result.Provider.BehaviorContracts == nil || len(result.Coverage) != 5 {
		return invalid("lineage")
	}
	registry := result.Provider.BehaviorContracts
	if registry.ContractID != request.ContractID || !validBehaviorRevisions(registry.Revisions) || result.Provider.Source != registry.Revisions.E2E || registry.SourceRevision != result.Provider.Source.Revision || registry.DocumentationRevision != registry.Revisions.Docs.Revision || !sameBehaviorRepositories(registry.Revisions, request.Revisions) {
		return invalid("contract lineage")
	}
	declarations := *registry
	declarations.ContractSHA256 = ""
	declarations.Stability = nil
	declarations.Tests = slices.Clone(registry.Tests)
	declarations.Legacy = slices.Clone(registry.Legacy)
	for index := range declarations.Legacy {
		declarations.Legacy[index].Runtime = nil
	}
	for index := range declarations.Tests {
		declarations.Tests[index].Runtime = nil
	}
	contractSHA256, err := hashValue(declarations)
	if err != nil || registry.ContractSHA256 != contractSHA256 {
		return invalid("contract digest")
	}
	if !validBehaviorAdapterArtifacts(result.Provider, registry, result.Artifacts) {
		return invalid("artifact digest")
	}
	variationIDs := make([]string, 0, len(result.Variations))
	for _, variation := range result.Variations {
		variationIDs = append(variationIDs, variation.ID)
		if !validBehaviorAdapterVariation(variation) {
			return invalid("variation")
		}
	}
	if !uniqueIdentities(variationIDs) {
		return invalid("duplicate variation")
	}
	claimIDs := map[string]bool{}
	for _, record := range result.Claims {
		key := record.TestID + "\x00" + record.Claim.VariationID
		if !textOK(record.TestID) || !validBehaviorAdapterClaims([]BehaviorAdapterTestClaim{record.Claim}) {
			return invalid("claim")
		}
		if claimIDs[key] {
			return invalid("duplicate claim")
		}
		claimIDs[key] = true
	}
	for _, link := range result.Delta.LostReverseLinks {
		// V1-0050: lost_reverse_links is a published text field; the corpus
		// text rule (textOK) was not enforced on read, so a NUL-joined value
		// from before this fix would otherwise round-trip silently.
		if !textOK(link) {
			return invalid("lost reverse link")
		}
	}
	metrics := make([]string, 0, len(result.Coverage))
	for _, row := range result.Coverage {
		metrics = append(metrics, row.Metric)
	}
	if !slices.Equal(metrics, []string{"documented_variations", "source_discovered_candidates", "discovered_project_executions", "linked_contracts", "runtime_witnessed_contracts"}) {
		return invalid("coverage")
	}
	if validateBehaviorAdapterDeclarations(registry.Flows, registry.Behaviors, registry.Tests) != nil {
		return invalid("provider declarations")
	}
	flows := map[string]BehaviorFlow{}
	tests := map[string]BehaviorTest{}
	variations := map[string]BehaviorAdapterVariation{}
	claims := map[string]BehaviorAdapterTestClaim{}
	for _, flow := range registry.Flows {
		flows[flow.ID] = flow
	}
	for _, test := range registry.Tests {
		tests[test.ID] = test
	}
	for _, variation := range result.Variations {
		variations[variation.ID] = variation
	}
	for _, record := range result.Claims {
		claims[record.TestID+"\x00"+record.Claim.VariationID] = record.Claim
	}
	for _, variation := range result.Variations {
		flow, ok := flows[variation.Flow]
		if !ok || !slices.Contains(flow.Criteria, variation.ID) {
			return invalid("variation flow")
		}
		for _, testID := range variation.Tests {
			test, ok := tests[testID]
			claim, claimed := claims[testID+"\x00"+variation.ID]
			if !ok || !slices.Contains(test.Criteria, variation.ID) || !claimed || !behaviorAdapterClaimMatches(claim, variation) {
				return invalid("variation test claim")
			}
			for _, outcome := range variation.ExpectedOutcomes {
				if !slices.ContainsFunc(test.Assertions, func(assertion BehaviorAssertion) bool {
					return behaviorAdapterOutcomeMatches(assertion, variation.ID, outcome)
				}) {
					return invalid("variation assertion")
				}
			}
		}
	}
	for _, test := range registry.Tests {
		for _, criterion := range test.Criteria {
			variation, ok := variations[criterion]
			if !ok {
				// V1-0044: forward emission (reconcileTest) does not prune a
				// criterion absent from the normative variation set; it retains
				// it and reports undocumented-tested-behavior. A prior result
				// carrying that finding is not invalid input.
				continue
			}
			claim, claimed := claims[test.ID+"\x00"+criterion]
			if !slices.Contains(variation.Tests, test.ID) || !claimed || !behaviorAdapterClaimMatches(claim, variation) {
				return invalid("test variation claim")
			}
		}
	}
	for _, record := range result.Claims {
		test, testExists := tests[record.TestID]
		if !testExists {
			return invalid("orphan claim")
		}
		variation, variationExists := variations[record.Claim.VariationID]
		if !variationExists {
			// Same dangling-reference tolerance as above, but only for a
			// variation the test itself still declares as a criterion: that is
			// the shape reconcileTest retains and reports as
			// undocumented-tested-behavior. A claim naming a variation the test
			// never declared is not a retained finding, it is corruption.
			if slices.Contains(test.Criteria, record.Claim.VariationID) {
				continue
			}
			return invalid("orphan claim")
		}
		if !slices.Contains(test.Criteria, variation.ID) || !slices.Contains(variation.Tests, test.ID) || !behaviorAdapterClaimMatches(record.Claim, variation) {
			return invalid("orphan claim")
		}
	}
	for _, link := range result.Provider.Observations {
		if _, ok := tests[link.TestID]; !ok || link.Subject != result.Provider.ID+":test:"+link.TestID || link.SourceRevision != registry.SourceRevision {
			return invalid("observation subject")
		}
	}
	return nil
}

func sameBehaviorRepositories(left, right BehaviorRevisions) bool {
	return left.App.ID == right.App.ID && left.E2E.ID == right.E2E.ID && left.Docs.ID == right.Docs.ID
}

func validBehaviorAdapterArtifacts(provider ProviderRecord, registry *BehaviorRegistry, artifacts []BehaviorAdapterArtifact) bool {
	if len(artifacts) == 0 || len(artifacts) > MaxRecords {
		return false
	}
	inputs := map[string]bool{}
	byAnchor := map[string]BehaviorAdapterArtifact{}
	for _, artifact := range artifacts {
		anchor := artifact.Anchor
		key := anchor.Revision + "\x00" + anchor.Path
		digest := Digest([]byte(artifact.Document))
		if !words("migration discovery runtime receipt")[artifact.Role] || !textOK(artifact.Input) || inputs[artifact.Input] || byAnchor[key].Input != "" || anchor.Repository != provider.Source.ID || !wire.IsGitOid(anchor.Revision) || !wire.IsGitOid(anchor.Blob) || !validPath(anchor.Path) || anchor.Start != 1 || anchor.End < 1 || anchor.Authority != "external-provider" || artifact.SHA256 != digest || anchor.SHA256 != digest || anchor.SpanSHA256 != digest {
			return false
		}
		inputs[artifact.Input] = true
		byAnchor[key] = artifact
	}
	used := map[string]bool{}
	artifactFor := func(anchor Anchor, role string) (BehaviorAdapterArtifact, bool) {
		key := anchor.Revision + "\x00" + anchor.Path
		artifact, ok := byAnchor[key]
		if !ok || artifact.Role != role || artifact.Anchor != anchor {
			return BehaviorAdapterArtifact{}, false
		}
		used[key] = true
		return artifact, true
	}
	migrationArtifact, ok := artifactFor(registry.Manifest, "migration")
	if !ok {
		return false
	}
	var migration BehaviorMigration
	if decode([]byte(migrationArtifact.Document), &migration) != nil || migration.Schema != registry.Schema || migration.ContractID != registry.ContractID || migration.SourceRevision != registry.SourceRevision || migration.DocumentationRevision != registry.DocumentationRevision || migration.Revisions != registry.Revisions {
		return false
	}
	discoveryArtifact, ok := artifactFor(registry.Discovery, "discovery")
	if !ok {
		return false
	}
	var discovery BehaviorDiscovery
	if decode([]byte(discoveryArtifact.Document), &discovery) != nil || discovery.Schema != "corvint-playwright-discovery/1" || discovery.Mode != "live-playwright-list" || discovery.Revisions != registry.Revisions {
		return false
	}
	for _, test := range registry.Tests {
		if test.Runtime == nil {
			continue
		}
		if _, ok := artifactFor(test.Runtime.Evidence, "runtime"); !ok {
			return false
		}
	}
	for _, observation := range provider.Observations {
		key := observation.InputRevision + "\x00" + observation.Input
		artifact, ok := byAnchor[key]
		if !ok || artifact.Role != "receipt" || artifact.SHA256 != observation.RunID {
			return false
		}
		used[key] = true
	}
	return len(used) == len(artifacts)
}

func newBehaviorAdapter(request BehaviorAdapterRequest) (*behaviorAdapter, error) {
	if request.Schema != BehaviorAdapterRequestSchema || !textOK(request.ProviderID) || !textOK(request.ProviderVersion) || !textOK(request.ContractID) {
		return nil, fail("invalid behavior adapter identity")
	}
	if !validBehaviorRevisions(request.Revisions) || request.Source != request.Revisions.E2E || request.SourceRevision != request.Source.Revision || !wire.IsGitOid(request.DocumentationRevision) {
		return nil, fail("invalid behavior adapter revision set")
	}
	if len(request.Inputs) > MaxRecords || len(request.Mappings) != len(behaviorAdapterFields) || len(request.Observations) > MaxRecords {
		return nil, fail("behavior adapter bound exceeded")
	}
	a := &behaviorAdapter{
		request: request, inputs: map[string]BehaviorAdapterInput{}, inputKeys: map[string]bool{},
		mappings: map[string]BehaviorAdapterMapping{}, origins: map[string]behaviorAdapterOrigin{},
		testClaims: map[string][]BehaviorAdapterTestClaim{}, testReady: map[string]bool{},
		runtimeReady: map[string]bool{}, linkedReady: map[string]bool{}, variationRuntimeReady: map[string]bool{},
	}
	for _, input := range request.Inputs {
		if err := a.addInput(input); err != nil {
			return nil, err
		}
	}
	if a.inputs[request.MigrationInput].ID == "" || a.inputs[request.DiscoveryInput].ID == "" {
		return nil, fail("behavior adapter required input missing")
	}
	for _, mapping := range request.Mappings {
		if err := a.addMapping(mapping); err != nil {
			return nil, err
		}
	}
	if err := validateBehaviorAdapterObservations(request); err != nil {
		return nil, err
	}
	return a, nil
}

func validateBehaviorAdapterObservations(request BehaviorAdapterRequest) error {
	ids := []string{}
	for _, link := range request.Observations {
		ids = append(ids, link.ID)
		if !strings.HasPrefix(link.ID, request.ProviderID+":") || !textOK(link.TestID) || !textOK(link.Project) || !textOK(link.Test) || !validPath(link.Input) || !wire.IsGitOid(link.InputRevision) || link.SourceRevision != request.SourceRevision || !wire.IsSha256(link.RunID) {
			return fail("invalid behavior adapter observation identity")
		}
		for _, mapped := range link.SourcePaths {
			if !validPath(mapped) {
				return fail("invalid behavior adapter observation source mapping")
			}
		}
	}
	if !uniqueIdentities(ids) {
		return fail("duplicate behavior adapter observation identity")
	}
	return nil
}

func (a *behaviorAdapter) addInput(input BehaviorAdapterInput) error {
	if !textOK(input.ID) || a.inputs[input.ID].ID != "" || len(input.Document) == 0 || len(input.Document) > MaxBytes {
		return fail("invalid or duplicate behavior adapter input")
	}
	key := input.Anchor.Revision + "\x00" + input.Anchor.Path
	if a.inputKeys[key] {
		return fail("duplicate behavior adapter input path identity")
	}
	document := []byte(input.Document)
	digest := Digest(document)
	anchor := input.Anchor
	if anchor.Repository != a.request.Source.ID || !wire.IsGitOid(anchor.Revision) || !wire.IsGitOid(anchor.Blob) {
		return a.fieldError(input, "anchor", "repository, revision or blob identity is invalid", "supply immutable Git identities from the provider repository")
	}
	if !validPath(anchor.Path) || anchor.Start != 1 || anchor.End < 1 {
		return a.fieldError(input, "anchor", "full-file path or line identity is invalid", "supply a repository-relative full-file anchor starting at line 1")
	}
	if anchor.SHA256 != digest || anchor.SpanSHA256 != digest {
		return a.fieldError(input, "anchor", "document digest differs from its full-file anchor", "recompute the SHA-256 identities from the exact input bytes")
	}
	if anchor.Authority != "external-provider" || !words("declared observed review imported")[anchor.Kind] || !textOK(anchor.Reason) {
		return a.fieldError(input, "anchor", "authority, evidence kind or reason is invalid", "retain external-provider authority and an attributed evidence reason")
	}
	var parsed any
	if err := json.Unmarshal(document, &parsed); err != nil {
		return a.fieldError(input, "/", "input document is not valid JSON", "supply one closed JSON document")
	}
	a.inputs[input.ID] = input
	a.inputKeys[key] = true
	return nil
}

func (a *behaviorAdapter) addMapping(mapping BehaviorAdapterMapping) error {
	allowed, ok := behaviorAdapterFields[mapping.Kind]
	input := a.inputs[mapping.Input]
	if !ok || a.mappings[mapping.Kind].Kind != "" || input.ID == "" || !validJSONPointer(mapping.Records) {
		return fail(fmt.Sprintf("behavior adapter input=%s field=%s: mapping identity, uniqueness or record pointer is invalid; map one known input and bounded record-list pointer", mapping.Input, mapping.Records))
	}
	if len(mapping.Fields) > 16 {
		return a.fieldError(input, mapping.Records, "field mapping bound exceeded", "map at most 16 closed fields")
	}
	names := make([]string, 0, len(mapping.Fields))
	for name := range mapping.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pointer := mapping.Fields[name]
		if !allowed[name] || !validJSONPointer(pointer) {
			return a.fieldError(input, pointer, "field mapping is unsupported or invalid: "+mapping.Kind+"."+name, "use one supported bounded JSON pointer")
		}
	}
	for _, name := range behaviorAdapterRequired[mapping.Kind] {
		if _, ok := mapping.Fields[name]; !ok {
			return a.fieldError(input, mapping.Records, "required field mapping is missing: "+mapping.Kind+"."+name, "map every required field")
		}
	}
	a.mappings[mapping.Kind] = mapping
	return nil
}

func validJSONPointer(pointer string) bool {
	if pointer == "" {
		return true
	}
	if len(pointer) > 1024 || !strings.HasPrefix(pointer, "/") || len(strings.Split(pointer, "/"))-1 > 16 {
		return false
	}
	for index := 0; index < len(pointer); index++ {
		if pointer[index] == '~' && (index+1 >= len(pointer) || pointer[index+1] != '0' && pointer[index+1] != '1') {
			return false
		}
		if pointer[index] == '~' {
			index++
		}
	}
	return true
}

func resolveJSONPointer(value any, pointer string) (any, bool) {
	if pointer == "" {
		return value, true
	}
	current := value
	for _, token := range strings.Split(pointer[1:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		if object, ok := current.(map[string]any); ok {
			current, ok = object[token]
			if !ok {
				return nil, false
			}
			continue
		}
		array, ok := current.([]any)
		if !ok {
			return nil, false
		}
		index, err := strconv.Atoi(token)
		if err != nil || index < 0 || index >= len(array) || strconv.Itoa(index) != token {
			return nil, false
		}
		current = array[index]
	}
	return current, true
}

func (a *behaviorAdapter) records(kind string) ([]any, BehaviorAdapterMapping, error) {
	mapping := a.mappings[kind]
	input := a.inputs[mapping.Input]
	var document any
	if err := json.Unmarshal([]byte(input.Document), &document); err != nil {
		return nil, mapping, fail("invalid behavior adapter input document: " + input.ID)
	}
	value, ok := resolveJSONPointer(document, mapping.Records)
	if !ok {
		return nil, mapping, a.fieldError(input, mapping.Records, "record list is missing", "map the exact record-list pointer")
	}
	records, ok := value.([]any)
	if !ok || len(records) > MaxRecords {
		return nil, mapping, a.fieldError(input, mapping.Records, "record list is not a bounded array", "supply an array with at most 4096 records")
	}
	return records, mapping, nil
}

func (a *behaviorAdapter) mapped(record any, mapping BehaviorAdapterMapping, index int, name string, target any, required bool) (bool, error) {
	pointer, configured := mapping.Fields[name]
	if !configured {
		return false, nil
	}
	value, present := resolveJSONPointer(record, pointer)
	if !present {
		if !required {
			return false, nil
		}
		return false, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, name), "required field is missing", "map a present field with the required closed shape")
	}
	raw, err := json.Marshal(value, json.Deterministic(true))
	if err != nil || decode(raw, target) != nil {
		return false, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, name), "field has the wrong closed shape", "supply the mapped field using the documented type")
	}
	return true, nil
}

func behaviorAdapterRecordPointer(mapping BehaviorAdapterMapping, index int) string {
	return mapping.Records + "/" + strconv.Itoa(index)
}

func behaviorAdapterFieldPointer(mapping BehaviorAdapterMapping, index int, name string) string {
	return behaviorAdapterRecordPointer(mapping, index) + mapping.Fields[name]
}

func (a *behaviorAdapter) fieldError(input BehaviorAdapterInput, field, detail, correction string) error {
	return fail(fmt.Sprintf("behavior adapter input=%s field=%s revision=%s digest=%s: %s; %s", input.ID, field, input.Anchor.Revision, input.Anchor.SHA256, detail, correction))
}

func (a *behaviorAdapter) build(previous *BehaviorAdapterResult) (BehaviorAdapterResult, error) {
	_, discovery, err := a.baseArtifacts()
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	flows, err := a.flows()
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	variations, err := a.variations()
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	behaviors, err := a.candidates()
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	tests, err := a.tests()
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	flows = a.attachBehaviorVariations(flows, variations)
	if err := a.validateMappedBehaviorAdapterDeclarations(flows, behaviors, tests); err != nil {
		return BehaviorAdapterResult{}, err
	}
	registry := BehaviorRegistry{Schema: 2, ContractID: a.request.ContractID, SourceRevision: a.request.SourceRevision, DocumentationRevision: a.request.DocumentationRevision, Revisions: a.request.Revisions, Manifest: a.inputs[a.request.MigrationInput].Anchor, Discovery: a.inputs[a.request.DiscoveryInput].Anchor, Flows: flows, Behaviors: behaviors, Tests: tests}
	declarations := registry
	declarations.Tests = slices.Clone(registry.Tests)
	for index := range declarations.Tests {
		declarations.Tests[index].Runtime = nil
	}
	contractSHA256, err := hashValue(declarations)
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	registry.ContractSHA256 = contractSHA256
	if err := a.validateObservationSubjects(tests); err != nil {
		return BehaviorAdapterResult{}, err
	}
	provider := a.provider(registry)
	a.reconcile(&provider, discovery, variations)
	artifacts, err := a.artifacts(provider)
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	result := BehaviorAdapterResult{Schema: BehaviorAdapterResultSchema, Provider: provider, Variations: variations, Claims: a.claimRecords(), Artifacts: artifacts, Frontier: a.frontier, Fallback: "full-relevant-suite", Limitations: behaviorAdapterLimitations()}
	result.Coverage = behaviorAdapterCoverage(provider, discovery, variations, a.linkedReady, a.variationRuntimeReady, result.Limitations)
	delta, err := behaviorAdapterDelta(previous, result)
	if err != nil {
		return BehaviorAdapterResult{}, err
	}
	result.Delta = delta
	sortBehaviorAdapterResult(&result)
	return result, nil
}

func (a *behaviorAdapter) baseArtifacts() (BehaviorMigration, BehaviorDiscovery, error) {
	var migration BehaviorMigration
	migrationInput := a.inputs[a.request.MigrationInput]
	if err := decode([]byte(migrationInput.Document), &migration); err != nil || migration.Schema != 2 || migration.ContractID != a.request.ContractID || migration.SourceRevision != a.request.SourceRevision || migration.DocumentationRevision != a.request.DocumentationRevision || migration.Revisions != a.request.Revisions {
		return migration, BehaviorDiscovery{}, a.fieldError(migrationInput, "", "migration identity does not match the request", "supply the exact schema-2 migration record")
	}
	var discovery BehaviorDiscovery
	discoveryInput := a.inputs[a.request.DiscoveryInput]
	if err := decode([]byte(discoveryInput.Document), &discovery); err != nil || discovery.Schema != "corvint-playwright-discovery/1" || discovery.Mode != "live-playwright-list" || discovery.Revisions != a.request.Revisions || len(discovery.Executions) > MaxRecords {
		return migration, discovery, a.fieldError(discoveryInput, "", "discovery identity does not match the request", "supply the exact live Playwright discovery record")
	}
	executionIDs := make([]string, 0, len(discovery.Executions))
	for _, execution := range discovery.Executions {
		executionIDs = append(executionIDs, execution.ID)
		if !textOK(execution.Project) {
			return migration, discovery, a.fieldError(discoveryInput, "/executions", "discovery project is missing", "retain the exact live test/project identity")
		}
	}
	if !uniqueIdentities(executionIDs) {
		return migration, discovery, a.fieldError(discoveryInput, "/executions", "discovery execution identity is duplicate or invalid", "retain one exact execution identity per discovered test")
	}
	return migration, discovery, nil
}

func (a *behaviorAdapter) flows() ([]BehaviorFlow, error) {
	records, mapping, err := a.records("flows")
	if err != nil {
		return nil, err
	}
	flows := make([]BehaviorFlow, 0, len(records))
	for index, record := range records {
		flow := BehaviorFlow{}
		for _, field := range []struct {
			name   string
			target any
		}{{"id", &flow.ID}, {"derivation", &flow.Derivation}, {"evidence", &flow.Evidence}, {"required_pages", &flow.RequiredPages}, {"negative_controls", &flow.NegativeControls}, {"ordered_events", &flow.OrderedEvents}} {
			if _, err := a.mapped(record, mapping, index, field.name, field.target, true); err != nil {
				return nil, err
			}
		}
		if _, err := a.mapped(record, mapping, index, "missing_e2e_review", &flow.MissingReview, false); err != nil {
			return nil, err
		}
		if !textOK(flow.ID) {
			return nil, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, "id"), "flow id is invalid", "supply a nonempty stable flow id")
		}
		a.origin("flow:"+flow.ID, mapping, index, "id")
		flows = append(flows, flow)
	}
	return uniqueBehaviorFlows(flows)
}

func (a *behaviorAdapter) variations() ([]BehaviorAdapterVariation, error) {
	records, mapping, err := a.records("variations")
	if err != nil {
		return nil, err
	}
	variations := make([]BehaviorAdapterVariation, 0, len(records))
	for index, record := range records {
		variation := BehaviorAdapterVariation{}
		fields := []struct {
			name   string
			target any
		}{{"id", &variation.ID}, {"flow", &variation.Flow}, {"preconditions", &variation.Preconditions}, {"actions", &variation.Actions}, {"observable_facts", &variation.ObservableFacts}, {"expected_outcomes", &variation.ExpectedOutcomes}, {"projects", &variation.Projects}, {"tests", &variation.Tests}}
		for _, field := range fields {
			if _, err := a.mapped(record, mapping, index, field.name, field.target, true); err != nil {
				return nil, err
			}
		}
		if !validBehaviorAdapterVariation(variation) {
			return nil, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, "id"), "variation identity or semantic contract is invalid", "supply one globally stable variation with explicit actions, facts, outcomes, projects and unique exact tests")
		}
		sort.Strings(variation.Preconditions)
		sort.Strings(variation.ObservableFacts)
		sort.Strings(variation.Projects)
		sort.Strings(variation.Tests)
		sort.Slice(variation.ExpectedOutcomes, func(i, j int) bool { return variation.ExpectedOutcomes[i].ID < variation.ExpectedOutcomes[j].ID })
		a.origin("variation:"+variation.ID, mapping, index, "id")
		variations = append(variations, variation)
	}
	sort.Slice(variations, func(i, j int) bool { return variations[i].ID < variations[j].ID })
	for index := 1; index < len(variations); index++ {
		if variations[index-1].ID == variations[index].ID {
			return nil, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, "id"), "variation identity is duplicated", "supply one globally unique variation identity")
		}
	}
	return variations, nil
}

func validBehaviorAdapterVariation(variation BehaviorAdapterVariation) bool {
	if !textOK(variation.ID) || !textOK(variation.Flow) || len(variation.Actions) == 0 || len(variation.ObservableFacts) == 0 || len(variation.ExpectedOutcomes) == 0 || len(variation.Projects) == 0 {
		return false
	}
	if !uniqueIdentities(variation.Preconditions) || !uniqueIdentities(variation.Actions) || !uniqueIdentities(variation.ObservableFacts) || !uniqueIdentities(variation.Projects) || !uniqueIdentities(variation.Tests) {
		return false
	}
	ids := make([]string, 0, len(variation.ExpectedOutcomes))
	for _, outcome := range variation.ExpectedOutcomes {
		ids = append(ids, outcome.ID)
		if !textOK(outcome.Behavior) || !textOK(outcome.Matcher) || !textOK(outcome.Locator) || !textOK(outcome.Value) {
			return false
		}
	}
	return uniqueIdentities(ids)
}

func (a *behaviorAdapter) candidates() ([]BehaviorSource, error) {
	records, mapping, err := a.records("candidates")
	if err != nil {
		return nil, err
	}
	values := make([]BehaviorSource, 0, len(records))
	for index, record := range records {
		value := BehaviorSource{}
		for _, field := range []struct {
			name   string
			target any
		}{{"id", &value.ID}, {"evidence", &value.Evidence}, {"flows", &value.Flows}} {
			if _, err := a.mapped(record, mapping, index, field.name, field.target, true); err != nil {
				return nil, err
			}
		}
		sort.Strings(value.Flows)
		a.origin("candidate:"+value.ID, mapping, index, "id")
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	if !uniqueSourceIDs(values) {
		return nil, a.fieldError(a.inputs[mapping.Input], mapping.Records, "candidate identity is duplicate or invalid", "supply globally unique nonempty candidate identities")
	}
	return values, nil
}

func (a *behaviorAdapter) tests() ([]BehaviorTest, error) {
	records, mapping, err := a.records("tests")
	if err != nil {
		return nil, err
	}
	values := make([]BehaviorTest, 0, len(records))
	for index, record := range records {
		value := BehaviorTest{}
		claims := []BehaviorAdapterTestClaim{}
		required := []struct {
			name   string
			target any
		}{{"id", &value.ID}, {"project", &value.Project}, {"title", &value.Title}, {"evidence", &value.Evidence}, {"flows", &value.Flows}, {"criteria", &value.Criteria}, {"assertions", &value.Assertions}}
		for _, field := range required {
			if _, err := a.mapped(record, mapping, index, field.name, field.target, true); err != nil {
				return nil, err
			}
		}
		for _, field := range []struct {
			name   string
			target any
		}{{"runtime", &value.Runtime}, {"fixtures", &value.Fixtures}, {"roles", &value.Roles}} {
			if _, err := a.mapped(record, mapping, index, field.name, field.target, false); err != nil {
				return nil, err
			}
		}
		if _, err := a.mapped(record, mapping, index, "variation_claims", &claims, true); err != nil {
			return nil, err
		}
		sort.Strings(value.Flows)
		sort.Strings(value.Criteria)
		for index := range claims {
			sort.Strings(claims[index].Preconditions)
			sort.Strings(claims[index].ObservableFacts)
		}
		sort.Slice(claims, func(i, j int) bool { return claims[i].VariationID < claims[j].VariationID })
		if !validBehaviorAdapterClaims(claims) {
			return nil, a.fieldError(a.inputs[mapping.Input], behaviorAdapterFieldPointer(mapping, index, "variation_claims"), "test variation claims are invalid", "supply unique stable variation identities and explicit semantic fields")
		}
		a.origin("test:"+value.ID, mapping, index, "id")
		a.testClaims[value.ID] = claims
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].ID+"\x00"+values[i].Project < values[j].ID+"\x00"+values[j].Project
	})
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	if !uniqueIdentities(ids) {
		return nil, a.fieldError(a.inputs[mapping.Input], mapping.Records, "test identity is duplicate or invalid", "supply globally unique nonempty test identities")
	}
	return values, nil
}

func validBehaviorAdapterClaims(claims []BehaviorAdapterTestClaim) bool {
	ids := make([]string, 0, len(claims))
	for _, claim := range claims {
		ids = append(ids, claim.VariationID)
		if !uniqueIdentities(claim.Preconditions) || !uniqueIdentities(claim.Actions) || !uniqueIdentities(claim.ObservableFacts) {
			return false
		}
	}
	return uniqueIdentities(ids)
}

func (a *behaviorAdapter) origin(key string, mapping BehaviorAdapterMapping, index int, field string) {
	input := a.inputs[mapping.Input]
	a.origins[key] = behaviorAdapterOrigin{Input: input, Record: behaviorAdapterRecordPointer(mapping, index), Field: behaviorAdapterFieldPointer(mapping, index, field), Revision: input.Anchor.Revision, Digest: input.Anchor.SHA256}
}

func uniqueBehaviorFlows(values []BehaviorFlow) ([]BehaviorFlow, error) {
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	for index := 1; index < len(values); index++ {
		if values[index-1].ID == values[index].ID {
			return nil, fail("duplicate behavior adapter flow identity")
		}
	}
	return values, nil
}

func uniqueSourceIDs(values []BehaviorSource) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1].ID == values[index].ID || !textOK(values[index].ID) {
			return false
		}
	}
	return len(values) == 0 || textOK(values[0].ID)
}

func (a *behaviorAdapter) attachBehaviorVariations(flows []BehaviorFlow, variations []BehaviorAdapterVariation) []BehaviorFlow {
	byID := map[string]int{}
	for index, flow := range flows {
		byID[flow.ID] = index
	}
	for _, variation := range variations {
		index, ok := byID[variation.Flow]
		if !ok {
			continue
		}
		flows[index].Criteria = append(flows[index].Criteria, variation.ID)
		flows[index].Tests = append(flows[index].Tests, variation.Tests...)
		origin := a.origins["variation:"+variation.ID]
		origin.Field = origin.Record + a.mappings["variations"].Fields["tests"]
		for _, testID := range variation.Tests {
			a.origins["flow-test:"+variation.Flow+":"+testID] = origin
		}
	}
	for index := range flows {
		flows[index].Criteria = sortedUnique(flows[index].Criteria)
		flows[index].Tests = sortedUnique(flows[index].Tests)
	}
	return flows
}

func validateBehaviorAdapterDeclarations(flows []BehaviorFlow, behaviors []BehaviorSource, tests []BehaviorTest) error {
	ids := []string{}
	for _, flow := range flows {
		ids = append(ids, flow.ID)
		if !words("generated source-derived declared imported")[flow.Derivation] || !uniqueIdentities(flow.Criteria) || !uniqueIdentities(flow.Tests) || !uniqueIdentities(flow.RequiredPages) || !uniqueIdentities(flow.NegativeControls) || len(flow.OrderedEvents) > MaxRecords {
			return fail("invalid mapped behavior flow")
		}
	}
	for _, behavior := range behaviors {
		ids = append(ids, behavior.ID)
		if !uniqueIdentities(behavior.Flows) {
			return fail("invalid mapped source candidate")
		}
	}
	for _, test := range tests {
		ids = append(ids, test.ID)
		if !textOK(test.Project) || !textOK(test.Title) || !uniqueIdentities(test.Flows) || !uniqueIdentities(test.Criteria) || len(test.Assertions) > MaxRecords {
			return fail("invalid mapped behavior test")
		}
		assertionIDs := []string{}
		for _, assertion := range test.Assertions {
			assertionIDs = append(assertionIDs, assertion.ID)
			if !textOK(assertion.Behavior) || !textOK(assertion.Criterion) || !textOK(assertion.Matcher) || !textOK(assertion.Locator) || !textOK(assertion.Value) {
				return fail("incomplete mapped assertion identity")
			}
		}
		if !uniqueIdentities(assertionIDs) {
			return fail("duplicate mapped assertion identity")
		}
	}
	if !uniqueIdentities(ids) {
		return fail("duplicate mapped behavior identity")
	}
	return nil
}

func (a *behaviorAdapter) validateMappedBehaviorAdapterDeclarations(flows []BehaviorFlow, behaviors []BehaviorSource, tests []BehaviorTest) error {
	if err := validateBehaviorAdapterDeclarations(flows, behaviors, tests); err == nil {
		return nil
	}
	for _, flow := range flows {
		if !words("generated source-derived declared imported")[flow.Derivation] {
			return a.originError("flow:"+flow.ID, "derivation", "mapped flow derivation is invalid", "supply one supported derivation")
		}
		if !uniqueIdentities(flow.RequiredPages) {
			return a.originError("flow:"+flow.ID, "required_pages", "mapped required-page identities are duplicate or invalid", "supply unique exact required-page identities")
		}
		if !uniqueIdentities(flow.NegativeControls) {
			return a.originError("flow:"+flow.ID, "negative_controls", "mapped negative-control identities are duplicate or invalid", "supply unique exact negative-control identities")
		}
		if len(flow.OrderedEvents) > MaxRecords {
			return a.originError("flow:"+flow.ID, "ordered_events", "mapped ordered-event list exceeds the bound", "supply at most 4096 ordered events")
		}
		if !uniqueIdentities(flow.Criteria) || !uniqueIdentities(flow.Tests) {
			return a.originError("flow:"+flow.ID, "id", "derived flow relation identities are invalid", "supply unique exact variation and test relations")
		}
	}
	for _, behavior := range behaviors {
		if !uniqueIdentities(behavior.Flows) {
			return a.originError("candidate:"+behavior.ID, "flows", "mapped source candidate flow identities are invalid", "supply unique exact documented-flow proposals")
		}
	}
	for _, test := range tests {
		if !textOK(test.Project) {
			return a.originError("test:"+test.ID, "project", "mapped test project is invalid", "supply one exact nonempty project identity")
		}
		if !textOK(test.Title) {
			return a.originError("test:"+test.ID, "title", "mapped test title is invalid", "supply one exact nonempty title")
		}
		if !uniqueIdentities(test.Flows) {
			return a.originError("test:"+test.ID, "flows", "mapped test flow identities are duplicate or invalid", "supply unique exact flow identities")
		}
		if !uniqueIdentities(test.Criteria) {
			return a.originError("test:"+test.ID, "criteria", "mapped test criterion identities are duplicate or invalid", "supply unique exact variation identities")
		}
		if len(test.Assertions) > MaxRecords {
			return a.originError("test:"+test.ID, "assertions", "mapped assertion list exceeds the bound", "supply at most 4096 assertions")
		}
		assertionIDs := []string{}
		for _, assertion := range test.Assertions {
			assertionIDs = append(assertionIDs, assertion.ID)
			if !textOK(assertion.Behavior) || !textOK(assertion.Criterion) || !textOK(assertion.Matcher) || !textOK(assertion.Locator) || !textOK(assertion.Value) {
				return a.originError("test:"+test.ID, "assertions", "mapped assertion identity is incomplete", "supply exact behavior, criterion, matcher, locator and value fields")
			}
		}
		if !uniqueIdentities(assertionIDs) {
			return a.originError("test:"+test.ID, "assertions", "mapped assertion identity is duplicated", "supply globally unique assertion identities")
		}
	}
	return a.fieldError(a.inputs[a.request.DiscoveryInput], "/", "mapped behavior identity is duplicated", "supply globally unique flow, candidate and test identities")
}

func (a *behaviorAdapter) originError(originKey, field, detail, correction string) error {
	origin := a.origins[originKey]
	if mapping, ok := a.mappings[strings.Split(originKey, ":")[0]+"s"]; ok {
		if pointer, exists := mapping.Fields[field]; exists {
			origin.Field = origin.Record + pointer
		}
	}
	return a.fieldError(origin.Input, origin.Field, detail, correction)
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func (a *behaviorAdapter) provider(registry BehaviorRegistry) ProviderRecord {
	provider := ProviderRecord{Schema: BehaviorProviderSchema, ID: a.request.ProviderID, Version: a.request.ProviderVersion, Source: a.request.Source, BehaviorContracts: &registry}
	provider.Observations = slices.Clone(a.request.Observations)
	for _, test := range registry.Tests {
		provider.Subjects = append(provider.Subjects, Subject{ID: a.request.ProviderID + ":test:" + test.ID, Kind: "test", Name: test.Title, Provider: a.request.ProviderID, Evidence: behaviorAdapterEvidence(test.Evidence)})
	}
	if len(provider.Subjects) > 0 {
		provider.Capabilities = append(provider.Capabilities, CapabilityDeclaration{Name: "subjects", State: "present", Reason: "mapped behavior test declarations"})
	}
	if len(provider.Observations) > 0 {
		provider.Capabilities = append(provider.Capabilities, CapabilityDeclaration{Name: "observations", State: "present", Reason: "caller-retained qualified receipt links"})
	}
	return provider
}

func (a *behaviorAdapter) validateObservationSubjects(tests []BehaviorTest) error {
	testIDs := map[string]bool{}
	for _, test := range tests {
		testIDs[test.ID] = true
	}
	for _, link := range a.request.Observations {
		expected := a.request.ProviderID + ":test:" + link.TestID
		if !testIDs[link.TestID] || link.Subject != expected {
			return fail("behavior adapter observation subject must name the exact generated test subject")
		}
	}
	return nil
}

func (a *behaviorAdapter) claimRecords() []BehaviorAdapterClaimRecord {
	records := []BehaviorAdapterClaimRecord{}
	for testID, claims := range a.testClaims {
		for _, claim := range claims {
			records = append(records, BehaviorAdapterClaimRecord{TestID: testID, Claim: claim})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].TestID+"\x00"+records[i].Claim.VariationID < records[j].TestID+"\x00"+records[j].Claim.VariationID
	})
	return records
}

func behaviorAdapterEvidence(anchor Anchor) Evidence {
	return Evidence{Derivation: "imported", Trust: "generated", State: "unknown", Freshness: "unknown", Anchors: []Anchor{anchor}, Unknown: "adapter declaration requires corpus validation", Limitations: []string{"mapping and structural reconciliation do not authenticate runtime or assertion adequacy"}}
}

func behaviorAdapterLimitations() []string {
	return []string{
		"experimental producer; exact consumer fixtures and external utility are not qualified",
		"mapped records and runtime witnesses remain caller-owned declarations until corpus Git rebinding",
		"runtime-witnessed is not semantic adequacy, authenticated execution or current served content",
		"full relevant suite fallback always retained; no narrowing authority",
	}
}

func (a *behaviorAdapter) reconcile(provider *ProviderRecord, discovery BehaviorDiscovery, variations []BehaviorAdapterVariation) {
	registry := provider.BehaviorContracts
	discoveryInput := a.inputs[a.request.DiscoveryInput]
	variationByID := map[string]BehaviorAdapterVariation{}
	for _, variation := range variations {
		variationByID[variation.ID] = variation
	}
	normativeReviewed := true
	for _, kind := range []string{"flows", "variations"} {
		input := a.inputs[a.mappings[kind].Input]
		if input.Anchor.Kind != "review" {
			normativeReviewed = false
			a.gapFromInput(input, "unreviewed-normative-input", provider.ID, a.mappings[kind].Records, "normative documentation input lacks an explicit retained review", "mark the immutable caller-reviewed inventory as review evidence; its contained anchors still bind the declared documentation revision")
		}
	}
	if discovery.Config.Revision != registry.SourceRevision {
		a.gapFromInput(discoveryInput, "stale-anchor", provider.ID, "/config", "discovery configuration differs from the source revision", "refresh live discovery from the declared source revision")
	}
	for _, execution := range discovery.Executions {
		if execution.Evidence.Revision != registry.SourceRevision {
			a.gapFromInput(discoveryInput, "stale-anchor", execution.ID, "/executions", "discovered execution evidence differs from the source revision", "refresh the exact test/project discovery anchor")
		}
	}
	flows := map[string]BehaviorFlow{}
	tests := map[string]BehaviorTest{}
	for _, flow := range registry.Flows {
		flows[flow.ID] = flow
		if flow.Evidence.Revision != registry.DocumentationRevision {
			a.gap("flow:"+flow.ID, "stale-anchor", flow.ID, "evidence", "flow evidence revision differs from the documentation revision", "rebind the flow anchor to the declared documentation revision")
		}
	}
	for _, test := range registry.Tests {
		tests[test.ID] = test
		a.reconcileTest(*registry, discovery, flows, variationByID, test, provider.Observations)
	}
	for _, flow := range registry.Flows {
		a.reconcileFlow(flow, tests, len(discovery.Executions) > 0)
	}
	for _, candidate := range registry.Behaviors {
		if candidate.Evidence.Revision != registry.SourceRevision {
			a.gap("candidate:"+candidate.ID, "stale-anchor", candidate.ID, "evidence", "source candidate evidence differs from the source revision", "rebind the candidate anchor to the declared source revision")
		}
		linkedTest := false
		if len(candidate.Flows) == 0 {
			a.gap("candidate:"+candidate.ID, "source-candidate-orphan", candidate.ID, "flows", "source-discovered candidate has no documented flow proposal", "map a reviewed flow or retain the candidate as an explicit orphan proposal")
		}
		for _, flow := range candidate.Flows {
			mapped, ok := flows[flow]
			if !ok {
				a.gap("candidate:"+candidate.ID, "source-candidate-orphan", candidate.ID, "flows", "source-discovered candidate has no exact documented flow", "review and map the candidate to an accepted flow or retain it as proposed")
				continue
			}
			if len(mapped.Tests) > 0 {
				linkedTest = true
			}
		}
		if len(candidate.Flows) > 0 && !linkedTest {
			a.gap("candidate:"+candidate.ID, "source-candidate-orphan", candidate.ID, "flows", "source-discovered candidate has no exact test-backed variation", "review the candidate and link an accepted criterion to an exact test")
		}
	}
	for _, execution := range discovery.Executions {
		test, ok := tests[execution.ID]
		if !ok || test.Project != execution.Project {
			a.gapFromInput(a.inputs[a.request.DiscoveryInput], "unregistered-discovered-execution", execution.ID, "/executions", "live discovery execution has no exact test/project contract", "add a reviewed contract or retain the execution as unreviewed")
		}
	}
	a.reconcileVariations(variations, flows, tests, discovery, normativeReviewed)
	if len(registry.Flows) == 0 || len(registry.Tests) == 0 {
		a.gapFromInput(a.inputs[a.mappings["flows"].Input], "unreviewed", provider.ID, a.mappings["flows"].Records, "empty or incomplete inventory cannot establish complete coverage", "supply reviewed flows, criteria and exact tests")
	}
}

func (a *behaviorAdapter) reconcileTest(registry BehaviorRegistry, discovery BehaviorDiscovery, flows map[string]BehaviorFlow, variations map[string]BehaviorAdapterVariation, test BehaviorTest, observations []ObservationLink) {
	contractReady := true
	if test.Evidence.Revision != registry.SourceRevision {
		contractReady = false
		a.gap("test:"+test.ID, "stale-anchor", test.ID, "evidence", "test evidence revision differs from the source revision", "rebind the test anchor to the declared source revision")
	}
	discovered := false
	for _, execution := range discovery.Executions {
		if execution.ID == test.ID && execution.Project == test.Project && execution.Evidence.Revision == test.Evidence.Revision && execution.Evidence.Path == test.Evidence.Path && execution.Evidence.SHA256 == test.Evidence.SHA256 && execution.Evidence.Start == test.Evidence.Start {
			discovered = true
		}
	}
	if !discovered {
		contractReady = false
		a.gap("test:"+test.ID, "unregistered-test-execution", test.ID, "project", "test/project/source is absent from bound live discovery", "refresh discovery and map the exact execution identity")
	}
	if len(test.Assertions) == 0 {
		contractReady = false
		a.gap("test:"+test.ID, "assertion-free-ui-execution", test.ID, "assertions", "test has no exact assertion identity", "add reviewed matcher, locator and expected-value assertions")
	}
	claims := a.testClaims[test.ID]
	for _, variationID := range test.Criteria {
		variation, exists := variations[variationID]
		claim, claimed := behaviorAdapterClaim(claims, variationID)
		if !exists {
			contractReady = false
			a.gap("test:"+test.ID, "undocumented-tested-behavior", test.ID, "criteria", "test references a variation absent from the normative documentation contract", "review the behavior into the normative inventory or remove the proposed test claim")
			continue
		}
		if !claimed || !behaviorAdapterClaimMatches(claim, variation) {
			contractReady = false
			a.gap("test:"+test.ID, "semantic-mismatch", test.ID, "variation_claims", "test preconditions, actions or observable facts differ from the normative variation", "copy no proposal; review and align the exact semantic claim with the normative variation")
		}
		if !slices.Contains(variation.Projects, test.Project) {
			contractReady = false
			a.gap("test:"+test.ID, "extra-project-witness", test.ID, "project", "test project is not allowed by the normative variation", "review the project into the normative contract or use an allowed exact project")
		}
		for _, outcome := range variation.ExpectedOutcomes {
			if !slices.ContainsFunc(test.Assertions, func(assertion BehaviorAssertion) bool {
				return behaviorAdapterOutcomeMatches(assertion, variation.ID, outcome)
			}) {
				contractReady = false
				a.gap("test:"+test.ID, "semantic-mismatch", test.ID, "assertions", "normative expected outcome has no exact declared assertion: "+outcome.ID, "declare the same criterion, behavior, matcher, locator and value")
			}
		}
		for _, assertion := range test.Assertions {
			if assertion.Criterion == variation.ID && !slices.ContainsFunc(variation.ExpectedOutcomes, func(outcome BehaviorAdapterOutcome) bool {
				return behaviorAdapterOutcomeMatches(assertion, variation.ID, outcome)
			}) {
				contractReady = false
				a.gap("test:"+test.ID, "semantic-mismatch", test.ID, "assertions", "test assertion is absent from the normative expected outcomes: "+assertion.ID, "review the outcome into the normative contract or remove the extra assertion claim")
			}
		}
	}
	for _, claim := range claims {
		if !slices.Contains(test.Criteria, claim.VariationID) {
			contractReady = false
			a.gap("test:"+test.ID, "undocumented-tested-behavior", test.ID, "variation_claims", "test semantic claim lacks the same exact criterion identity", "add the reviewed variation identity to both sides or retain the claim as proposed")
		}
	}
	for _, criterion := range test.Criteria {
		if !slices.ContainsFunc(test.Assertions, func(assertion BehaviorAssertion) bool {
			return assertion.Criterion == criterion && behaviorAssertionValid(registry, test, assertion)
		}) {
			contractReady = false
			a.gap("test:"+test.ID, "assertion-contradiction", test.ID, "assertions", "criterion lacks an exact reviewed behavior assertion", "map the criterion to a reviewed assertion and source candidate sharing the documented flow")
		}
	}
	for _, assertion := range test.Assertions {
		if !behaviorAssertionValid(registry, test, assertion) {
			contractReady = false
			a.gap("test:"+test.ID, "assertion-contradiction", test.ID, "assertions", "assertion has an undeclared, unreviewed or stale behavior/criterion identity", "bind the assertion to the exact candidate, criterion, source anchor and reviewed annotation")
		}
	}
	if len(test.Flows) == 0 {
		contractReady = false
		a.gap("test:"+test.ID, "missing-reverse-link", test.ID, "flows", "test does not identify a documented flow", "map the exact documented flow and reciprocal criterion relation")
	}
	for _, flowID := range test.Flows {
		flow, ok := flows[flowID]
		if !ok || !slices.Contains(flow.Tests, test.ID) {
			contractReady = false
			a.gap("test:"+test.ID, "missing-reverse-link", test.ID, "flows", "test to flow join lacks the reverse exact test identity", "add the same test/criterion relation to the flow inventory")
			continue
		}
		for _, criterion := range test.Criteria {
			if !slices.Contains(flow.Criteria, criterion) {
				contractReady = false
				a.gap("test:"+test.ID, "criterion-contradiction", test.ID, "criteria", "test criterion is absent from its linked flow", "use the same criterion set on both sides of the relation")
			}
		}
	}
	witnessed := a.runtimeWitnessed(registry, discovery, flows, test, observations)
	a.testReady[test.ID] = contractReady
	a.runtimeReady[test.ID] = contractReady && witnessed
}

func behaviorAdapterClaim(claims []BehaviorAdapterTestClaim, variationID string) (BehaviorAdapterTestClaim, bool) {
	for _, claim := range claims {
		if claim.VariationID == variationID {
			return claim, true
		}
	}
	return BehaviorAdapterTestClaim{}, false
}

func behaviorAdapterClaimMatches(claim BehaviorAdapterTestClaim, variation BehaviorAdapterVariation) bool {
	return claim.VariationID == variation.ID && slices.Equal(claim.Preconditions, variation.Preconditions) && slices.Equal(claim.Actions, variation.Actions) && slices.Equal(claim.ObservableFacts, variation.ObservableFacts)
}

func behaviorAdapterOutcomeMatches(assertion BehaviorAssertion, variationID string, outcome BehaviorAdapterOutcome) bool {
	return assertion.ID == outcome.ID && assertion.Criterion == variationID && assertion.Behavior == outcome.Behavior && assertion.Matcher == outcome.Matcher && assertion.Locator == outcome.Locator && assertion.Value == outcome.Value
}

func (a *behaviorAdapter) reconcileVariations(variations []BehaviorAdapterVariation, flows map[string]BehaviorFlow, tests map[string]BehaviorTest, discovery BehaviorDiscovery, normativeReviewed bool) {
	for _, variation := range variations {
		ready := normativeReviewed
		runtimeProjects := map[string]bool{}
		declaredProjects := map[string]bool{}
		discoveredProjects := map[string]bool{}
		flow, flowExists := flows[variation.Flow]
		if !flowExists || !slices.Contains(flow.Criteria, variation.ID) {
			ready = false
			a.gap("variation:"+variation.ID, "documented-variation-orphan", variation.ID, "flow", "normative variation names a missing documented flow", "map the stable variation identity to an exact reviewed flow")
		} else {
			if flow.Evidence.Revision != a.request.DocumentationRevision {
				ready = false
			}
			if len(flow.RequiredPages) == 0 || len(flow.OrderedEvents) == 0 {
				ready = false
				a.gap("variation:"+variation.ID, "semantic-mismatch", variation.ID, "flow", "normative flow lacks explicit required pages or ordered events", "review the complete page and event contract for this variation")
			}
		}
		if len(variation.Tests) == 0 {
			ready = false
			a.gap("variation:"+variation.ID, "documented-untested-behavior", variation.ID, "tests", "normative variation has no exact test identity", "retain the missing evidence or add an exact reviewed test/project claim")
		}
		for _, testID := range variation.Tests {
			test, exists := tests[testID]
			if !exists || !slices.Contains(test.Criteria, variation.ID) {
				ready = false
				a.gap("variation:"+variation.ID, "missing-reverse-link", variation.ID, "tests", "normative variation to test join lacks the reverse exact variation identity", "add the same stable variation identity to the exact test claim")
				continue
			}
			if !a.testReady[testID] {
				ready = false
			}
			if slices.Contains(variation.Projects, test.Project) {
				declaredProjects[test.Project] = true
			}
			for _, execution := range discovery.Executions {
				if execution.ID != test.ID {
					continue
				}
				if !slices.Contains(variation.Projects, execution.Project) {
					ready = false
					a.gap("variation:"+variation.ID, "extra-project-witness", variation.ID, "projects", "discovered execution uses a project outside the normative variation: "+execution.Project, "review the project into the contract or retain the execution as proposed")
					continue
				}
				if execution.Project == test.Project {
					discoveredProjects[execution.Project] = true
					if a.runtimeReady[testID] {
						runtimeProjects[execution.Project] = true
					}
				}
			}
		}
		for _, test := range tests {
			if slices.Contains(test.Criteria, variation.ID) && !slices.Contains(variation.Tests, test.ID) {
				ready = false
				a.gap("variation:"+variation.ID, "missing-reverse-link", variation.ID, "tests", "test references the variation but the normative reverse test list omits it: "+test.ID, "add the exact test identity to the reviewed variation or remove the proposed reference")
			}
		}
		allRuntime := ready
		for _, project := range variation.Projects {
			if !declaredProjects[project] || !discoveredProjects[project] {
				ready = false
				allRuntime = false
				a.gap("variation:"+variation.ID, "missing-project-witness", variation.ID, "projects", "allowed project lacks an exact declared and discovered execution: "+project, "add the exact test/project execution or retain the missing evidence")
			}
			if !runtimeProjects[project] {
				allRuntime = false
			}
		}
		a.linkedReady[variation.ID] = ready
		a.variationRuntimeReady[variation.ID] = ready && allRuntime
	}
}

func (a *behaviorAdapter) reconcileFlow(flow BehaviorFlow, tests map[string]BehaviorTest, hasDiscovery bool) {
	for _, testID := range flow.Tests {
		test, ok := tests[testID]
		if !ok || !slices.Contains(test.Flows, flow.ID) {
			a.gap("flow-test:"+flow.ID+":"+testID, "missing-reverse-link", flow.ID, "tests", "flow to test join lacks the reverse exact flow identity", "add the same flow identity to the exact test contract")
		}
	}
	for _, criterion := range flow.Criteria {
		covered := false
		for _, testID := range flow.Tests {
			if slices.Contains(tests[testID].Criteria, criterion) {
				covered = true
			}
		}
		if !covered {
			a.gap("variation:"+criterion, "documented-variation-orphan", criterion, "tests", "documented variation has no exact test join", "link the variation to an exact test/project or retain it as unreviewed")
		}
	}
	if len(flow.Tests) == 0 {
		kind := "unreviewed"
		if hasDiscovery && flow.MissingReview != nil && flow.MissingReview.Kind == "review" && flow.MissingReview.Revision == a.request.DocumentationRevision {
			kind = "confirmed-missing_e2e"
		}
		a.gap("flow:"+flow.ID, kind, flow.ID, "tests", "flow has no joined tests", "retain unreviewed or supply an explicit current reviewed absence decision")
	}
}

func (a *behaviorAdapter) runtimeWitnessed(registry BehaviorRegistry, discovery BehaviorDiscovery, flows map[string]BehaviorFlow, test BehaviorTest, observations []ObservationLink) bool {
	if test.Runtime == nil {
		a.gap("test:"+test.ID, "missing-runtime-witness", test.ID, "runtime", "test has no immutable runtime witness", "supply a bound behavior run and qualified native receipt")
		return false
	}
	runInput, ok := a.inputForAnchor(test.Runtime.Evidence)
	if !ok {
		a.gap("test:"+test.ID, "missing-runtime-witness", test.ID, "runtime", "runtime anchor is not one of the retained inputs", "include the exact runtime document and matching full-file anchor")
		return false
	}
	var run BehaviorRun
	if decode([]byte(runInput.Document), &run) != nil || run.Schema != "corvint-behavior-run/1" || run.ContractID != registry.ContractID || run.ContractSHA256 != registry.ContractSHA256 || run.SourceRevision != registry.SourceRevision || run.DocumentationRevision != registry.DocumentationRevision || run.Revisions != registry.Revisions || run.TestID != test.ID || run.Project != test.Project || !wire.IsSha256(run.RunSHA256) || run.Retry < 0 || run.Cleanup != "passed" || !slices.Equal(run.Fixtures, test.Fixtures) || !slices.Equal(run.Roles, test.Roles) {
		a.gapFromInput(runInput, "runtime-identity-contradiction", test.ID, "", "runtime identity, cleanup or contract digest does not match", "regenerate the runtime witness from the exact contract and execution")
		return false
	}
	link, ok := observationByID(observations, test.Runtime.Observation)
	if !ok || link.TestID != test.ID || link.Project != test.Project || link.Test != test.Title || link.SourceRevision != registry.SourceRevision {
		a.gap("test:"+test.ID, "runtime-identity-contradiction", test.ID, "runtime", "runtime observation lacks the exact test/project/title join", "map the qualified receipt observation to the exact contract identity")
		return false
	}
	receiptInput, ok := a.inputForPath(link.InputRevision, link.Input)
	if !ok || Digest([]byte(receiptInput.Document)) != run.RunSHA256 || link.RunID != run.RunSHA256 {
		a.gap("test:"+test.ID, "missing-runtime-witness", test.ID, "runtime", "qualified receipt bytes do not match the run identity", "retain the exact native receipt and its SHA-256 identity")
		return false
	}
	decoded, err := testvaliditydoc.Decode([]byte(receiptInput.Document))
	if err != nil {
		a.gapFromInput(receiptInput, "missing-runtime-witness", test.ID, "", "native receipt is not a qualified closed provider document", "supply canonical qualified Playwright receipt bytes")
		return false
	}
	document := testvaliditydoc.Project(decoded)
	if !behaviorAdapterReceiptMatches(document, discovery, link, test, run) {
		a.gapFromInput(receiptInput, "runtime-identity-contradiction", test.ID, "", "native receipt lacks the exact passing test/project/retry", "retain the matching qualified execution rather than a similarly named run")
		return false
	}
	return a.runtimeEventsMatch(flows, test, run)
}

func behaviorAdapterReceiptMatches(document testvaliditydoc.Document, discovery BehaviorDiscovery, link ObservationLink, test BehaviorTest, run BehaviorRun) bool {
	if document.Playwright == nil || document.TestsOmitted != 0 || document.Run.Execution.State == "INCOMPLETE" || document.Run.Freshness.State == "STALE" {
		return false
	}
	identity := document.Playwright.Identity
	if identity.ConfigDigest != discovery.Config.SHA256 {
		return false
	}
	configPath := identity.ConfigFile
	if mapped, ok := link.SourcePaths[configPath]; ok {
		configPath = mapped
	}
	if configPath != discovery.Config.Path {
		return false
	}
	bound := false
	for _, native := range document.Playwright.Tests {
		if native.ID != test.ID || native.Anchor == nil {
			continue
		}
		path := native.Anchor.File
		if mapped, ok := link.SourcePaths[path]; ok {
			path = mapped
		}
		if path == test.Evidence.Path && identity.TestFileDigests[native.Anchor.File] == test.Evidence.SHA256 && native.Anchor.Line >= test.Evidence.Start && native.Anchor.Line <= test.Evidence.End {
			bound = true
		}
	}
	if !bound {
		return false
	}
	for _, observed := range document.Tests {
		if observed.ID != test.ID || observed.Project == nil || observed.Project.Name != test.Project || observed.Name != test.Title || observed.State != "passed" || observed.Projection.Execution.State != "PASSED" {
			continue
		}
		for _, attempt := range observed.Attempts {
			if attempt.Retry == run.Retry && attempt.State == "passed" {
				return true
			}
		}
	}
	return false
}

func (a *behaviorAdapter) runtimeEventsMatch(flows map[string]BehaviorFlow, test BehaviorTest, run BehaviorRun) bool {
	valid := len(run.Events) > 0 && len(run.Events) <= MaxRecords
	if !valid {
		a.gap("test:"+test.ID, "invalid-runtime-event", test.ID, "runtime", "runtime event list is empty or exceeds the bound", "retain one bounded complete ordered event sequence")
	}
	seen := map[string]bool{}
	for index, event := range run.Events {
		key := event.Kind + ":" + event.ID
		invalid := event.Sequence != index+1 || !event.Passed || !textOK(event.ID) || !textOK(event.Context) || !textOK(event.Page) || !textOK(event.Frame) || !words("page assertion negative-control")[event.Kind] || seen[key]
		if invalid {
			valid = false
			a.gap("test:"+test.ID, "invalid-runtime-event", test.ID, "runtime", "runtime event identity, sequence, result or browser context is invalid: "+event.ID, "retain one unique passing closed event at its exact sequence")
		}
		if event.Kind == "page" && !words("main-frame frame redirect popup setup")[event.Navigation] {
			valid = false
			a.gap("test:"+test.ID, "invalid-runtime-event", test.ID, "runtime", "page event navigation is invalid: "+event.ID, "use a closed page navigation identity")
		}
		if event.Navigation == "popup" && !textOK(event.ParentPage) {
			valid = false
			a.gap("test:"+test.ID, "invalid-runtime-event", test.ID, "runtime", "popup event lacks its parent page: "+event.ID, "retain the exact parent page identity")
		}
		seen[key] = true
		if event.Kind == "assertion" && !slices.ContainsFunc(test.Assertions, func(assertion BehaviorAssertion) bool { return behaviorAssertionEventMatches(event, assertion) }) {
			valid = false
			a.gap("test:"+test.ID, "assertion-contradiction", test.ID, "assertions", "runtime matcher/locator/value does not match its reviewed assertion", "retain the exact assertion event identity")
		}
	}
	for _, assertion := range test.Assertions {
		if !slices.ContainsFunc(run.Events, func(event BehaviorEvent) bool { return behaviorAssertionEventMatches(event, assertion) }) {
			valid = false
			a.gap("test:"+test.ID, "missing-assertion-event", test.ID, "assertions", "reviewed assertion has no exact runtime event", "record the matcher, locator and value event")
		}
	}
	for _, flowID := range test.Flows {
		flow, ok := flows[flowID]
		if !ok {
			valid = false
			continue
		}
		if !slices.Equal(flow.OrderedEvents, run.Events) {
			valid = false
			a.gap("test:"+test.ID, "out-of-order-events", test.ID, "runtime", "runtime event sequence differs from the declared flow", "retain the complete ordered page/assertion/control sequence")
		}
		for _, page := range flow.RequiredPages {
			if !seen["page:"+page] {
				valid = false
				a.gap("test:"+test.ID, "missing-page-event", test.ID, "runtime", "required page event is absent: "+page, "record the exact page/frame/navigation event")
			}
		}
		for _, control := range flow.NegativeControls {
			if !seen["negative-control:"+control] {
				valid = false
				a.gap("test:"+test.ID, "missing-negative-control", test.ID, "runtime", "negative control is absent: "+control, "record the exact negative-control event")
			}
		}
	}
	return valid
}

func observationByID(values []ObservationLink, id string) (ObservationLink, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return ObservationLink{}, false
}

func (a *behaviorAdapter) inputForAnchor(anchor Anchor) (BehaviorAdapterInput, bool) {
	for _, input := range a.inputs {
		if input.Anchor.Revision == anchor.Revision && input.Anchor.Path == anchor.Path && input.Anchor.SHA256 == anchor.SHA256 {
			return input, true
		}
	}
	return BehaviorAdapterInput{}, false
}

func (a *behaviorAdapter) inputForPath(revision, path string) (BehaviorAdapterInput, bool) {
	for _, input := range a.inputs {
		if input.Anchor.Revision == revision && input.Anchor.Path == path {
			return input, true
		}
	}
	return BehaviorAdapterInput{}, false
}

func (a *behaviorAdapter) gap(originKey, kind, subject, field, detail, correction string) {
	origin, ok := a.origins[originKey]
	if !ok {
		origin = behaviorAdapterOrigin{Input: a.inputs[a.request.DiscoveryInput], Field: field, Revision: a.request.SourceRevision, Digest: a.inputs[a.request.DiscoveryInput].Anchor.SHA256}
	}
	if pointer, ok := a.mappings[strings.Split(originKey, ":")[0]+"s"].Fields[field]; ok {
		origin.Field = origin.Record + pointer
	}
	a.frontier = append(a.frontier, BehaviorAdapterDiagnostic{Kind: kind, Subject: subject, Input: origin.Input.ID, Field: origin.Field, Revision: origin.Revision, Digest: origin.Digest, Detail: detail, Correction: correction})
}

func (a *behaviorAdapter) gapFromInput(input BehaviorAdapterInput, kind, subject, field, detail, correction string) {
	if field == "" {
		field = "/"
	}
	a.frontier = append(a.frontier, BehaviorAdapterDiagnostic{Kind: kind, Subject: subject, Input: input.ID, Field: field, Revision: input.Anchor.Revision, Digest: input.Anchor.SHA256, Detail: detail, Correction: correction})
}

func (a *behaviorAdapter) artifacts(provider ProviderRecord) ([]BehaviorAdapterArtifact, error) {
	roles := map[string]string{a.request.MigrationInput: "migration", a.request.DiscoveryInput: "discovery"}
	for _, test := range provider.BehaviorContracts.Tests {
		if test.Runtime != nil {
			if input, ok := a.inputForAnchor(test.Runtime.Evidence); ok {
				roles[input.ID] = "runtime"
			}
		}
	}
	for _, observation := range provider.Observations {
		input, ok := a.inputForPath(observation.InputRevision, observation.Input)
		if !ok || Digest([]byte(input.Document)) != observation.RunID {
			return nil, fail("behavior adapter observation must name a retained receipt input with the run identity digest")
		}
		if role, taken := roles[input.ID]; taken && role != "receipt" {
			return nil, fail("behavior adapter observation receipt input already serves the " + role + " role")
		}
		roles[input.ID] = "receipt"
	}
	artifacts := make([]BehaviorAdapterArtifact, 0, len(roles))
	for id, role := range roles {
		input := a.inputs[id]
		artifacts = append(artifacts, BehaviorAdapterArtifact{Role: role, Input: id, Anchor: input.Anchor, SHA256: Digest([]byte(input.Document)), Document: input.Document})
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Role+"\x00"+artifacts[i].Input < artifacts[j].Role+"\x00"+artifacts[j].Input
	})
	return artifacts, nil
}

func behaviorAdapterCoverage(provider ProviderRecord, discovery BehaviorDiscovery, variations []BehaviorAdapterVariation, linkedReady, runtimeReady map[string]bool, limitations []string) []BehaviorAdapterCoverage {
	registry := provider.BehaviorContracts
	tests := map[string]BehaviorTest{}
	for _, test := range registry.Tests {
		tests[test.ID] = test
	}
	discoveredLinked := 0
	for _, execution := range discovery.Executions {
		if test, ok := tests[execution.ID]; ok && test.Project == execution.Project {
			discoveredLinked++
		}
	}
	candidateLinked := 0
	flows := map[string]bool{}
	for _, flow := range registry.Flows {
		flows[flow.ID] = true
	}
	for _, candidate := range registry.Behaviors {
		if len(candidate.Flows) > 0 && allStrings(candidate.Flows, flows) {
			candidateLinked++
		}
	}
	linked := 0
	runtime := 0
	for _, variation := range variations {
		if linkedReady[variation.ID] {
			linked++
		}
		if runtimeReady[variation.ID] {
			runtime++
		}
	}
	rows := []BehaviorAdapterCoverage{
		behaviorAdapterCoverageRow("documented_variations", len(variations), len(variations), registry.DocumentationRevision, "caller-reviewed normative variation inventory", limitations),
		behaviorAdapterCoverageRow("source_discovered_candidates", candidateLinked, len(registry.Behaviors), registry.SourceRevision, "proposed source candidates with exact documented-flow joins", limitations),
		behaviorAdapterCoverageRow("discovered_project_executions", discoveredLinked, len(discovery.Executions), registry.SourceRevision, "live discovery executions with exact test/project joins", limitations),
		behaviorAdapterCoverageRow("linked_contracts", linked, len(variations), registry.SourceRevision, "normative variations with identical semantic, assertion, project and reverse test joins", limitations),
		behaviorAdapterCoverageRow("runtime_witnessed_contracts", runtime, len(variations), registry.SourceRevision, "linked variations with structurally matching qualified receipt and ordered runtime witness for every allowed project", limitations),
	}
	return rows
}

func behaviorAdapterCoverageRow(metric string, value, denominator int, revision, rule string, limitations []string) BehaviorAdapterCoverage {
	state := "unreviewed"
	if denominator == 0 {
		state = "unknown"
	}
	return BehaviorAdapterCoverage{Metric: metric, Value: value, Denominator: denominator, Defined: denominator > 0, State: state, Revision: revision, Rule: rule, Limitations: slices.Clone(limitations)}
}

func allStrings(values []string, set map[string]bool) bool {
	for _, value := range values {
		if !set[value] {
			return false
		}
	}
	return true
}

func behaviorAdapterDelta(previous *BehaviorAdapterResult, current BehaviorAdapterResult) (BehaviorAdapterDelta, error) {
	if previous == nil {
		return BehaviorAdapterDelta{AddedCriteria: []string{}, RemovedCriteria: []string{}, ChangedCriteria: []string{}, LostReverseLinks: []string{}, PreviousAvailable: false}, nil
	}
	before, err := behaviorVariationDigests(previous.Variations)
	if err != nil {
		return BehaviorAdapterDelta{}, err
	}
	after, err := behaviorVariationDigests(current.Variations)
	if err != nil {
		return BehaviorAdapterDelta{}, err
	}
	delta := BehaviorAdapterDelta{PreviousAvailable: true, AddedCriteria: []string{}, RemovedCriteria: []string{}, ChangedCriteria: []string{}, LostReverseLinks: []string{}}
	for id, digest := range after {
		if old, ok := before[id]; !ok {
			delta.AddedCriteria = append(delta.AddedCriteria, id)
		} else if old != digest {
			delta.ChangedCriteria = append(delta.ChangedCriteria, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			delta.RemovedCriteria = append(delta.RemovedCriteria, id)
		}
	}
	beforeLinks, err := behaviorReverseLinks(*previous)
	if err != nil {
		return BehaviorAdapterDelta{}, err
	}
	afterLinks, err := behaviorReverseLinks(current)
	if err != nil {
		return BehaviorAdapterDelta{}, err
	}
	for link := range beforeLinks {
		if !afterLinks[link] {
			delta.LostReverseLinks = append(delta.LostReverseLinks, link)
		}
	}
	sort.Strings(delta.AddedCriteria)
	sort.Strings(delta.RemovedCriteria)
	sort.Strings(delta.ChangedCriteria)
	sort.Strings(delta.LostReverseLinks)
	return delta, nil
}

func behaviorVariationDigests(variations []BehaviorAdapterVariation) (map[string]string, error) {
	result := map[string]string{}
	for _, variation := range variations {
		digest, err := hashValue(variation)
		if err != nil {
			return nil, err
		}
		result[variation.ID] = digest
	}
	return result, nil
}

// reverseLinkJoin joins a reverse-link key's fields into one printable string.
// The corpus text rule (encoding.go textOK) forbids NUL in any published
// field, so this can no longer join fields with "\x00" as lost_reverse_links
// did before V1-0050 (fcdb12dc). A field may itself contain "|" or "\", so
// each field is backslash-escaped before joining on "|"; that keeps distinct
// field tuples from folding into the same joined string the way an
// unescaped separator could.
func reverseLinkJoin(parts ...string) string {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		part = strings.ReplaceAll(part, "\\", "\\\\")
		escaped[i] = strings.ReplaceAll(part, "|", "\\|")
	}
	return strings.Join(escaped, "|")
}

func behaviorReverseLinks(adapterResult BehaviorAdapterResult) (map[string]bool, error) {
	links := map[string]bool{}
	provider := adapterResult.Provider
	if provider.BehaviorContracts == nil {
		return links, nil
	}
	tests := map[string]BehaviorTest{}
	for _, test := range provider.BehaviorContracts.Tests {
		tests[test.ID] = test
		for _, assertion := range test.Assertions {
			digest, err := hashValue(assertion)
			if err != nil {
				return nil, err
			}
			links["assertion:"+reverseLinkJoin(test.ID, assertion.Criterion, assertion.ID, digest)] = true
		}
	}
	for _, flow := range provider.BehaviorContracts.Flows {
		for _, testID := range flow.Tests {
			test := tests[testID]
			if !slices.Contains(test.Flows, flow.ID) {
				continue
			}
			for _, criterion := range flow.Criteria {
				if slices.Contains(test.Criteria, criterion) {
					links["flow-test:"+reverseLinkJoin(flow.ID, criterion, testID, test.Project)] = true
				}
			}
		}
	}
	for _, variation := range adapterResult.Variations {
		links["variation-flow:"+reverseLinkJoin(variation.ID, variation.Flow)] = true
		for _, testID := range variation.Tests {
			links["variation-test:"+reverseLinkJoin(variation.ID, testID)] = true
		}
	}
	for _, test := range provider.BehaviorContracts.Tests {
		for _, criterion := range test.Criteria {
			links["test-variation:"+reverseLinkJoin(test.ID, criterion)] = true
		}
	}
	for _, record := range adapterResult.Claims {
		digest, err := hashValue(record.Claim)
		if err != nil {
			return nil, err
		}
		links["claim:"+reverseLinkJoin(record.TestID, record.Claim.VariationID, digest)] = true
	}
	return links, nil
}

func sortBehaviorAdapterResult(result *BehaviorAdapterResult) {
	sort.Slice(result.Frontier, func(i, j int) bool {
		left := result.Frontier[i]
		right := result.Frontier[j]
		return left.Kind+"\x00"+left.Subject+"\x00"+left.Input+"\x00"+left.Field+"\x00"+left.Detail < right.Kind+"\x00"+right.Subject+"\x00"+right.Input+"\x00"+right.Field+"\x00"+right.Detail
	})
}
