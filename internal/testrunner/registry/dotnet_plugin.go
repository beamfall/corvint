// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
)

// DotnetCapture is core-observed completion, not a fact inferred from stdout.
// Complete must include successful output capture and process-tree retirement.
type DotnetCapture struct {
	Complete bool
	ExitCode int
}

// DotnetSource is immutable core-captured repository content. No method below
// opens a worktree file or writes an evidence store.
type DotnetSource struct {
	Blob testevidence.Blob
	Data []byte
}

// DotnetProject is the evaluated build graph for one project/TFM. Compile and
// References must include implicit globs, imports and conditional MSBuild items;
// raw csproj XML cannot attest that closure. Assets is its captured restore file.
type DotnetProject struct {
	Framework           string
	Complete            bool
	Compile, References []string
	Assets              []byte
}

// DotnetCollection names core-captured Cobertura artifacts (including coverlet
// exports). Every test in the observed process tree receives the union; the
// plugin never invents per-test attribution from a suite coverage file.
type DotnetCollection struct {
	Tests, Artifacts []string // native inventory IDs; artifact handles, not paths
	Complete         bool     // core proves complete instrumentation/export
}

// DotnetConfig is an in-process attachment to the existing SPI, not a wire.
// Platform is the core's evaluated mode: vstest, mtp, or mtp-msbuild. Empty or
// unknown means FULL, never a guess from a package name or a project suffix.
// Bindings maps list-tests IDs to repository sources (PDB/adapter observations
// for VSTest). ResultIDs maps native TRX identities to list-tests IDs; an absent
// bridge cannot make two different identities equal, especially parameter cases.
type DotnetConfig struct {
	Key                                        testrunner.InventoryKey
	Repository, Root, Executable, Project, TFM string
	Platform                                   string
	Sources                                    map[string]DotnetSource
	Projects                                   map[string]DotnetProject
	Bindings, ResultIDs, SourceNames           map[string]string
	Discovery, Run                             DotnetCapture
	Collections                                []DotnetCollection
	Artifacts                                  map[string][]byte
}

type dotnetPlugin struct {
	c      DotnetConfig
	reason string
}

// NewDotnetPlugin attaches captured inputs to the six existing .NET runner
// declarations. Unconfigured SelectionPlugin calls retain explicit absence.
// Qualification belongs to the T6 harness; this factory cannot qualify itself.
func NewDotnetPlugin(c DotnetConfig) (RunnerPlugin, testrunner.InventoryKey, error) {
	if !dotnetRunner(c.Key.Runner) || !dotnetPath(c.Project) || !strings.HasSuffix(c.Project, ".csproj") || !dotnetWord(c.TFM) || !filepath.IsAbs(c.Root) || !dotnetText(c.Root) || !filepath.IsAbs(c.Executable) || !dotnetText(c.Executable) || !dotnetText(c.Repository) {
		return nil, c.Key, fmt.Errorf("dotnet-configuration-invalid")
	}
	if b, err := hex.DecodeString(c.Key.ConfigDigest); err != nil || len(b) != 32 || strings.ToLower(c.Key.ConfigDigest) != c.Key.ConfigDigest {
		return nil, c.Key, fmt.Errorf("dotnet-configuration-digest-invalid")
	}
	// The project and TFM enter both the inventory key and each test identity.
	// A multi-target project never shares an inventory or record across TFMs.
	data, _ := json.Marshal([]string{c.Key.ConfigDigest, c.Key.Runner, c.Project, c.TFM, c.Platform})
	sum := sha256.Sum256(data)
	c.Key.ConfigDigest = hex.EncodeToString(sum[:])
	if _, err := testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: c.Key, Complete: true}, c.Key); err != nil {
		return nil, c.Key, err
	}
	// Copy the bounded attachment so later core/caller changes cannot silently
	// alter an admitted inventory's graph or attribution.
	data, err := json.Marshal(c)
	if err != nil || len(data) > 32<<20 {
		return nil, c.Key, fmt.Errorf("dotnet-input-cap-exceeded")
	}
	var copy DotnetConfig
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, c.Key, err
	}
	p := &dotnetPlugin{c: copy}
	wantMTP := strings.HasPrefix(c.Key.Runner, "dotnet-mtp-")
	if c.Platform != "vstest" && c.Platform != "mtp" && c.Platform != "mtp-msbuild" {
		p.reason = "dotnet-platform-undetermined"
	} else if wantMTP != (c.Platform != "vstest") {
		p.reason = "dotnet-platform-runner-mismatch"
	}
	return p, c.Key, nil
}

func dotnetRunner(r string) bool {
	switch r {
	case runnernames.RunnerDotnetMtpMstest, runnernames.RunnerDotnetMtpNunit, runnernames.RunnerDotnetMtpXunit, runnernames.RunnerDotnetVstestMstest, runnernames.RunnerDotnetVstestNunit, runnernames.RunnerDotnetVstestXunit:
		return true
	}
	return false
}
func dotnetText(s string) bool {
	return s != "" && len(s) <= 4096 && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}
