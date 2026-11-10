// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/nativelinecov"
	"github.com/Beamfall/corvint/internal/rustsyntax"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func (p *rustPlugin) Evidence(ctx context.Context, req EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Runner != p.o.Key.Runner || len(req.Artifacts) > testrunner.MaxReports || len(p.o.Tests) == 0 {
		return nil, fmt.Errorf("rust-evidence-runner-artifacts-or-attribution-invalid")
	}
	n := testevidence.Native{
		Executed: testevidence.Missing[testevidence.ExecutedFile]("rust-coverage-not-collected"),
		Regions:  testevidence.Missing[testevidence.RegionsFile]("rust-regions-not-collected"),
		Loaded:   p.loaded(),
	}
	// These are LLVM's LCOV line exports after the core has merged every
	// instrumented process dump with llvm-profdata/llvm-cov (including dumps
	// produced by -C instrument-coverage and cargo-llvm-cov). A file's existence
	// alone cannot attest complete process-tree instrumentation or attribution.
	if len(req.Artifacts) > 0 {
		lines := v8.Lines{}
		reason := ""
		seen := map[string]bool{}
		for _, handle := range req.Artifacts {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			raw, ok := p.o.Artifacts[handle]
			if !ok || seen[handle] {
				reason = "rust-coverage-artifact-missing-or-duplicate"
				break
			}
			seen[handle] = true
			decoded, err := nativelinecov.Decode(nativelinecov.Report{Format: "lcov", Data: raw}, p.o.NativeSources)
			if err != nil {
				reason = "rust-coverage-invalid:" + err.Error()
				break
			}
			lines.Merge(decoded)
		}
		if reason == "" && !p.o.CoverageComplete {
			reason = "rust-coverage-process-tree-incomplete"
		}
		if reason == "" {
			n.Executed, n.Regions = p.lineComponents(lines)
		} else {
			n.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Incomplete, Reason: reason}
			n.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Incomplete, Reason: reason}
		}
	}
	// Only native components cross the SPI; Assemble and Attribute remain core
	// operations. In particular loaded never replaces/drops build-phase traces.
	r := testevidence.New("unused", "unused", "unused")
	r.Executed, r.Regions, r.Loaded = n.Executed, n.Regions, n.Loaded
	if err := r.ValidateComponents(); err != nil {
		return nil, fmt.Errorf("rust-evidence-invalid:%w", err)
	}
	template := rustNative("", n)
	size := len(template.Executed) + len(template.Regions) + len(template.Loaded)
	if size > testevidence.MaxRecordBytes || size*len(p.o.Tests) > 256<<20 {
		return nil, fmt.Errorf("rust-native-record-cap")
	}
	out := make([]NativeEvidence, 0, len(p.o.Tests))
	for _, test := range p.o.Tests {
		out = append(out, NativeEvidence{Test: test, Executed: bytes.Clone(template.Executed), Regions: bytes.Clone(template.Regions), Loaded: bytes.Clone(template.Loaded)})
	}
	return out, nil
}

