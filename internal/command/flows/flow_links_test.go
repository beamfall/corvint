// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// Exercise the public dispatch and error envelope against a disposable repository.
func TestFLPV1004AcceptanceCLI(t *testing.T) {
	fx := newShopFixture(t)
	const file = "e2e/checkout.spec.ts"
	const id = "playwright:" + file + "::chromium::pays"
	const cfg = "playwright.config.ts"
	config := "export default {testDir:'e2e'};"
	ident, _ := testinventory.CurrentLister("playwright")
	in := testinventory.Instance{Runner: "playwright", Key: "playwright@.", Root: ".", Config: cfg, ConfigDigest: testinventory.BlobID("sha1", []byte(config)), Commit: fx.head,
		Lister: testinventory.Lister{Name: ident.Name, Version: ident.Version, Kind: ident.Kind, Command: []string{"fixture-list"}}, Tests: []testinventory.Test{{ID: id, File: file}}}
	for _, f := range []string{file, "e2e/search.spec.ts", "e2e/profile.spec.ts"} {
		raw, err := os.ReadFile(filepath.Join(fx.root, f))
		if err != nil {
			t.Fatal(err)
		}
		in.Files = append(in.Files, testinventory.File{Path: f, Blob: testinventory.BlobID("sha1", raw)})
	}
	data, err := testinventory.Encode(testinventory.Inventory{Profile: testinventory.Profile, Instances: []testinventory.Instance{in}})
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, fx.root, map[string]string{cfg: config, testinventory.Path: string(data), appflows.FlowsDeclarationPath: `{"profile":"corvint-flows/0","flows":"flows"}`})
	shopCommit(t, fx.root, "declare inventory")
	var out, diag bytes.Buffer
	env := Env{EmitError: func(_ io.Writer, err error) {
		var e *gokernel.Error
		if errors.As(err, &e) {
			diag.WriteString(e.Code)
		} else {
			diag.WriteString(err.Error())
		}
	}}
	run := func(wantCode int, wantDiag string, args ...string) {
		t.Helper()
		out.Reset()
		diag.Reset()
		code := Run(t.Context(), fx.root, args, &out, &diag, env)
		if code != wantCode || diag.String() != wantDiag {
			t.Fatalf("%s: exit %d, stderr %s stdout %s", args, code, &diag, &out)
		}
	}
	args := []string{"accept", "--flow", "checkout", "--by", "Fixture reviewer", "--only", id}
	orig := os.Stdin
	defer func() { os.Stdin = orig }()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	os.Stdin = devnull
	run(2, "flow-accept-needs-person", args...)
	if d, err := appflows.ReadFlowsDeclaration(fx.root); err != nil || len(d.Links) != 0 {
		t.Fatal("non-terminal changed the declaration", d, err)
	}
	terminal, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skip("no pseudo-terminal to exercise positive acceptance", err)
	}
	defer terminal.Close()
	os.Stdin = terminal
	run(2, "flow-proposals-absent", args...)
	run(0, "", "propose", "--links", "--flow", "checkout")
	var proposals appflows.LinkProposals
	if err := json.Unmarshal(out.Bytes(), &proposals); err != nil || len(proposals.Proposals) == 0 {
		t.Fatalf("proposal output: %s %v", &out, err)
	}
	shopWrite(t, fx.root, map[string]string{"revision": "new head"})
	shopCommit(t, fx.root, "advance HEAD")
	run(2, "flow-proposals-stale", args...)
	run(0, "", "propose", "--all")
	run(0, "", args...)
	d, err := appflows.ReadFlowsDeclaration(fx.root)
	if err != nil || len(d.Links) == 0 {
		t.Fatal(d, err)
	}
	for _, l := range d.Links {
		if l.TestID != id || !l.HasAcceptance() || l.AcceptedBy != "Fixture reviewer" || l.PinnedCommit != shopGit(t, fx.root, "rev-parse", "HEAD") {
			t.Fatal(l)
		}
	}
}

// Check the actual MCP catalogs and the tracked automation entry points.
func TestFLPV1005NoAutomationAccepts(t *testing.T) {
	for _, tool := range append((bridge.Profile{Flows: true}).Tools(), docsbridge.DescribedTools()...) {
		if strings.Contains(tool.Name, "accept") {
			t.Fatalf("accept exposed as %s", tool.Name)
		}
	}
	cmd := fixtureGitCommand("grep", "-n", "-E", `(flows["', ]+accept|AcceptLinks\(|runFlowsAccept\()`, "--",
		":(top)internal/mcp", ":(top,glob)cmd/*/*hook*.go", ":(top,glob)script/*hook*",
		":(top).github/workflows", ":(top).githooks", ":(top).claude/hooks", ":(top).codex/hooks.json", ":(top,exclude,glob)**/*_test.go")
	data, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("automation accept path or search failure: %v %s", err, data)
	}
}
