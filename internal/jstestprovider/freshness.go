package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tcq"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const FreshnessProfile = "corvint-playwright-freshness/0"

// FreshCommandIdentity records bytes read from a resolved local command. Its
// expectation is a requested target, never an execution observation.
type FreshCommandIdentity struct {
	Argv             []string `json:"argv"`
	ExecutableDigest string   `json:"executableDigest"`
	EntrypointDigest string   `json:"entrypointDigest"`
}

type FreshCommandBinding struct {
	Expected FreshCommandIdentity  `json:"expected"`
	Before   *FreshCommandIdentity `json:"before,omitempty"`
	After    *FreshCommandIdentity `json:"after,omitempty"`
}

type FreshProcessIdentity struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
}

// FreshServeObservation is produced by the independently pinned local observer.
// Body is the response it fetched, not a digest echoed from the request.
type FreshServeObservation struct {
	URL             string               `json:"url"`
	Process         FreshProcessIdentity `json:"process"`
	Body            []byte               `json:"body"`
	ServerImports   map[string]string    `json:"serverImports"`
	ObserverImports map[string]string    `json:"observerImports"`
}

type FreshnessBinding struct {
	Dependencies         *FreshDependencyManifest         `json:"dependencies"`
	DependencyDigest     string                           `json:"dependencyDigest"`
	DependenciesBefore   *FreshDependencyObservation      `json:"dependenciesBefore,omitempty"`
	DependenciesAfter    *FreshDependencyObservation      `json:"dependenciesAfter,omitempty"`
	Mutation             *ResponseMutationDefinition      `json:"mutation,omitempty"`
	ServerInputDigest    string                           `json:"serverInputDigest"`
	ProductExpected      ApplicationRepositoryIdentity    `json:"productExpected"`
	TestExpected         ApplicationRepositoryIdentity    `json:"testExpected"`
	ProductBefore        *ApplicationRepositoryIdentity   `json:"productBefore,omitempty"`
	ProductAfter         *ApplicationRepositoryIdentity   `json:"productAfter,omitempty"`
	ArtifactPath         string                           `json:"artifactPath"`
	Source               []byte                           `json:"source"`
	SourceDigest         string                           `json:"sourceDigest"`
	ArtifactBefore       string                           `json:"artifactBefore"`
	ArtifactAfter        string                           `json:"artifactAfter"`
	DocumentURL          string                           `json:"documentUrl"`
	Leader               *FreshProcessIdentity            `json:"leader,omitempty"`
	ServedBefore         *FreshServeObservation           `json:"servedBefore,omitempty"`
	ServedAfter          *FreshServeObservation           `json:"servedAfter,omitempty"`
	Runner               FreshCommandBinding              `json:"runner"`
	Server               FreshCommandBinding              `json:"server"`
	Observer             FreshCommandBinding              `json:"observer"`
	ObserverConfigBefore string                           `json:"observerConfigBefore"`
	ObserverConfigAfter  string                           `json:"observerConfigAfter"`
	EnvironmentBefore    map[string]string                `json:"environmentBefore"`
	EnvironmentAfter     map[string]string                `json:"environmentAfter"`
	IdentityAfter        *Identity                        `json:"identityAfter,omitempty"`
	ServerDescendants    *procgroup.DescendantObservation `json:"serverDescendants,omitempty"`
	ServerGone           *bool                            `json:"serverGone,omitempty"`
	Failures             []string                         `json:"failures"`
}