func (p *rustPlugin) lineComponents(lines v8.Lines) (testevidence.Component[testevidence.ExecutedFile], testevidence.Component[testevidence.RegionsFile]) {
	executed := testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}
	regions := testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}
	paths := make([]string, 0, len(lines))
	for name := range lines {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		src, ok := p.o.Sources[name]
		if !ok || !strings.HasSuffix(name, ".rs") {
			executed.State, executed.Reason = testevidence.Incomplete, "rust-coverage-source-unsupported"
			regions.State, regions.Reason = testevidence.Incomplete, "rust-coverage-source-unsupported"
			continue
		}
		count := bytes.Count(src.Data, []byte{'\n'}) + 1
		sourceLines := bytes.Split(src.Data, []byte{'\n'})
		numbers := make([]int, 0, len(lines[name]))
		for line := range lines[name] {
			numbers = append(numbers, line)
		}
		sort.Ints(numbers)
		f := testevidence.ExecutedFile{Blob: src.Blob}
		for _, line := range numbers {
			if line > count {
				executed.State, executed.Reason = testevidence.Incomplete, "rust-coverage-line-outside-blob"
				continue
			}
			r := testevidence.Range{Start: int32(line), StartColumn: 1, End: int32(line + 1), EndColumn: 1}
			if lines[name][line] {
				f.Executed = append(f.Executed, r)
			} else {
				// LCOV has no region columns. End on this same line so the
				// selector cannot certify a whole unexecuted line (including
				// a match pattern) and instead checks its enclosing body.
				if len(sourceLines[line-1]) == 0 {
					continue
				}
				r.End, r.EndColumn = int32(line), int32(len(sourceLines[line-1])+1)
				f.Unexecuted = append(f.Unexecuted, r)
			}
		}
		executed.Items = append(executed.Items, f)
		bodies, err := rustsyntax.ExecutableBodies(src.Data)
		if err != nil {
			regions.State, regions.Reason = testevidence.Incomplete, err.Error()
			continue
		}
		regions.Items = append(regions.Items, testevidence.RegionsFile{Blob: src.Blob, Bodies: rustBodies(bodies)})
	}
	return executed, regions
}

type rustTarget struct {
	Kind   []string `json:"kind"`
	Source string   `json:"src_path"`
}
type rustPackage struct {
	ID       string       `json:"id"`
	Manifest string       `json:"manifest_path"`
	Source   *string      `json:"source"`
	Targets  []rustTarget `json:"targets"`
}

