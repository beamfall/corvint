// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitrun"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const ScriptDeclarationPath = ".corvint/test-commands.json"

// ScriptPlugins is an immutable, core-captured declaration. Each command gets
// one SPI instance and one inventory identity, so the existing one-process SPI
// can trace and reconcile aggregate commands independently (RSP-V0-050).
// This does not register a legacy execution profile or confer qualification.
type ScriptPlugins struct {
	checkout, source, digest string
	commands                 []sessiontest.LanguageCommand
	goEnv                    []string
}

// ReadScriptPlugins reads only a regular tracked blob at a pinned commit. It
// never substitutes a dirty worktree declaration, follows a link, or executes
// discovery. The explicit path also admits the existing CI schema instance.
func ReadScriptPlugins(ctx context.Context, root, checkout, source string) (*ScriptPlugins, error) {
	if source != ScriptDeclarationPath && source != sessiontest.DeclarationPath {
		return nil, fmt.Errorf("script-declaration-path-unsupported")
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, fmt.Errorf("script-declaration-unavailable:%w", err)
	}
	tree, err := scriptCommitTree(ctx, repo, checkout)
	if err != nil {
		return nil, fmt.Errorf("script-checkout-unavailable:%w", err)
	}
	entry, exists, err := repo.LookupTreeEntry(ctx, tree, source)
	if err != nil {
		return nil, fmt.Errorf("script-declaration-unavailable:%w", err)
	}
	if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, fmt.Errorf("script-declaration-not-tracked-regular")
	}
	raw, err := repo.BlobBytesBounded(ctx, entry.OID, sessiontest.MaxDocumentBytes)
	if err != nil {
		return nil, fmt.Errorf("script-declaration-unavailable:%w", err)
	}
	return parseScriptPlugins(checkout, source, raw)
}

func parseScriptPlugins(checkout, source string, raw []byte) (*ScriptPlugins, error) {
	d := sessiontest.ParseDeclared(raw)
	if d.Unreadable != "" {
		return nil, fmt.Errorf("script-declaration-invalid:%s", d.Unreadable)
	}
	// Go's template is itself one declared aggregate command; no per-package or
	// glob expansion is invented. Other entries already contain literal argv.
	commands := []sessiontest.LanguageCommand{{Language: sessiontest.AdapterGo, Argv: append(append([]string{sessiontest.AdapterGo, "test"}, d.GoFlags...), "./...")}}
	for _, c := range d.Languages {
		if c.None == "" {
			commands = append(commands, c)
		}
	}
	if len(commands) > testrunner.MaxTests {
		return nil, fmt.Errorf("script-inventory-cap-exceeded")
	}
	sum := sha256.Sum256(raw)
	p := &ScriptPlugins{checkout: checkout, source: source, digest: hex.EncodeToString(sum[:]), commands: commands, goEnv: append([]string(nil), d.GoEnv...)}
	return p, nil
}

// IDs enumerates declaration entries, not files matching argv glob words. A
// Gradle task and a compiler check each remain one indivisible test identity.
func (p *ScriptPlugins) IDs() []string {
	if p == nil {
		return nil
	}
	ids := make([]string, 0, len(p.commands))
	for _, c := range p.commands {
		ids = append(ids, "script:"+c.Language)
	}
	return ids
}

// Plugin binds one declared command to a target-specific inventory. An unknown
// command returns an absent plugin: even a caller supplying qualified=true gets
// FULL. No PATH lookup or project process is run by these SPI methods.
func (p *ScriptPlugins) Plugin(id, os, arch string) (RunnerPlugin, testrunner.InventoryKey) {
	key := testrunner.InventoryKey{Runner: "script", OS: os, Arch: arch}
	if p != nil {
		key.Checkout = p.checkout
		sum := sha256.Sum256([]byte(p.source + "\x00" + p.digest + "\x00" + id))
		key.ConfigDigest = hex.EncodeToString(sum[:])
		for _, c := range p.commands {
			if id == "script:"+c.Language {
				env := map[string]string{}
				if c.Language == sessiontest.AdapterGo {
					for _, assignment := range p.goEnv {
						k, v, _ := strings.Cut(assignment, "=")
						env[k] = v
					}
				}
				return &scriptPlugin{key: key, id: id, source: p.source, argv: append([]string(nil), c.Argv...), env: env}, key
			}
		}
	}
	return absentPlugin{runner: "script-command-undeclared"}, key
}

