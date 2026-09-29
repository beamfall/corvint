package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/lspevidence"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const lspSource = lspevidence.Source

// Explicit CLI selection overrides the legacy operator environment. MCP uses
// the shared attachment directly and never consults this environment variable.
func attachLSPEvidence(ctx context.Context, index *contextindex.Index, subject string, packet map[string]any, limit int, selection string) error {
	if selection == "" {
		selection = runtimeenv.Value("CONTEXT_LSP")
	}
	if selection != "gopls" {
		return nil
	}
	return lspevidence.Attach(ctx, index, subject, packet, limit, lspevidence.Executable())
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