// loaded conservatively includes every tracked file under each local Cargo
// package (not just its entry point), plus build-script declared input trees.
// Metadata cannot enumerate mod/include!/generated inputs; the complete core
// snapshot supplies that superset. Missing snapshots, external path packages,
// unknown build messages and uncaptured build scripts keep loaded incomplete.
func (p *rustPlugin) loaded() testevidence.Component[testevidence.Blob] {
	out := testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	fail := func(reason string) { out.State, out.Reason = testevidence.Incomplete, reason }
	if !p.o.SourcesComplete {
		fail("rust-loaded-source-universe-incomplete")
	}
	var metadata struct {
		Version       int           `json:"version"`
		Packages      []rustPackage `json:"packages"`
		WorkspaceRoot string        `json:"workspace_root"`
		Members       []string      `json:"workspace_members"`
	}
	if len(p.o.Metadata) == 0 {
		return testevidence.Missing[testevidence.Blob]("rust-cargo-metadata-missing")
	}
	if err := jsonv2.Unmarshal(p.o.Metadata, &metadata); err != nil || metadata.Version != 1 || metadata.Packages == nil || len(metadata.Packages) == 0 || len(metadata.Packages) > testrunner.MaxTests || metadata.Members == nil {
		return testevidence.Component[testevidence.Blob]{State: testevidence.Incomplete, Reason: "rust-cargo-metadata-malformed-or-capped"}
	}
	loaded := map[string]bool{}
	addFile := func(name string) {
		if _, ok := p.o.Sources[name]; !ok {
			fail("rust-loaded-blob-missing:" + name)
			return
		}
		loaded[name] = true
	}
	addTree := func(prefix string) {
		for name := range p.o.Sources {
			if prefix == "." || name == prefix || strings.HasPrefix(name, prefix+"/") {
				loaded[name] = true
			}
		}
	}
	// Workspace manifest/lock/configuration are dependencies even when no
	// root package exists. An absent lock is not invented as a loaded file.
	workspace, err := p.relative(metadata.WorkspaceRoot, true)
	if err != nil {
		fail("rust-workspace-outside-repository")
	} else {
		addFile(path.Join(workspace, "Cargo.toml"))
		for dir := workspace; ; dir = path.Dir(dir) {
			for _, file := range []string{".cargo/config", ".cargo/config.toml"} {
				name := path.Join(dir, file)
				if _, ok := p.o.Sources[name]; ok {
					addFile(name)
				}
			}
			if dir == "." {
				break
			}
		}
		for _, file := range []string{"Cargo.lock", ".cargo/config", ".cargo/config.toml", "rust-toolchain", "rust-toolchain.toml", ".config/nextest.toml"} {
			name := path.Join(workspace, file)
			if _, ok := p.o.Sources[name]; ok {
				addFile(name)
			}
		}
	}
	packages := map[string]rustPackage{}
	localDirs := map[string]string{}
	for _, pkg := range metadata.Packages {
		if !rustText(pkg.ID) || packages[pkg.ID].ID != "" || pkg.Targets == nil || len(pkg.Targets) > testrunner.MaxTests {
			fail("rust-cargo-package-invalid")
			continue
		}
		packages[pkg.ID] = pkg
		if pkg.Source != nil {
			if !strings.HasPrefix(*pkg.Source, "registry+") && !strings.HasPrefix(*pkg.Source, "git+") {
				fail("rust-cargo-package-source-unsupported")
			}
			continue
		} // registry/git sources are outside this repository
		manifest, err := p.relative(pkg.Manifest, false)
		if err != nil {
			fail("rust-local-dependency-outside-repository")
			continue
		}
		if path.Base(manifest) != "Cargo.toml" {
			fail("rust-package-manifest-invalid")
			continue
		}
		addFile(manifest)
		localDirs[pkg.ID] = path.Dir(manifest)
		addTree(path.Dir(manifest))
		for _, target := range pkg.Targets {
			if len(target.Kind) == 0 {
				fail("rust-cargo-target-kind-missing")
			}
			name, err := p.relative(target.Source, false)
			if err != nil {
				fail("rust-cargo-target-unresolved")
				continue
			}
			addFile(name)
		}
	}
	for _, member := range metadata.Members {
		if _, ok := localDirs[member]; !ok {
			fail("rust-workspace-member-unresolved")
		}
	}
	if _, ok := loaded[p.o.Manifest]; !ok {
		fail("rust-runner-manifest-not-modeled")
	}
	// A successful terminal JSON build message is mandatory, including when
	// every artifact was cached. Arbitrary stdout never certifies completeness.
	done := false
	scripts := map[string]bool{}
	for _, raw := range bytes.Split(p.o.BuildMessages, []byte{'\n'}) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		if done {
			fail("rust-build-trailing-output")
			break
		}
		var msg struct {
			Reason      string     `json:"reason"`
			PackageID   string     `json:"package_id"`
			Target      rustTarget `json:"target"`
			Success     *bool      `json:"success"`
			OutDir      string     `json:"out_dir"`
			LinkedPaths []string   `json:"linked_paths"`
		}
		if !utf8.Valid(raw) || jsonv2.Unmarshal(raw, &msg) != nil {
			fail("rust-build-message-malformed")
			break
		}
		switch msg.Reason {
		case "build-finished":
			done = true
			if msg.Success == nil || !*msg.Success {
				fail("rust-build-failed")
			}
		case "compiler-artifact", "compiler-message", "build-script-executed":
			pkg, ok := packages[msg.PackageID]
			if !ok {
				fail("rust-build-package-unresolved")
				continue
			}
			if pkg.Source != nil {
				continue
			}
			if _, ok := localDirs[msg.PackageID]; !ok {
				fail("rust-build-local-package-unresolved")
				continue
			}
			if msg.Reason == "build-script-executed" {
				if !filepath.IsAbs(msg.OutDir) {
					fail("rust-build-script-output-unresolved")
				}
				scripts[msg.PackageID] = true
				output, ok := p.o.BuildScripts[msg.PackageID]
				if !ok {
					fail("rust-build-script-output-missing")
					continue
				}
				if err := p.scriptInputs(output, localDirs[msg.PackageID], addTree); err != nil {
					fail(err.Error())
				}
				// Native links can bring generated or sibling repository code
				// into the test. Without their own graph they remain incomplete.
				if len(msg.LinkedPaths) > 0 {
					fail("rust-native-link-inputs-unmodeled")
				}
			} else {
				name, err := p.relative(msg.Target.Source, false)
				if err != nil {
					fail("rust-build-target-unresolved")
				} else {
					addFile(name)
				}
			}
		default:
			fail("rust-build-message-unsupported")
		}
	}
	if !done {
		fail("rust-build-output-incomplete")
	}
	for id := range p.o.BuildScripts {
		if !scripts[id] {
			fail("rust-build-script-without-message")
		}
	}
	for _, pkg := range metadata.Packages {
		for _, target := range pkg.Targets {
			for _, kind := range target.Kind {
				if pkg.Source == nil && kind == "custom-build" && !scripts[pkg.ID] {
					fail("rust-build-script-not-captured")
				}
			}
		}
	}
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.Items = append(out.Items, p.o.Sources[name].Blob)
	}
	return out
}