// FreshnessCurrency rederives currency from retained observations. It collects
// mismatches before missing facts, so an unavailable later observation cannot
// erase an already observed stale execution (PTF-V0-004).
func FreshnessCurrency(r Receipt, t TestOutcome) string {
	f := r.Freshness
	if r.Profile != FreshnessProfile || f == nil {
		return testvalidity.FreshnessUnknown
	}
	stale, missing := false, false
	if !freshManifestComplete(f.Dependencies) || f.DependencyDigest != FreshDependencyDigest(f.Dependencies) {
		missing = true
	}
	for _, observed := range []*FreshDependencyObservation{f.DependenciesBefore, f.DependenciesAfter} {
		if !FreshDependenciesMatch(f.Dependencies, observed) {
			missing = true
		}
		if observed != nil && f.Dependencies != nil {
			for path, digest := range observed.Files {
				if expected, ok := f.Dependencies.Files[path]; ok && expected != digest {
					stale = true
				}
			}
		}
	}
	compareImported := func(imports map[string]string) {
		if f.Dependencies != nil {
			for path, digest := range imports {
				if expected, ok := f.Dependencies.Files[path]; ok && digest != "" && expected != digest {
					stale = true
				}
			}
		}
	}
	compareImported(t.ImportedFiles)
	compareImported(r.Identity.ConfigInputDigests)
	if !freshImportsBound(t.ImportedFiles, func() string {
		if t.Anchor != nil {
			return t.Anchor.File
		}
		return ""
	}(), f.Dependencies) {
		missing = true
	}
	for path, digest := range r.Identity.ConfigInputDigests {
		if f.Dependencies == nil || f.Dependencies.Files[path] != digest {
			missing = true
		}
	}

	compareRepo := func(expected ApplicationRepositoryIdentity, actual *ApplicationRepositoryIdentity) {
		if !freshRepoComplete(expected) {
			missing = true
		}
		if actual == nil {
			missing = true
			return
		}
		if !freshRepoComplete(*actual) {
			missing = true
		}
		for _, pair := range [][2]string{{expected.RootCommit, actual.RootCommit}, {expected.Revision, actual.Revision}, {expected.Tree, actual.Tree}} {
			if pair[0] != "" && pair[1] != "" && pair[0] != pair[1] {
				stale = true
			}
		}
		if actual.DirtyDigest != "" || (actual.DirtyState != "" && actual.DirtyState != "clean") {
			stale = true
		}
	}
	compareRepo(f.ProductExpected, f.ProductBefore)
	compareRepo(f.ProductExpected, f.ProductAfter)
	compareRepo(f.TestExpected, r.TestRepositoryAtStart)
	compareRepo(f.TestExpected, r.TestRepositoryAtPublish)
	compareDigest := func(expected, actual string) {
		if !rawDigestPattern.MatchString(expected) || !rawDigestPattern.MatchString(actual) {
			missing = true
		}
		if expected != "" && actual != "" && expected != actual {
			stale = true
		}
	}
	if len(f.Source) == 0 || len(f.Source) > 64<<10 || f.ArtifactPath == "" || secretscreen.MatchString(string(f.Source)) {
		missing = true
	}
	compareDigest(f.SourceDigest, sha256Hex(f.Source))
	compareDigest(f.SourceDigest, f.ArtifactBefore)
	compareDigest(f.SourceDigest, f.ArtifactAfter)
	response := f.Source
	if f.Mutation != nil {
		var err error
		response, err = ResponseMutation(f.Source, f.ArtifactPath, *f.Mutation)
		if err != nil {
			missing = true
		}
	}
	serverInput, _ := json.Marshal(struct {
		Response []byte `json:"response"`
	}{response})
	compareDigest(sha256Hex(append(serverInput, '\n')), f.ServerInputDigest)
	if len(r.Identity.Argv) < len(f.Runner.Expected.Argv) || !reflect.DeepEqual(r.Identity.Argv[:min(len(r.Identity.Argv), len(f.Runner.Expected.Argv))], f.Runner.Expected.Argv) {
		missing = true
	}

	for _, binding := range []FreshCommandBinding{f.Runner, f.Server, f.Observer} {
		if len(binding.Expected.Argv) < 2 {
			missing = true
		}
		for _, actual := range []*FreshCommandIdentity{binding.Before, binding.After} {
			if actual == nil {
				missing = true
				continue
			}
			compareDigest(binding.Expected.ExecutableDigest, actual.ExecutableDigest)
			compareDigest(binding.Expected.EntrypointDigest, actual.EntrypointDigest)
			if !reflect.DeepEqual(binding.Expected.Argv, actual.Argv) {
				stale = true
			}
		}
	}
	compareDigest(f.ObserverConfigBefore, f.ObserverConfigAfter)
	if !freshEnvironment(f.EnvironmentBefore) || !freshEnvironment(f.EnvironmentAfter) {
		missing = true
	}
	if f.EnvironmentBefore != nil && f.EnvironmentAfter != nil && !reflect.DeepEqual(f.EnvironmentBefore, f.EnvironmentAfter) {
		stale = true
	}
	if !reflect.DeepEqual(r.Identity.Environment, f.EnvironmentBefore) {
		missing = true
	}
	if f.IdentityAfter == nil {
		missing = true
	} else if !reflect.DeepEqual(r.Identity, *f.IdentityAfter) {
		stale = true
	}
	if f.Leader == nil || f.Leader.PID <= 0 || f.Leader.Start == "" || f.DocumentURL == "" {
		missing = true
	}
	for _, observed := range []*FreshServeObservation{f.ServedBefore, f.ServedAfter} {
		if observed == nil {
			missing = true
			continue
		}
		compareImported(observed.ServerImports)
		compareImported(observed.ObserverImports)
		if len(f.Server.Expected.Argv) < 2 || len(f.Observer.Expected.Argv) < 2 || !freshImportsBound(observed.ServerImports, f.Server.Expected.Argv[1], f.Dependencies) || !freshImportsBound(observed.ObserverImports, f.Observer.Expected.Argv[1], f.Dependencies) {
			missing = true
		}
		if observed.Process.PID <= 0 || observed.Process.Start == "" || observed.URL == "" || len(observed.Body) == 0 || len(observed.Body) > 64<<10 {
			missing = true
		}
		if f.Leader != nil && observed.Process.PID > 0 && (observed.Process.PID != f.Leader.PID || observed.Process.Start != f.Leader.Start) {
			stale = true
		}
		if observed.URL != "" && observed.URL != f.DocumentURL {
			stale = true
		}
		servedDigest := f.SourceDigest
		if f.Mutation != nil {
			mutant, e := ResponseMutation(f.Source, f.ArtifactPath, *f.Mutation)
			if e != nil {
				missing = true
			} else {
				servedDigest = sha256Hex(mutant)
			}
		}
		compareDigest(servedDigest, sha256Hex(observed.Body))
		if secretscreen.MatchString(string(observed.Body)) {
			missing = true
		}
	}

	if r.ApplicationAttestation != nil {
		for _, observation := range []*ApplicationAttestationObservation{r.ApplicationAttestation.Before, r.ApplicationAttestation.After} {
			if observation != nil {
				compareRepo(f.ProductExpected, &observation.Attestation.Repository)
				compareDigest(f.SourceDigest, strings.TrimPrefix(observation.Attestation.Build.Digest, "sha256:"))
				compareDigest(r.Identity.ConfigDigest, strings.TrimPrefix(observation.Attestation.Configuration.Digest, "sha256:"))
				if f.Leader != nil && (observation.Attestation.Instance.ID != strconv.Itoa(f.Leader.PID) || observation.Attestation.Instance.StartGeneration != f.Leader.Start) {
					stale = true
				}
			}
		}
		for _, failure := range r.ApplicationAttestation.Failures {
			if strings.Contains(failure, "drift") || strings.Contains(failure, "mismatch") {
				stale = true
			}
		}
	}
	if stale {
		return testvalidity.FreshnessStale
	}
	if missing || len(f.Failures) != 0 || r.Infrastructure != nil || r.Cancelled || f.ServerGone == nil || !*f.ServerGone || !freshDescendantsAbsent(f.ServerDescendants) || !freshDescendantsAbsent(r.DescendantObservation) || freshRowUnknown(r, t) {
		return testvalidity.FreshnessUnknown
	}
	return testvalidity.FreshnessCurrent
}