func dotnetPath(s string) bool {
	return dotnetText(s) && path.Clean(s) == s && s != "." && s != ".." && !strings.HasPrefix(s, "../") && !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, "\\:")
}
func dotnetWord(s string) bool {
	return dotnetText(s) && !strings.ContainsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_'
	}) && !strings.HasPrefix(s, "-")
}
func (p *dotnetPlugin) id(native string) string { return p.c.Project + "::" + p.c.TFM + "::" + native }
func (p *dotnetPlugin) Pieces() testrunner.RunnerPieces {
	qualification := "unqualified"
	if p.reason != "" {
		qualification = p.reason
	}
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: qualification}}
}
func (p *dotnetPlugin) check(ctx context.Context, runner string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.reason != "" {
		return fmt.Errorf("%s", p.reason)
	}
	if runner != p.c.Key.Runner {
		return fmt.Errorf("dotnet-runner-mismatch")
	}
	return nil
}

func (p *dotnetPlugin) Inventory(ctx context.Context, r InventoryRequest) (testrunner.RunnerInventory, error) {
	fail := func(err error) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Key: p.c.Key, Reason: err.Error()}, err
	}
	if err := p.check(ctx, r.Key.Runner); err != nil {
		return fail(err)
	}
	if r.Key != p.c.Key || !p.c.Discovery.Complete || p.c.Discovery.ExitCode != 0 || len(r.Output) == 0 || len(r.Output) > testrunner.MaxReportBytes || !utf8.ValidString(r.Output) || !strings.HasSuffix(r.Output, "\n") {
		return fail(fmt.Errorf("dotnet-inventory-capture-incomplete"))
	}
	var tests []testrunner.InventoryTest
	if p.c.Platform == "vstest" {
		found := false
		for _, line := range strings.Split(strings.ReplaceAll(r.Output, "\r\n", "\n"), "\n") {
			if line == "The following Tests are available:" && !found {
				found = true
				continue
			}
			if !found {
				// Only the native driver banner is allowed ahead of the list.
				if line != "" && !strings.HasPrefix(line, "Test run for ") && !strings.HasPrefix(line, "VSTest version ") {
					return fail(fmt.Errorf("dotnet-inventory-output-unrecognized"))
				}
				continue
			}
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "    ") || !dotnetText(strings.TrimSpace(line)) {
				return fail(fmt.Errorf("dotnet-inventory-row-invalid"))
			}
			name := strings.TrimSpace(line)
			tests = append(tests, testrunner.InventoryTest{ID: p.id(name), Path: p.c.Bindings[name]})
		}
		if !found {
			return fail(fmt.Errorf("dotnet-inventory-list-missing"))
		}
	} else {
		var doc dotnetMTPList
		if err := jsonv2.Unmarshal([]byte(r.Output), &doc, jsonv2.RejectUnknownMembers(true)); err != nil || doc.Version != 1 || doc.Tests == nil {
			return fail(fmt.Errorf("dotnet-mtp-inventory-invalid"))
		}
		for _, test := range doc.Tests {
			if !dotnetText(test.UID) || !dotnetText(test.Name) {
				return fail(fmt.Errorf("dotnet-mtp-inventory-identity-invalid"))
			}
			source := p.c.Bindings[test.UID]
			// Evidence reconciles missing collections against this immutable map.
			// A native location may validate a binding, but cannot replace it.
			if _, err := p.source(source); err != nil || !strings.HasSuffix(source, ".cs") {
				return fail(fmt.Errorf("dotnet-inventory-source-unresolved:%s", source))
			}
			if test.Location != nil {
				mapped := p.c.SourceNames[test.Location.File]
				if mapped == "" && dotnetPath(test.Location.File) {
					mapped = test.Location.File
				}
				if mapped == "" || source != "" && source != mapped || test.Location.Start < 0 || test.Location.End < test.Location.Start {
					return fail(fmt.Errorf("dotnet-mtp-location-unresolved"))
				}
				source = mapped
			}
			tests = append(tests, testrunner.InventoryTest{ID: p.id(test.UID), Path: source})
		}
	}
	if len(tests) > testrunner.MaxTests {
		return fail(fmt.Errorf("dotnet-inventory-cap-exceeded"))
	}
	for _, t := range tests {
		if _, err := p.source(t.Path); err != nil || !strings.HasSuffix(t.Path, ".cs") {
			return fail(fmt.Errorf("dotnet-inventory-source-unresolved:%s", t.Path))
		}
	}
	in, err := testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: p.c.Key, Tests: tests, Complete: true}, r.Key)
	if err != nil {
		return fail(err)
	}
	return in, nil
}

