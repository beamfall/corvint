package main

import (
	"context"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/lspprovider"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// lspSource is the provider row's source for the in-process gopls provider.
const lspSource = "lsp:gopls"

// attachLSPEvidence adds the gopls definition and reference expansion as the
// packet's separated `external` member when `CORVINT_CONTEXT_LSP=gopls`
// (TCP-V0-043). Any other value leaves the packet, and so its bytes,
// unchanged. The member never changes `results`, and a provider that is
// absent or fails is an unavailable provider row with its reason (TCP-V0-045).
func attachLSPEvidence(ctx context.Context, index *contextindex.Index, subject string, packet map[string]any, limit int) {
	if runtimeenv.Value("CONTEXT_LSP") != lspprovider.ProviderID {
		return
	}
	executable, err := exec.LookPath(lspprovider.ProviderID)
	if err != nil {
		executable = "" // includes exec.ErrDot: a gopls in the repository is never run
	}
	result := lspprovider.Expand(ctx, lspprovider.Request{
		Root: index.Root, Revision: index.CommitRevision, Seeds: lspSeeds(subject, packet),
		Committed: func(path string) (string, string, bool) { return committedText(index, path) }, Executable: executable,
	})
	anchors := append(append([]string{}, result.Query["seeds"].([]string)...), result.Origins...)
	section := extevidence.InlineSection(ctx, index, lspSource, result.Record, result.Failure, anchors, limit)
	section["query"] = result.Query
	packet["external"] = section
}

// lspSeeds is the subject, then the packet's result paths, in packet order.
func lspSeeds(subject string, packet map[string]any) []string {
	seeds := []string{}
	if subject != "" {
		seeds = append(seeds, subject)
	}
	results, _ := packet["results"].([]any)
	for _, row := range results {
		id, _ := row.(map[string]any)["id"].(string)
		if strings.HasSuffix(id, ".go") {
			seeds = append(seeds, id)
		}
	}
	return seeds
}

// committedText is the indexed text and blob of a pinned, valid text source.
func committedText(index *contextindex.Index, path string) (string, string, bool) {
	source, ok := index.Sources[path]
	if !ok || source.BlobHash == "" {
		return "", "", false
	}
	text, valid, loaded := source.Text()
	return text, source.BlobHash, valid && loaded
}
