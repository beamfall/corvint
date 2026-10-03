package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// FreshnessConfig selects the optional trusted source-direct profile. All
// commands require independently approved byte pins; arbitrary providers and
// compiled source provenance remain outside its qualification boundary.
type FreshnessConfig struct {
	Dependencies         *FreshDependencyManifest      `json:"dependencies"`
	DependencyDigest     string                        `json:"dependencyDigest"`
	Mutation             *ResponseMutationDefinition   `json:"mutation,omitempty"`
	ProductDir           string                        `json:"productDir"`
	ProductExpected      ApplicationRepositoryIdentity `json:"productExpected"`
	TestExpected         ApplicationRepositoryIdentity `json:"testExpected"`
	ArtifactPath         string                        `json:"artifactPath"`
	SourceDigest         string                        `json:"sourceDigest"`
	DocumentURL          string                        `json:"documentUrl"`
	Runner               FreshCommandIdentity          `json:"runner"`
	Server               FreshCommandIdentity          `json:"server"`
	Observer             FreshCommandIdentity          `json:"observer"`
	ObserverConfigFile   string                        `json:"observerConfigFile"`
	ObserverConfigDigest string                        `json:"observerConfigDigest"`
}

func freshCommandObserve(expected FreshCommandIdentity) *FreshCommandIdentity {
	if len(expected.Argv) < 2 || !filepath.IsAbs(expected.Argv[0]) || !filepath.IsAbs(expected.Argv[1]) {
		return nil
	}
	executable, err := readBoundedRegularFile(expected.Argv[0], applicationAttestationExecutableLimit)
	if err != nil {
		return nil
	}
	entry, err := readBoundedRegularFile(expected.Argv[1], 4<<20)
	if err != nil {
		return nil
	}
	return &FreshCommandIdentity{Argv: append([]string{}, expected.Argv...), ExecutableDigest: sha256Hex(executable), EntrypointDigest: sha256Hex(entry)}
}

func freshProcessObserve(ctx context.Context, pid int) (*FreshProcessIdentity, error) {
	if pid <= 0 {
		return nil, errors.New("freshness-leader-unavailable")
	}
	// lstart is the existing host's kernel process-start observation. It is
	// sampled, second-granularity evidence, not hostile PID-reuse containment.
	command := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	data, err := command.Output()
	start := strings.TrimSpace(string(data))
	if err != nil || len(data) > 256 || start == "" {
		return nil, errors.New("freshness-leader-unavailable")
	}
	return &FreshProcessIdentity{PID: pid, Start: start}, nil
}

func freshLoopback(address string) bool {
	u, err := url.Parse(address)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.User == nil && u.Fragment == "" && u.RawQuery == "" && len(address) <= 4096
}

func freshServeObserve(ctx context.Context, c FreshnessConfig, dir string, environment []string) (*FreshServeObservation, string) {
	config, err := readBoundedRegularFile(c.ObserverConfigFile, applicationAttestationLimit)
	if err != nil || sha256Hex(config) != c.ObserverConfigDigest {
		return nil, "freshness-observer-config-drift"
	}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: c.Observer.Argv, Dir: dir, Env: environment, Stdin: config, InputLimit: applicationAttestationLimit, OutputLimit: 128 << 10, Timeout: 5 * time.Second, ObserveDescendants: true})
	if !o.Started || !o.WaitCompleted || !o.ExitObserved || o.ExitStatus != 0 || o.Cancelled || o.TimedOut || o.OutputOverflow || !o.OwnedProcessGroupCleanup || !freshDescendantsAbsent(o.DescendantObservation) {
		return nil, "freshness-observer-unavailable"
	}
	var observed FreshServeObservation
	// Body may base64-expand the 64 KiB artifact beyond the application's codec
	// limit. Use the separately bounded strict freshness decoder framing here.
	if err := decodeFreshCanonical(o.Stdout, &observed, 128<<10); err != nil || len(observed.Body) == 0 || len(observed.Body) > 64<<10 || secretscreen.MatchString(string(observed.Body)) {
		return nil, "freshness-observer-output-invalid"
	}
	return &observed, ""
}