func (p *rustPlugin) relative(name string, root bool) (string, error) {
	if !rustText(name) || !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return "", fmt.Errorf("rust-path-not-absolute")
	}
	rel, err := filepath.Rel(p.o.Root, name)
	rel = filepath.ToSlash(rel)
	if err != nil || !rustPath(rel) && !(root && rel == ".") {
		return "", fmt.Errorf("rust-path-outside-repository")
	}
	return rel, nil
}

func (p *rustPlugin) scriptInputs(output []byte, dir string, addTree func(string)) error {
	if !utf8.Valid(output) || bytes.ContainsAny(output, "\x00\r") {
		return fmt.Errorf("rust-build-script-output-malformed")
	}
	for _, line := range strings.Split(string(output), "\n") {
		instruction, ok := strings.CutPrefix(line, "cargo::")
		if !ok {
			instruction, ok = strings.CutPrefix(line, "cargo:")
		}
		if !ok {
			continue
		} // ordinary build-script diagnostics
		key, value, ok := strings.Cut(instruction, "=")
		if !ok {
			return fmt.Errorf("rust-build-script-instruction-malformed")
		}
		switch key {
		case "rerun-if-changed":
			if !rustText(value) {
				return fmt.Errorf("rust-build-script-input-malformed")
			}
			name := value
			if !filepath.IsAbs(name) {
				name = filepath.Join(p.o.Root, filepath.FromSlash(dir), value)
			}
			rel, err := p.relative(name, true)
			if err != nil {
				return fmt.Errorf("rust-build-script-input-outside-repository")
			}
			// Missing paths remain dependencies in the core's lookups trace.
			// Existing directories conservatively include every tracked child.
			addTree(rel)
		case "rerun-if-env-changed", "rustc-cfg", "rustc-check-cfg", "rustc-env", "warning", "metadata":
			// Environment identity and the trace belong to core.
		case "rustc-link-lib", "rustc-link-search", "rustc-flags", "rustc-link-arg", "rustc-link-arg-bin", "rustc-link-arg-bins", "rustc-link-arg-tests", "rustc-link-arg-examples", "rustc-link-arg-benches", "rustc-link-arg-cdylib":
			return fmt.Errorf("rust-build-script-native-inputs-unmodeled")
		default:
			return fmt.Errorf("rust-build-script-instruction-unsupported")
		}
	}
	return nil
}

// rustBodies converts the Rust analyzer's pure body shape to test-evidence
// bodies; the two are field-for-field identical.
func rustBodies(in []rustsyntax.Body) []testevidence.Body {
	if in == nil {
		return nil
	}
	out := make([]testevidence.Body, 0, len(in))
	for _, b := range in {
		out = append(out, testevidence.Body{ID: b.ID, Parent: b.Parent, Range: testevidence.Range(b.Range)})
	}
	return out
}