type scriptPlugin struct {
	key        testrunner.InventoryKey
	id, source string
	argv       []string
	env        map[string]string
}

func (p *scriptPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "script: selection not qualified"}}
}
func (p *scriptPlugin) Inventory(ctx context.Context, r InventoryRequest) (testrunner.RunnerInventory, error) {
	if err := ctx.Err(); err != nil {
		return testrunner.RunnerInventory{Reason: "script-inventory-cancelled"}, err
	}
	return testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: p.key, Tests: []testrunner.InventoryTest{{ID: p.id, Path: p.source}}, Complete: true}, r.Key)
}
func (p *scriptPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if r.Key != p.key || (r.Purpose != "run" && r.Purpose != "collect") || !filepath.IsAbs(r.RunDir) || strings.ContainsRune(r.RunDir, 0) {
		return CommandResponse{}, fmt.Errorf("script-command-request-invalid")
	}
	if r.Tests != nil && (len(r.Tests) != 1 || r.Tests[0] != p.id) {
		return CommandResponse{}, fmt.Errorf("script-command-undeclared")
	}
	env := make(map[string]string, len(p.env))
	for k, v := range p.env {
		env[k] = v
	}
	return CommandResponse{Argv: append([]string(nil), p.argv...), Cwd: r.RunDir, Env: env}, nil
}
func (p *scriptPlugin) Parse(ctx context.Context, r ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Only the core executor produces this in-process exit observation. Native
	// stdout/stderr is never parsed as a successful terminal result.
	code, err := strconv.Atoi(r.Report)
	if r.Runner != "script" || err != nil || code < 0 || code > 255 || strconv.Itoa(code) != r.Report {
		return nil, fmt.Errorf("script-terminal-result-invalid")
	}
	outcome := "passed"
	if code != 0 {
		outcome = "failed"
	}
	return []testrunner.RunnerResult{{ID: p.id, Outcome: outcome}}, nil
}
func (p *scriptPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != "script" || len(r.Artifacts) != 0 {
		return nil, fmt.Errorf("script-native-artifacts-unsupported")
	}
	native := scriptNative()
	executed, _ := json.Marshal(native.Executed)
	regions, _ := json.Marshal(native.Regions)
	loaded, _ := json.Marshal(native.Loaded)
	return []NativeEvidence{{Test: p.id, Executed: executed, Regions: regions, Loaded: loaded}}, nil
}
func scriptNative() testevidence.Native {
	return testevidence.Native{Executed: testevidence.Missing[testevidence.ExecutedFile]("script-kind"), Regions: testevidence.Missing[testevidence.RegionsFile]("script-kind"), Loaded: testevidence.Missing[testevidence.Blob]("script-kind")}
}

// The authority kernel also supports direct tree reads for historical callers;
// script inventories and records require an exact commit rather than that
// broader profile or a movable ref.
func scriptCommitTree(ctx context.Context, repo *gitauth.Repository, commit string) (string, error) {
	if !wire.IsGitOid(commit) {
		return "", fmt.Errorf("script-checkout-not-pinned-commit")
	}
	resolved, err := repo.Resolve(ctx, commit)
	if err != nil {
		return "", err
	}
	if resolved != commit {
		return "", fmt.Errorf("script-checkout-not-pinned-commit")
	}
	return repo.CommitTree(ctx, commit)
}