func freshRepoComplete(r ApplicationRepositoryIdentity) bool {
	return gitOIDPattern.MatchString(r.RootCommit) && gitOIDPattern.MatchString(r.Revision) && gitOIDPattern.MatchString(r.Tree) && r.DirtyState == "clean" && r.DirtyDigest == ""
}

func freshEnvironment(env map[string]string) bool {
	if len(env) != 4 {
		return false
	}
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "TMPDIR"} {
		if _, ok := env[key]; !ok {
			return false
		}
	}
	return true
}

func freshRowUnknown(r Receipt, t TestOutcome) bool {
	if r.Freshness == nil {
		return true
	}
	x := r.External
	if x == nil || x.Ownership != "external" || x.CleanupResponsibility != "external" || x.ServerDescendants != "unknown" || !x.ReadyAtStart || !x.ReadyAtPublish || !x.InputsUnchanged || !x.RunnerDescendantsGone || x.ConfigOverride == "" || x.DeclaredAppIdentity != "" || r.ServerDescendantsGone != nil || applicationAttestationUnknown(r.ApplicationAttestation) {
		return true
	}
	if r.Identity.RunnerName != "playwright" || r.Identity.RunnerVersion != "1.63.0" || r.Identity.NodeVersion != "v22.23.3" || !rawDigestPattern.MatchString(r.Identity.PackageDigest) || !rawDigestPattern.MatchString(r.Identity.ConfigDigest) || r.Identity.ConfigInputDigests[r.Identity.ConfigFile] != r.Identity.ConfigDigest || len(r.Identity.Argv) == 0 || len(r.Identity.ConfigInputDigests) > externalMaxConfigInputs {
		return true
	}
	if t.Project == nil || t.Project.Name == "" || t.Project.Browser != "chromium" || t.Project.Device == "" || t.Project.ConfigDigest != r.Identity.ConfigDigest || t.Anchor == nil || t.Anchor.Line <= 0 || t.FullName == "" || !rawDigestPattern.MatchString(r.Identity.TestFileDigests[t.Anchor.File]) || t.ID != qualifiedTestID(r.Identity, t) || len(t.Attempts) != 1 || t.Retries != 0 || t.Attempts[0].Retry != 0 || t.Attempts[0].State != t.State || !knownState(t.State) || hasAttemptDetails(r) || hasSensitiveInputEvidence(r) {
		return true
	}
	var use playwrightUseIdentity
	if json.Unmarshal(t.Project.Use, &use) != nil || use.BaseURL == nil || *use.BaseURL != r.Freshness.DocumentURL || use.BrowserName == nil || *use.BrowserName != "chromium" || use.Channel == nil || *use.Channel != "" || len(use.ConnectOptions) != 0 {
		return true
	}
	b := use.CorvintBrowser
	if b.NodeVersion == nil || *b.NodeVersion != "v22.23.3" || b.Platform == nil || *b.Platform != "darwin" || b.Arch == nil || *b.Arch != "arm64" || b.BrowserType == nil || *b.BrowserType != "chromium" || b.Channel == nil || *b.Channel != "" || b.ExecutablePath == nil || b.BrowserVersion == nil || b.HeadlessShellAvailable == nil || !*b.HeadlessShellAvailable || b.ExecutableSource == nil {
		return true
	}
	return !qualifiedBundledPlaywrightBrowser(use.Headless, use.LaunchOptions.ExecutablePath, b)
}

