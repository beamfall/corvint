// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Effects proposals (TLF-V0-888, TLF-V0-889, V1-3576): touch paths derived
// from repository paths a ticket's own prose names, for OPEN tickets whose
// effects coverage is UNKNOWN. A proposal is a heuristic, never authority:
// the read writes nothing and carries no payload a writer accepts as-is.
const (
	profileEffectsProposals = "taskman-effects-proposals/0"
	labelProposal           = "proposal: not authority"
	maxProposalPaths        = 32
	maxProposalSpanBytes    = 160

	verdictProposed   = "PROPOSED"
	verdictNoProposal = "NO_PROPOSAL"
)

// proposalPath is a repository-relative path token made of path characters:
// at least one slash, a file name with an extension, or a bare name that
// exactly matches a non-directory entry of the worktree root.
var proposalPath = regexp.MustCompile(`[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)*/?`)

type proposal struct{ path, source, span string }

// proposalsFor returns the paths that text names and that exist under root,
// each once, with its first source and span.
func proposalsFor(root string, sources [][2]string) []proposal {
	seen := map[string]bool{}
	var rootFiles map[string]bool // exact-case root file names, read once
	var out []proposal
	for _, s := range sources {
		for _, loc := range proposalPath.FindAllStringIndex(s[1], -1) {
			tok := strings.TrimRight(s[1][loc[0]:loc[1]], ".-")
			// A bare dotfile (.gitignore) or extensionless name (Makefile)
			// is a path only when it names a root file exactly: a prose
			// word must not match a directory or, on a case-insensitive
			// file system, a differently cased file.
			if !strings.Contains(tok, "/") && (strings.HasPrefix(tok, ".") || !strings.Contains(tok, ".")) {
				if rootFiles == nil {
					rootFiles = map[string]bool{}
					entries, _ := os.ReadDir(root)
					for _, e := range entries {
						if !e.IsDir() {
							rootFiles[e.Name()] = true
						}
					}
				}
				if !rootFiles[tok] {
					continue
				}
			}
			clean := filepath.ToSlash(filepath.Clean(tok))
			if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(tok) {
				continue
			}
			fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(clean)))
			if err != nil {
				continue
			}
			if fi.IsDir() {
				clean += "/"
			}
			if seen[clean] {
				continue
			}
			seen[clean] = true
			lo, hi := max(0, loc[0]-40), min(len(s[1]), loc[1]+40)
			out = append(out, proposal{path: clean, source: s[0], span: truncateUTF8(strings.TrimSpace(s[1][lo:hi]), maxProposalSpanBytes)})
			if len(out) == maxProposalPaths {
				break
			}
		}
		if len(out) == maxProposalPaths {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func effectsProposalsCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "effects-proposals"}
	_, pos, err := flags(args)
	if err == nil && len(pos) != 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket effects-proposals takes no positional argument")
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	var rows []wire.Value
	human := []string{}
	proposed := 0
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		root := rc.repo.PrimaryWorktree
		for _, id := range rc.store.Inventory.Sorted() {
			if s, ok := rc.store.Inventory.Summary(id); !ok || s.Status != ticket.StatusOpen {
				continue
			}
			rec, _ := rc.store.Inventory.Get(id)
			if rec.Effects.Coverage != "UNKNOWN" {
				continue
			}
			sources := [][2]string{{"title", rec.Title}}
			if rec.Body != nil {
				sources = append(sources, [2]string{"body", *rec.Body})
			}
			for i, c := range rec.AcceptanceCriteria {
				sources = append(sources, [2]string{"acceptanceCriteria[" + strconv.Itoa(i) + "]", c})
			}
			props := proposalsFor(root, sources)
			o := wire.NewObject()
			o.Set("ticketId", wire.String(rec.TicketID.Raw))
			o.Set("local", wire.String(rec.TicketID.Local))
			o.Set("revision", wire.String(string(rec.Revision)))
			paths := make([]wire.Value, 0, len(props))
			for _, p := range props {
				paths = append(paths, wire.ObjectValue(wire.NewObject().Set("path", wire.String(p.path)).Set("source", wire.String(p.source)).Set("span", wire.String(p.span))))
			}
			verdict := verdictNoProposal
			if len(props) > 0 {
				verdict = verdictProposed
				proposed++
			}
			o.Set("verdict", wire.String(verdict))
			o.Set("touchPaths", wire.Array(paths...))
			rows = append(rows, wire.ObjectValue(o))
			human = append(human, rec.TicketID.Local+" "+verdict+" "+strconv.Itoa(len(props))+" path(s)")
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileEffectsProposals))
	o.Set("label", wire.String(labelProposal))
	o.Set("ticketsScreened", count(len(rows)))
	o.Set("ticketsProposed", count(proposed))
	o.Set("tickets", wire.Array(rows...))
	o.Set("human", wire.Strings(append([]string{"effects-proposals: " + strconv.Itoa(proposed) + " of " + strconv.Itoa(len(rows)) + " UNKNOWN-coverage OPEN ticket(s) have a proposal; " + labelProposal}, human...)))
	o.Set("mutationAuthority", wire.Bool(false))
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	res.Untrusted = true
	return res
}
