package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/stepnegation"
)

// RefusalSchema names the provider's typed negate refusal (LPCV-V0-065).
const RefusalSchema = "corvint-step-negation-refusal/0"

// errRefused marks a refusal already written to stdout.
var errRefused = errors.New("negate refused")

// negateRefusal is the closed refusal document: a typed code and a message.
type negateRefusal struct {
	Schema  string `json:"schema"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// negateRunner is replaced in tests.
var negateRunner = jstestprovider.RunNegate

// runNegate implements `corvint-js-test-provider negate` (LPCV-V0-057): it
// runs the step-level negative controls of one test against an external
// application and writes the corvint-step-negation/0 document to stdout.
// A refusal writes one corvint-step-negation-refusal/0 document instead and
// exits 2. It never retains; the core command owns retention (LPCV-V0-067).
func runNegate(ctx context.Context, args []string, stdout io.Writer) error {
	cfg, err := parseNegate(args)
	if err == nil {
		var document stepnegation.Document
		document, err = negateRunner(ctx, cfg)
		if err == nil {
			data, encodeErr := stepnegation.Encode(document)
			if encodeErr != nil {
				return encodeErr
			}
			_, err = stdout.Write(data)
			return err
		}
	}
	var refusal *jstestprovider.NegateRefusal
	if !errors.As(err, &refusal) {
		return err
	}
	data, _ := json.Marshal(negateRefusal{Schema: RefusalSchema, Code: refusal.Code, Message: refusal.Message})
	_, _ = stdout.Write(append(data, '\n'))
	return errRefused
}

// unsupportedNegateFlags name modes negate does not compose with
// (LPCV-V0-058); they refuse negate-mode-unsupported before any run.
var unsupportedNegateFlags = map[string]bool{"sensitive-input-redaction": true, "sensitive-action-pattern": true, "sensitive-field": true, "retain-attempt-details": true, "keep-reporters": true, "server-arg": true, "test-arg": true, "watch": true, "watch-path": true, "retain": true}

func parseNegate(args []string) (jstestprovider.NegateConfig, error) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if strings.HasPrefix(arg, "-") && unsupportedNegateFlags[name] {
			return jstestprovider.NegateConfig{}, &jstestprovider.NegateRefusal{Code: jstestprovider.NegateModeUnsupported, Message: "--" + name + " is not composed with negate"}
		}
	}
	fs := flag.NewFlagSet("negate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "test repository worktree root")
	spec := fs.String("spec", "", "worktree-relative spec file")
	test := fs.String("test", "", "exact full test title (describe titles and test title joined by \" > \")")
	project := fs.String("project", "", "Playwright project")
	step := fs.String("step", "", "exact test.step title to control")
	allSteps := fs.Bool("all-steps", false, "control every step in one joint run")
	maxRuns := fs.Int("max-runs", 0, "cap on runs, baselines included (default 3 for --step, 2 for --all-steps)")
	baselineRepeat := fs.Int("baseline-repeat", 1, "unfaulted baseline runs, 1 to 5")
	dir := fs.String("dir", "", "working directory of the Playwright command (default --root)")
	configFile := fs.String("config", "", "playwright config file path")
	packageJSON := fs.String("package-json", "", "package.json path")
	lockfile := fs.String("lockfile", "", "lockfile path")
	runnerVersion := fs.String("runner-version", "", "pinned @playwright/test version")
	appBuildDir := fs.String("app-build-dir", "", "served app build directory")
	externalServer := fs.Bool("external-server", true, "negate runs only against an externally managed server")
	appIdentity := fs.String("app-identity", "", "declared external application identity (/0)")
	appAttestationCommand := fs.String("app-attestation-command", "", "JSON argv for the typed application-attestation provider (/1)")
	appAttestationConfig := fs.String("app-attestation-config", "", "canonical application-attestation expectation config (/1)")
	appAttestationTimeout := fs.Duration("app-attestation-timeout", 5*time.Second, "bound on each application-attestation observation")
	serverReadyURL := fs.String("server-ready-url", "", "URL polled until it answers with status < 500")
	serverReadyTimeout := fs.Duration("server-ready-timeout", 15*time.Second, "bound on waiting for server readiness")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on each playwright run")
	var testFiles, envKeys stringList
	fs.Var(&testFiles, "test-file", "an additional test file to bind identity to (repeatable)")
	fs.Var(&envKeys, "env-key", "a declared environment variable name to observe (repeatable)")
	invalid := func(err error) error {
		return &jstestprovider.NegateRefusal{Code: jstestprovider.NegateInvalidArguments, Message: err.Error()}
	}
	if err := fs.Parse(args); err != nil {
		return jstestprovider.NegateConfig{}, invalid(err)
	}
	if fs.NArg() != 0 {
		return jstestprovider.NegateConfig{}, invalid(errors.New("negate takes no positional arguments"))
	}
	if !*externalServer {
		return jstestprovider.NegateConfig{}, &jstestprovider.NegateRefusal{Code: jstestprovider.NegateModeUnsupported, Message: "owned-server mode is not composed with negate"}
	}
	resolve := func(path string) (string, error) {
		resolved, err := resolvePath(path)
		if err != nil || resolved == "" {
			return resolved, err
		}
		if canonical, err := filepath.EvalSymlinks(resolved); err == nil {
			return canonical, nil
		}
		return resolved, nil
	}
	resolvedRoot, err := resolve(*root)
	if err != nil || resolvedRoot == "" {
		return jstestprovider.NegateConfig{}, invalid(errors.New("--root is required"))
	}
	if *dir == "" {
		*dir = resolvedRoot
	}
	resolvedDir, err := resolve(*dir)
	if err != nil {
		return jstestprovider.NegateConfig{}, invalid(err)
	}
	resolvedConfig, err := resolve(*configFile)
	if err != nil {
		return jstestprovider.NegateConfig{}, invalid(err)
	}
	resolvedTestFiles := make([]string, len(testFiles))
	for i, file := range testFiles {
		if resolvedTestFiles[i], err = resolve(file); err != nil {
			return jstestprovider.NegateConfig{}, invalid(err)
		}
	}
	var attestation *jstestprovider.ApplicationAttestationProvider
	if *appAttestationCommand != "" || *appAttestationConfig != "" {
		var command []string
		attestationConfig, err := resolve(*appAttestationConfig)
		if err != nil || *appAttestationCommand == "" || attestationConfig == "" || json.Unmarshal([]byte(*appAttestationCommand), &command) != nil || len(command) == 0 {
			return jstestprovider.NegateConfig{}, invalid(errors.New("app-attestation-command and app-attestation-config must name a nonempty JSON argv and config"))
		}
		attestation = &jstestprovider.ApplicationAttestationProvider{Argv: command, ConfigFile: attestationConfig, Timeout: *appAttestationTimeout}
	}
	return jstestprovider.NegateConfig{
		E2E: jstestprovider.E2EConfig{
			Config: jstestprovider.Config{
				Dir:             resolvedDir,
				TestFiles:       resolvedTestFiles,
				ConfigFile:      resolvedConfig,
				PackageJSON:     *packageJSON,
				Lockfile:        *lockfile,
				RunnerName:      "playwright",
				RunnerVersion:   *runnerVersion,
				DeclaredEnvKeys: envKeys,
				Timeout:         *timeout,
			},
			ExternalServer:         true,
			AppIdentity:            *appIdentity,
			ServerReadyURL:         *serverReadyURL,
			ServerReadyLimit:       *serverReadyTimeout,
			AppBuildDir:            *appBuildDir,
			ApplicationAttestation: attestation,
		},
		Root:           resolvedRoot,
		Spec:           *spec,
		Test:           *test,
		Project:        *project,
		Step:           *step,
		AllSteps:       *allSteps,
		MaxRuns:        *maxRuns,
		BaselineRepeat: *baselineRepeat,
	}, nil
}
