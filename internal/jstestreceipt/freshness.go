package jstestreceipt

import (
	"encoding/json"
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
	if !FreshManifestComplete(f.Dependencies) || f.DependencyDigest != FreshDependencyDigest(f.Dependencies) {
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
		if !FreshRepoComplete(expected) {
			missing = true
		}
		if actual == nil {
			missing = true
			return
		}
		if !FreshRepoComplete(*actual) {
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
		if !RawDigestPattern.MatchString(expected) || !RawDigestPattern.MatchString(actual) {
			missing = true
		}
		if expected != "" && actual != "" && expected != actual {
			stale = true
		}
	}
	if len(f.Source) == 0 || len(f.Source) > 64<<10 || f.ArtifactPath == "" || secretscreen.MatchString(string(f.Source)) {
		missing = true
	}
	compareDigest(f.SourceDigest, Sha256Hex(f.Source))
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
	compareDigest(Sha256Hex(append(serverInput, '\n')), f.ServerInputDigest)
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
	if !FreshEnvironment(f.EnvironmentBefore) || !FreshEnvironment(f.EnvironmentAfter) {
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
				servedDigest = Sha256Hex(mutant)
			}
		}
		compareDigest(servedDigest, Sha256Hex(observed.Body))
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
	if missing || len(f.Failures) != 0 || r.Infrastructure != nil || r.Cancelled || f.ServerGone == nil || !*f.ServerGone || !FreshDescendantsAbsent(f.ServerDescendants) || !FreshDescendantsAbsent(r.DescendantObservation) || freshRowUnknown(r, t) {
		return testvalidity.FreshnessUnknown
	}
	return testvalidity.FreshnessCurrent
}

func FreshRepoComplete(r ApplicationRepositoryIdentity) bool {
	return GitOIDPattern.MatchString(r.RootCommit) && GitOIDPattern.MatchString(r.Revision) && GitOIDPattern.MatchString(r.Tree) && r.DirtyState == "clean" && r.DirtyDigest == ""
}

func FreshEnvironment(env map[string]string) bool {
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
	if x == nil || x.Ownership != "external" || x.CleanupResponsibility != "external" || x.ServerDescendants != "unknown" || !x.ReadyAtStart || !x.ReadyAtPublish || !x.InputsUnchanged || !x.RunnerDescendantsGone || x.ConfigOverride == "" || x.DeclaredAppIdentity != "" || r.ServerDescendantsGone != nil || ApplicationAttestationUnknown(r.ApplicationAttestation) {
		return true
	}
	if r.Identity.RunnerName != "playwright" || !RuntimeTupleAdmitted(FreshnessProfile, r.Identity.RunnerVersion, r.Identity.NodeVersion, BrowserSourceBundled) || !RawDigestPattern.MatchString(r.Identity.PackageDigest) || !RawDigestPattern.MatchString(r.Identity.ConfigDigest) || r.Identity.ConfigInputDigests[r.Identity.ConfigFile] != r.Identity.ConfigDigest || len(r.Identity.Argv) == 0 || ConfigInputsWithinBounds(r.Identity.ConfigInputDigests) != nil {
		return true
	}
	if t.Project == nil || t.Project.Name == "" || t.Project.Browser != "chromium" || t.Project.Device == "" || t.Project.ConfigDigest != r.Identity.ConfigDigest || t.Anchor == nil || t.Anchor.Line <= 0 || t.FullName == "" || !RawDigestPattern.MatchString(r.Identity.TestFileDigests[t.Anchor.File]) || t.ID != QualifiedTestID(r.Identity, t) || len(t.Attempts) != 1 || t.Retries != 0 || t.Attempts[0].Retry != 0 || t.Attempts[0].State != t.State || !KnownState(t.State) || HasAttemptDetails(r) || HasSensitiveInputEvidence(r) {
		return true
	}
	var use PlaywrightUseIdentity
	if json.Unmarshal(t.Project.Use, &use) != nil || use.BaseURL == nil || *use.BaseURL != r.Freshness.DocumentURL || use.BrowserName == nil || *use.BrowserName != "chromium" || use.Channel == nil || *use.Channel != "" || len(use.ConnectOptions) != 0 {
		return true
	}
	b := use.CorvintBrowser
	if b.NodeVersion == nil || *b.NodeVersion != r.Identity.NodeVersion || b.Platform == nil || *b.Platform != "darwin" || b.Arch == nil || *b.Arch != "arm64" || b.BrowserType == nil || *b.BrowserType != "chromium" || b.Channel == nil || *b.Channel != "" || b.ExecutablePath == nil || b.BrowserVersion == nil || b.HeadlessShellAvailable == nil || !*b.HeadlessShellAvailable || b.ExecutableSource == nil {
		return true
	}
	return !qualifiedBundledPlaywrightBrowser(r.Identity.RunnerVersion, use.Headless, use.LaunchOptions.ExecutablePath, b)
}

func freshTestProjection(r Receipt, t TestOutcome) testvalidity.Projection {
	base := ToTestProjection(t)
	claim := testvalidity.ClaimFacts{AssociationState: base.Association.State, HygieneState: base.Hygiene.State, ReportState: reportStateFor(t.State), Anchors: anchorStrings(t.Anchor)}
	if FlakyOutcome(t) {
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

func FreshDescendantsAbsent(d *procgroup.DescendantObservation) bool {
	return d != nil && d.Absent && len(d.Failures) == 0 && d.IntervalMS == 20 && d.Scope != "" && len(d.Limitations) != 0
}