func freshTestProjection(r Receipt, t TestOutcome) testvalidity.Projection {
	base := ToTestProjection(t)
	claim := testvalidity.ClaimFacts{AssociationState: base.Association.State, HygieneState: base.Hygiene.State, ReportState: reportStateFor(t.State), Anchors: anchorStrings(t.Anchor)}
	if flakyOutcome(t) {
		claim.Reasons = []string{"flaky-retry"}
	}
	e := testvalidity.ExecutionFacts{Currency: FreshnessCurrency(r, t), Anchors: anchorStrings(t.Anchor)}
	if r.Cancelled {
		e.Outcome = "INCOMPLETE"
		e.Cause = "CANCELLATION"
	} else if r.Infrastructure != nil || freshRowUnknown(r, t) {
		e.Outcome = "INCOMPLETE"
		e.Cause = "INFRASTRUCTURE"
	}
	// An absent source anchor affects association only, independently of currency.
	if t.Anchor == nil {
		claim.AssociationState = tcq.AssociationAbstained
	}
	return testvalidity.Project(testvalidity.Input{Claim: &claim, Execution: &e})
}

type freshnessDocument struct {
	Receipt     Receipt                   `json:"receipt"`
	Projections []testvalidity.Projection `json:"testProjections"`
	Run         testvalidity.Projection   `json:"runProjection"`
}

