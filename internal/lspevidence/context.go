// Package lspevidence attaches explicitly requested semantic evidence to a
// task-context packet without changing its ranked results or authority.
package lspevidence

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/lspprovider"
)

const Source = "lsp:gopls"

var ErrRepositoryChanged = errors.New("repository changed during LSP enrichment")

// Executable resolves only an absolute operator-provided PATH entry. A missing
// binary is retained as unavailable evidence, never installed by a read.
func Executable() string {
	path, err := exec.LookPath(lspprovider.ProviderID)
	if err != nil || !filepath.IsAbs(path) {
		return ""
	}
	return path
}

// Attach brackets the live workspace read with the original index observation
// (TCP-V0-052). On drift no packet containing external evidence is returned.
func Attach(ctx context.Context, index *contextindex.Index, subject string, packet map[string]any, limit int, executable string) error {
	if err := unchanged(ctx, index); err != nil {
		return err
	}
	result := lspprovider.Expand(ctx, lspprovider.Request{
		Root: index.Root, Revision: index.CommitRevision, Seeds: seeds(subject, packet),
		Committed: func(path string) (string, string, bool) {
			source, ok := index.Sources[path]
			if !ok || source.BlobHash == "" {
				return "", "", false
			}
			text, valid, loaded := source.Text()
			return text, source.BlobHash, valid && loaded
		}, Executable: executable,
	})
	if err := ctx.Err(); err != nil {
		return err
	}
	anchors := append(append([]string{}, result.Query["seeds"].([]string)...), result.Origins...)
	section := extevidence.InlineTaskSection(ctx, index, Source, result.Record, result.Failure, anchors, subject, limit)
	section["query"] = result.Query
	if err := unchanged(ctx, index); err != nil {
		return err
	}
	packet["external"] = section
	return nil
}

func unchanged(ctx context.Context, index *contextindex.Index) error {
	observed, err := contextindex.Observe(ctx, index.Root)
	if err != nil {
		return err
	}
	if observed.CommitRevision != index.CommitRevision || observed.Revision != index.Revision || observed.ObjectFormat != index.ObjectFormat || observed.StatusSHA256 != index.StatusSHA256 || !slices.Equal(observed.DirtyPaths, index.DirtyPaths) {
		return ErrRepositoryChanged
	}
	return nil
}

func seeds(subject string, packet map[string]any) []string {
	out := []string{}
	if subject != "" {
		out = append(out, subject)
	}
	rows, _ := packet["results"].([]any)
	for _, row := range rows {
		fields, _ := row.(map[string]any)
		id, _ := fields["id"].(string)
		if strings.HasSuffix(id, ".go") {
			out = append(out, id)
		}
	}
	return out
}
