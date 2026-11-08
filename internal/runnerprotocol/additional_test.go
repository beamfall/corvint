// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// RSP-V0-026 and RSP-V0-027: the server validates both directions and never
// dispatches an unknown method or a request preceding hello.
func TestRSPV0026ServerValidation(t *testing.T) {
	hello := `{"id":"1","method":"hello","params":{"protocols":["corvint-runner/1"],"core":"test"}}` + "\n"
	shutdown := `{"id":"2","method":"shutdown","params":{}}` + "\n"
	var output bytes.Buffer
	if err := Serve(context.Background(), strings.NewReader(hello+shutdown), &output, "reference", "1", map[string]registry.RunnerPlugin{ReferenceRunner: Reference{}}); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 2 {
		t.Fatalf("%s", output.Bytes())
	}
	var h Hello
	if err := decodeResponse(lines[0], "1", "hello", &h); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{shutdown, hello + hello, hello + `{"id":"2","method":"execute","params":{}}` + "\n", hello + strings.Repeat("x", 1<<20+2)} {
		output.Reset()
		if err := Serve(context.Background(), strings.NewReader(input), &output, "reference", "1", map[string]registry.RunnerPlugin{ReferenceRunner: Reference{}}); err == nil {
			t.Fatal("accepted invalid stream")
		}
	}
}
func TestRSPV0028TrustedBlobReader(t *testing.T) {
	pin, _ := selfPin(t)
	data, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
	var revisions []string
	read := func(rev, path string) ([]byte, error) {
		if path != ".corvint/runner-plugins.json" {
			t.Fatal(path)
		}
		revisions = append(revisions, rev)
		return data, nil
	}
	plan := ReadProjectPins(read, strings.Repeat("a", 40), strings.Repeat("b", 40), true)
	if plan.FullAll != "" || len(revisions) != 2 || revisions[0] != strings.Repeat("a", 40) {
		t.Fatal(plan, revisions)
	}
	if ReadProjectPins(read, "candidate", strings.Repeat("b", 40), true).FullAll == "" {
		t.Fatal("accepted mutable base")
	}
}
func TestRSPV0029ShutdownDeadlineAndExistingDirtyFile(t *testing.T) {
	s, _ := openFixture(t, "shutdown-timeout", Limits{Request: 100 * time.Millisecond})
	start := time.Now()
	if err := s.Close(context.Background()); err == nil {
		t.Fatal("ignored shutdown")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("shutdown deadline exceeded")
	}
	resolved := fixtureResolved(t)
	root := fixtureRepo(t, "mutation")
	write(t, filepath.Join(root, "tracked.txt"), "already dirty")
	s, err := Open(context.Background(), resolved, Options{Repository: root, RunDir: t.TempDir(), CoreVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	requireFailure(t, s.Close(context.Background()), "repository-mutation")
}
func TestRSPV0030NativeValidation(t *testing.T) {
	s, c := openFixture(t, "forged-authority", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: c.Artifacts})
	requireFailure(t, err, "schema-violation")
	if len(records) > 0 {
		t.Fatal("forged evidence escaped")
	}
	// Duplicate component keys, unknown trace components, and required members
	// are all checked before typed decoding or any publication.
	var out any
	for _, raw := range []string{`{"records":[{"test":"t","executed":{"state":"present","items":[],"state":"absent"},"regions":{},"loaded":{}}]}`, `{"records":[{"test":"t","executed":{},"regions":{},"loaded":{},"reads":{}}]}`} {
		if validatePayload("evidence", []byte(raw), true, &out) == nil {
			t.Fatal("malformed native evidence accepted")
		}
	}
}
func TestRSPV0031UndeclaredResponseAndMissingExecutable(t *testing.T) {
	s, c := openFixture(t, "wrong-response-runner", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	_, err := p.Parse(context.Background(), registry.ParseRequest{Runner: c.Key.Runner, Report: c.Report})
	requireFailure(t, err, "schema-violation")
	if s.FullPlans()[ReferenceRunner] == "" {
		t.Fatal("missing FULL reason")
	}
	resolved := fixtureResolved(t)
	if err = os.Remove(resolved.path); err != nil {
		t.Fatal(err)
	}
	_, err = Open(context.Background(), resolved, Options{Repository: fixtureRepo(t, "reference"), RunDir: t.TempDir(), CoreVersion: "test"})
	requireFailure(t, err, "missing-executable")
}
func TestRSPV0041StoreRejectsClaimCollision(t *testing.T) {
	pin, source := selfPin(t)
	store := t.TempDir()
	if _, err := Install(store, pin, source); err != nil {
		t.Fatal(err)
	}
	pin.Name = "collision"
	_, err := Install(store, pin, source)
	requireFailure(t, err, "discovery-duplicate-or-empty-runner")
	config, err := ReadDiscovery(filepath.Join(store, "runner-plugins.json"))
	if err != nil || len(config.Plugins) != 1 || config.Plugins[0].Name != "reference" {
		t.Fatal("invalid install modified admitted pins", config, err)
	}
}

// RSP-V0-041: the store also stays outside bare repositories.
func TestRSPV0041BareRepositoryRefused(t *testing.T) {
	requireLinux(t)
	root := t.TempDir()
	gitFixture(t, root, "init", "--bare", "-q")
	requireFailure(t, outsideRepository(filepath.Join(root, "plugins")), "inside-repository")
}