// RunFreshE2E owns and joins the server on every return path, while preserving
// the external /1 runner's lifecycle facts. No external server is claimed by
// the reused observer. Only the separately captured leader is owned here.
func RunFreshE2E(ctx context.Context, cfg E2EConfig) (r Receipt, err error) {
	c := cfg.Freshness
	if c == nil || cfg.ExternalServer || cfg.ApplicationAttestation == nil || cfg.SensitiveInputPolicy != nil || cfg.RetainAttemptDetails || !freshRepoComplete(c.ProductExpected) || !freshRepoComplete(c.TestExpected) || !freshLoopback(c.DocumentURL) || !freshLoopback(cfg.ServerReadyURL) || !filepath.IsAbs(c.ProductDir) || filepath.IsAbs(c.ArtifactPath) || filepath.Clean(c.ArtifactPath) != c.ArtifactPath || c.ArtifactPath == "." || strings.HasPrefix(c.ArtifactPath, "../") || !rawDigestPattern.MatchString(c.SourceDigest) || !rawDigestPattern.MatchString(c.ObserverConfigDigest) || !reflect.DeepEqual(cfg.ServerArgv, c.Server.Argv) || cfg.RunnerVersion != "1.63.0" || cfg.Timeout <= 0 || cfg.Timeout > 60*time.Second {
		return Receipt{}, errors.New("freshness-config-unsupported")
	}
	environment := declaredEnv([]string{"PATH", "LANG", "LC_ALL", "TMPDIR"})
	if len(os.Environ()) != 4 || !freshEnvironment(environment) {
		return Receipt{}, errors.New("freshness-environment-unsupported")
	}
	if len(cfg.DeclaredEnvKeys) != 4 {
		return Receipt{}, errors.New("freshness-declared-environment-required")
	}
	before := &FreshnessBinding{Dependencies: c.Dependencies, DependencyDigest: c.DependencyDigest, Mutation: c.Mutation, ProductExpected: c.ProductExpected, TestExpected: c.TestExpected, ArtifactPath: c.ArtifactPath, SourceDigest: c.SourceDigest, DocumentURL: c.DocumentURL, Runner: FreshCommandBinding{Expected: c.Runner}, Server: FreshCommandBinding{Expected: c.Server}, Observer: FreshCommandBinding{Expected: c.Observer}, EnvironmentBefore: environment, Failures: []string{}}
	r = Receipt{Profile: FreshnessProfile, Kind: "e2e", Freshness: before, Tests: []TestOutcome{}}
	fail := func(reason string) { before.Failures = append(before.Failures, reason) }
	// Every return retains a publication observation, including early failures.
	before.DependenciesBefore = ObserveFreshDependencies(c.Dependencies)
	defer func() { before.DependenciesAfter = ObserveFreshDependencies(c.Dependencies) }()
	if !freshManifestComplete(c.Dependencies) || FreshDependencyDigest(c.Dependencies) != c.DependencyDigest || !FreshDependenciesMatch(c.Dependencies, before.DependenciesBefore) {
		fail("freshness-dependency-closure-unknown")
		return r, nil
	}

	before.ProductBefore, _ = observeTestRepository(ctx, c.ProductDir)
	r.TestRepositoryAtStart, _ = observeTestRepository(ctx, cfg.Dir)
	artifact := filepath.Join(c.ProductDir, c.ArtifactPath)
	before.Source, err = readBoundedRegularFile(artifact, 64<<10)
	if err != nil || secretscreen.MatchString(string(before.Source)) {
		fail("freshness-source-unavailable")
		return r, nil
	}
	before.ArtifactBefore = sha256Hex(before.Source)
	before.Runner.Before = freshCommandObserve(c.Runner)
	before.Server.Before = freshCommandObserve(c.Server)
	before.Observer.Before = freshCommandObserve(c.Observer)
	configRaw, configErr := readBoundedRegularFile(c.ObserverConfigFile, applicationAttestationLimit)
	if configErr == nil {
		before.ObserverConfigBefore = sha256Hex(configRaw)
	}
	if before.ArtifactBefore != c.SourceDigest || before.ObserverConfigBefore != c.ObserverConfigDigest || before.ProductBefore == nil || r.TestRepositoryAtStart == nil {
		fail("freshness-initial-identity-unavailable")
		return r, nil
	}
	for _, b := range []FreshCommandBinding{before.Runner, before.Server, before.Observer} {
		if b.Before == nil || !reflect.DeepEqual(b.Expected, *b.Before) {
			fail("freshness-command-unavailable")
			return r, nil
		}
	}
	tracked, failure := gitObservation(ctx, c.ProductDir, "ls-files", "--error-unmatch", "--", c.ArtifactPath)
	if failure != "" || strings.TrimSpace(string(tracked)) != c.ArtifactPath {
		fail("freshness-source-untracked")
		return r, nil
	}
	response := before.Source
	if c.Mutation != nil {
		response, err = ResponseMutation(before.Source, c.ArtifactPath, *c.Mutation)
		if err != nil {
			return r, err
		}
	}
	serverInput, _ := json.Marshal(struct {
		Response []byte `json:"response"`
	}{response})
	serverInput = append(serverInput, '\n')
	before.ServerInputDigest = sha256Hex(serverInput)
	serverCtx, stop := context.WithCancel(ctx)
	defer stop()
	started := make(chan *FreshProcessIdentity, 1)
	done := make(chan procgroup.Observation, 1)
	go func() {
		done <- procgroup.Run(serverCtx, procgroup.Spec{Argv: c.Server.Argv, Dir: c.ProductDir, Env: os.Environ(), Stdin: serverInput, InputLimit: 128 << 10, Timeout: cfg.Timeout, OutputLimit: 64 << 10, ObserveDescendants: true, AfterStart: func(hookCtx context.Context, pid int) error {
			p, e := freshProcessObserve(hookCtx, pid)
			started <- p
			return e
		}})
	}()
	// Joining is unconditional; readiness and attestation errors cannot strand
	// an owned server or its descendant observer (PTF-V0-009).
	defer func() {
		stop()
		o := <-done
		gone := o.OwnedProcessGroupCleanup && o.DescendantObservation != nil && o.DescendantObservation.Absent && len(o.DescendantObservation.Failures) == 0
		before.ServerGone = &gone
		before.ServerDescendants = o.DescendantObservation
		before.EnvironmentAfter = declaredEnv([]string{"PATH", "LANG", "LC_ALL", "TMPDIR"})
		before.Runner.After = freshCommandObserve(c.Runner)
		before.Server.After = freshCommandObserve(c.Server)
		before.Observer.After = freshCommandObserve(c.Observer)
		if b, e := readBoundedRegularFile(c.ObserverConfigFile, applicationAttestationLimit); e == nil {
			before.ObserverConfigAfter = sha256Hex(b)
		}
		if b, e := readBoundedRegularFile(artifact, 64<<10); e == nil {
			before.ArtifactAfter = sha256Hex(b)
		}
		post, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		before.ProductAfter, _ = observeTestRepository(post, c.ProductDir)
		if r.TestRepositoryAtPublish == nil {
			r.TestRepositoryAtPublish, _ = observeTestRepository(post, cfg.Dir)
		}
		r.Cancelled = r.Cancelled || ctx.Err() != nil
	}()
	select {
	case before.Leader = <-started:
	case <-ctx.Done():
		fail("freshness-server-cancelled")
		return r, nil
	case <-time.After(2 * time.Second):
		fail("freshness-server-start-unavailable")
		return r, nil
	}
	if before.Leader == nil {
		fail("freshness-leader-unavailable")
		return r, nil
	}
	if e := externalReady(ctx, cfg.ServerReadyURL, 5*time.Second); e != nil {
		fail("freshness-readiness-unavailable")
		return r, nil
	}
	var reason string
	before.ServedBefore, reason = freshServeObserve(ctx, *c, c.ProductDir, os.Environ())
	if reason != "" {
		fail(reason)
		return r, nil
	}
	// Preserve the observed early mismatch even when the runner never starts.
	if before.ServedBefore.Process != *before.Leader || before.ServedBefore.URL != c.DocumentURL || sha256Hex(before.ServedBefore.Body) != sha256Hex(response) {
		fail("freshness-served-identity-mismatch")
		return r, nil
	}
	external := cfg
	external.ExternalServer = true
	external.ServerArgv = nil
	external.ObserveDescendants = true
	r, err = runExternal(ctx, external)
	r.Freshness = before
	r.Profile = FreshnessProfile
	post, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before.ServedAfter, reason = freshServeObserve(post, *c, c.ProductDir, os.Environ())
	if reason != "" {
		fail(reason)
	}
	after, e := cfg.identity(r.Identity.Argv)
	if e == nil {
		after.ConfigInputDigests = map[string]string{}
		for path := range r.Identity.ConfigInputDigests {
			if d, e := digestFile(path); e == nil {
				after.ConfigInputDigests[path] = d
			}
		}
		before.IdentityAfter = &after
	}
	return r, err
}
