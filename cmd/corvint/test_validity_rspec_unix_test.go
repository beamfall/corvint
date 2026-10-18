//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

const rspecDiscoverSpec = `RSpec.describe User do
  subject(:user) { create(:user) }
  before { user }
  it 'creates a user' do
    expect { user }.to change(User, :count)
  end
  xit 'is skipped' do
    expect(user).to be_valid
  end
  it 'is valid' do
    expect(user).to be_valid
  end
end
`

// TestTestValidityDiscoverRSpecSourceRules: `corvint test-validity --discover` over retained Ruby
// evidence reads the bound spec file: a MEMOIZED-BLOCK-EXPECTATION example is refused as proof
// (hygiene INELIGIBLE), a skipped one is INELIGIBLE, a clean one is ELIGIBLE with its body
// expectations as the strength reason, and the MCP tool emits the same document (TSA-V0-068,
// TSA-V0-069, TSA-V0-070, V1-1787).
func TestTestValidityDiscoverRSpecSourceRules(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	helper := "RSpec.configure { }\n"
	repository := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), DirtyState: "clean"}
	helpers := []rubytestprovider.BoundFile{{Path: "spec/spec_helper.rb", Blob: rubytestprovider.BlobID("sha1", []byte(helper))}}
	document := rubytestprovider.Document{
		Profile: rubytestprovider.Profile, Runner: "rspec", ProjectDirectory: ".", ObjectFormat: "sha1",
		RepositoryBefore: repository, RepositoryAfter: repository, HelperBasis: rubytestprovider.HelperBasis,
		Helpers: helpers, HelperSet: rubytestprovider.HelperSetDigest(helpers), Examples: []rubytestprovider.Example{}, JUnit: rubytestprovider.JUnit{State: "NOT_REQUESTED"},
	}
	for _, e := range []struct {
		line   int
		status string
	}{{4, "passed"}, {7, "pending"}, {10, "passed"}} {
		document.Examples = append(document.Examples, rubytestprovider.Example{
			ID: fmt.Sprintf("spec/user_spec.rb:%d", e.line), File: "spec/user_spec.rb", Line: e.line, Status: e.status,
			SpecBlob: rubytestprovider.BlobID("sha1", []byte(rspecDiscoverSpec)), Helpers: document.HelperSet,
		})
	}
	evidence, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(root, ".git", "HEAD"):                                     "ref: refs/heads/main\n",
		filepath.Join(root, "spec", "user_spec.rb"):                             rspecDiscoverSpec,
		filepath.Join(root, "spec", "spec_helper.rb"):                           helper,
		filepath.Join(root, ".corvint", "test-evidence", "corvint-ruby-1.json"): string(evidence),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "test-validity", "--discover"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	type axis struct{ State, Reason string }
	var out struct {
		Tests []struct {
			ID          string
			Projection  struct{ Hygiene, Strength axis }
			SourceRules *struct {
				Read     string
				Findings []struct{ Rule string }
			}
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || len(out.Tests) != 3 {
		t.Fatalf("document %s: %v", stdout.String(), err)
	}
	memoized, skipped, clean := out.Tests[0], out.Tests[1], out.Tests[2]
	if memoized.Projection.Hygiene != (axis{"INELIGIBLE", "memoized-block-expectation"}) || memoized.SourceRules == nil || memoized.SourceRules.Findings[0].Rule != "MEMOIZED-BLOCK-EXPECTATION" {
		t.Errorf("memoized %+v", memoized)
	}
	if skipped.Projection.Hygiene.State != "INELIGIBLE" || skipped.SourceRules == nil || skipped.SourceRules.Findings[0].Rule != "RSPEC-SKIPPED" {
		t.Errorf("skipped %+v", skipped)
	}
	if clean.Projection.Hygiene.State != "ELIGIBLE" || clean.Projection.Strength != (axis{"NOT_MEASURED", "example-body-expectations"}) {
		t.Errorf("clean %+v", clean)
	}

	registry, registryErr := testvaliditybridge.New(root)
	if registryErr != nil {
		t.Fatal(registryErr)
	}
	structured, _, toolFailure, callErr := registry.Call(context.Background(), testvaliditybridge.ToolTestValidity, []byte(`{"discover":true}`))
	if toolFailure != nil || callErr != nil {
		t.Fatalf("failure=%+v err=%v", toolFailure, callErr)
	}
	var cliDocument any
	if err := json.Unmarshal(stdout.Bytes(), &cliDocument); err != nil {
		t.Fatal(err)
	}
	cli, cliErr := gokernel.CanonicalJSON(cliDocument)
	mcp, mcpErr := gokernel.CanonicalJSON(structured["document"])
	if cliErr != nil || mcpErr != nil || !bytes.Equal(cli, mcp) {
		t.Fatalf("cli=%s\nmcp=%s", cli, mcp)
	}
}