// EncodeFreshness and DecodeFreshness form a separate closed canonical codec.
// Legacy retention remains unsupported for this profile (PTF-V0-005).
func EncodeFreshness(r Receipt) ([]byte, error) {
	if r.Profile != FreshnessProfile || r.Freshness == nil || len(r.Tests) > 10000 || hasAttemptDetails(r) || hasSensitiveInputEvidence(r) {
		return nil, errors.New("freshness-profile-shape")
	}
	if r.Freshness.Mutation != nil {
		if _, err := ResponseMutation(r.Freshness.Source, r.Freshness.ArtifactPath, *r.Freshness.Mutation); err != nil {
			return nil, err
		}
	}
	d := freshnessDocument{Receipt: r, Run: ReceiptRunProjection(r), Projections: make([]testvalidity.Projection, 0, len(r.Tests))}
	for _, t := range r.Tests {
		d.Projections = append(d.Projections, ReceiptTestProjection(r, t))
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	if len(b) >= externalOutputLimit || secretscreen.MatchString(string(b)) {
		return nil, errors.New("freshness-output-bound-or-secret")
	}
	for _, observed := range []*FreshServeObservation{r.Freshness.ServedBefore, r.Freshness.ServedAfter} {
		if observed != nil && secretscreen.MatchString(string(observed.Body)) {
			return nil, errors.New("freshness-source-secret")
		}
	}
	if secretscreen.MatchString(string(r.Freshness.Source)) {
		return nil, errors.New("freshness-source-secret")
	}
	return append(b, '\n'), nil
}

func DecodeFreshness(data []byte) (Receipt, error) {
	if len(data) >= externalOutputLimit {
		return Receipt{}, errors.New("freshness-output-bound")
	}
	var d freshnessDocument
	if err := decodeFreshCanonical(data, &d, externalOutputLimit); err != nil {
		return Receipt{}, err
	}
	encoded, err := EncodeFreshness(d.Receipt)
	if err != nil {
		return Receipt{}, err
	}
	if !reflect.DeepEqual(d.Projections, freshnessProjections(d.Receipt)) || string(encoded) != string(data) {
		return Receipt{}, errors.New("freshness-projection-or-canonical-drift")
	}
	for _, t := range d.Receipt.Tests {
		if t.ID != "" && t.ID != qualifiedTestID(d.Receipt.Identity, t) {
			return Receipt{}, errors.New("freshness-native-identity-drift")
		}
	}
	return d.Receipt, nil
}

func decodeFreshCanonical(data []byte, out any, limit int) error {
	if len(data) == 0 || len(data) >= limit || secretscreen.MatchString(string(data)) {
		return errors.New("freshness-input-bound-or-secret")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("freshness-trailing-data")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if !bytes.Equal(append(b, '\n'), data) {
		return errors.New("freshness-noncanonical-input")
	}
	return nil
}

func freshnessProjections(r Receipt) []testvalidity.Projection {
	out := make([]testvalidity.Projection, 0, len(r.Tests))
	for _, t := range r.Tests {
		out = append(out, ReceiptTestProjection(r, t))
	}
	return out
}

func freshProcessLabel(p FreshProcessIdentity) string { return strconv.Itoa(p.PID) + ":" + p.Start }

func freshDescendantsAbsent(d *procgroup.DescendantObservation) bool {
	return d != nil && d.Absent && len(d.Failures) == 0 && d.IntervalMS == 20 && d.Scope != "" && len(d.Limitations) != 0
}