// Schema 1 of the native MTP --list-tests json report. This is a decoder of
// native output, not a change to corvint-runner/1 or the existing runner wires.
type dotnetMTPList struct {
	Version int `json:"schemaVersion"`
	Tests   []struct {
		UID  string `json:"uid"`
		Name string `json:"displayName"`
		Type *struct {
			Assembly   string   `json:"assemblyFullName"`
			Namespace  string   `json:"namespace"`
			Type       string   `json:"typeName"`
			Method     string   `json:"methodName"`
			Arity      int      `json:"methodArity"`
			Return     string   `json:"returnTypeFullName"`
			Parameters []string `json:"parameterTypeFullNames"`
		} `json:"type"`
		Location *struct {
			File  string `json:"file"`
			Start int    `json:"lineStart"`
			End   int    `json:"lineEnd"`
		} `json:"location"`
		Traits     []dotnetPair `json:"traits"`
		Properties []dotnetPair `json:"properties"`
	} `json:"tests"`
}
type dotnetPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (p *dotnetPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := p.check(ctx, r.Key.Runner); err != nil {
		return CommandResponse{}, err
	}
	if r.Key != p.c.Key || !filepath.IsAbs(r.RunDir) || !dotnetText(r.RunDir) || (r.Purpose != "inventory" && r.Purpose != "run" && r.Purpose != "collect") || r.Tests != nil && len(r.Tests) == 0 {
		return CommandResponse{}, fmt.Errorf("dotnet-command-request-invalid")
	}
	// Native adapter filters do not promise exact parameter-case identity.
	// Until qualification supplies an exact filter model, execute an explicit
	// FULL superset. No selector string ever reaches an MSBuild expression.
	for _, id := range r.Tests {
		if !strings.HasPrefix(id, p.id("")) || !dotnetText(id) {
			return CommandResponse{}, fmt.Errorf("dotnet-selection-target-mismatch")
		}
	}
	args := []string{p.c.Executable, "test"}
	if p.c.Platform == "mtp" {
		args = append(args, "--project", p.c.Project)
	} else {
		args = append(args, p.c.Project)
	}
	args = append(args, "--framework", p.c.TFM, "--no-build", "--no-restore")
	if p.c.Platform == "mtp-msbuild" {
		args = append(args, "-p:TestingPlatformCaptureOutput=false", "--")
	}
	out := CommandResponse{Cwd: p.c.Root, Env: map[string]string{"DOTNET_CLI_UI_LANGUAGE": "en-US", "DOTNET_CLI_TELEMETRY_OPTOUT": "1", "TESTINGPLATFORM_TELEMETRY_OPTOUT": "1"}, Superset: r.Tests != nil}
	if r.Purpose == "inventory" {
		args = append(args, "--list-tests")
		if p.c.Platform != "vstest" {
			args = append(args, "json", "--no-banner", "--no-ansi", "--no-progress")
		}
	} else if p.c.Platform == "vstest" {
		args = append(args, "--logger", "trx;LogFileName=results.trx", "--results-directory", r.RunDir)
		out.Artifacts = []string{"results.trx"}
		if r.Purpose == "collect" {
			args = append(args, "--collect:XPlat Code Coverage")
			// Core captures results.trx first, then resolves its generated
			// coverage paths with DotnetVSTestArtifacts before capturing them.
		}
	} else {
		args = append(args, "--report-trx", "--report-trx-filename", "results.trx", "--results-directory", r.RunDir, "--no-banner", "--no-ansi", "--no-progress")
		out.Artifacts = []string{"results.trx"}
		if r.Purpose == "collect" {
			args = append(args, "--coverage", "--coverage-output-format", "cobertura", "--coverage-output", filepath.Join(r.RunDir, "coverage.cobertura.xml"))
			out.Artifacts = append(out.Artifacts, "coverage.cobertura.xml")
		}
	}
	out.Argv = args
	return out, nil
}

func (p *dotnetPlugin) Parse(ctx context.Context, r ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := p.check(ctx, r.Runner); err != nil {
		return nil, err
	}
	if !p.c.Run.Complete {
		return nil, fmt.Errorf("dotnet-run-capture-incomplete")
	}
	observation, err := native.Parse(testrunner.Input{Runner: r.Runner, ExitCode: p.c.Run.ExitCode, Reports: map[string][]byte{"results.trx": []byte(r.Report)}})
	var out []testrunner.RunnerResult
	for _, test := range observation.Tests {
		id := p.c.ResultIDs[test.ID]
		if id == "" {
			// Without a bridge, retain the unmatched reported identity. Core
			// reconciliation reports divergence rather than guessing a match.
			id = test.ID
		}
		out = append(out, testrunner.RunnerResult{ID: p.id(id), Outcome: strings.ToLower(test.State), Reason: test.Message})
	}
	if err != nil && p.c.Platform == "vstest" {
		// RSP-V0-003: a rejected TRX report (such as a wrong total count) keeps
		// its validated failed rows, so a passing FULL retry cannot erase them.
		seen := map[string]bool{}
		for _, row := range out {
			seen[row.ID] = true
		}
		for _, test := range native.TRXFailures(r.Runner, []byte(r.Report)) {
			id := p.c.ResultIDs[test.ID]
			if id == "" {
				id = test.ID
			}
			if rid := p.id(id); !seen[rid] {
				seen[rid] = true
				out = append(out, testrunner.RunnerResult{ID: rid, Outcome: "failed", Reason: test.Message})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if err == nil && !observation.Complete {
		err = fmt.Errorf("dotnet-native-report-incomplete:%v", observation.Problems)
	}
	return out, err
}
