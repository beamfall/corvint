package dogfoodflow

import (
	"bytes"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// writeIntentsDraft writes, when --intents-draft names a file, the draft
// DOGFOOD_INTENTS_FILE of the change BASE..HEAD (DCW-V0-711): every Markdown
// file the change adds or modifies that derives as an intent at HEAD, as
// inputs-check and ocm-prepare derive it (lrfrepo.DeriveIntentAt), in the
// bytewise order and one-per-line grammar the manifest requires, or the
// no-intent manifest when none does. A file whose every hunk only repins
// citations is left out, since that edit declares no intent (DCW-V0-548), and
// an intent over one OCM map's obligation bound is left out by name. A
// derivation that fails for any reason other than a coded refusal of the
// document is refused and leaves FILE as it was. The draft is the only file
// propose-citations writes; the lane reviews it before use.
func (p *proposal) writeIntentsDraft() {
	target := p.options.IntentsDraft
	if target == "" {
		return
	}
	intents := []string{}
	for _, path := range p.changedMarkdown() {
		_, err := lrfrepo.DeriveIntentAt(p.ctx, p.root, p.target, path)
		switch {
		case err == nil:
			intents = append(intents, path)
		case overMapBound(err):
			p.say("%s: NOTE intents-draft left out %s: its Requirements section alone is over one OCM map's obligation bound, so ocm-prepare would refuse it too-many-obligations; cite its rows instead (DCW-V0-711, DCW-V0-961)\n", p.prefix, path)
		case intentRefusal(err) == nil:
			// Only a coded derivation refusal judges the document; a Git,
			// repository or context failure says nothing about it, so no
			// draft, least of all the no-intent manifest, is written.
			p.say("%s: REFUSE intents-draft-underivable %s: %v\n  %s was not written or changed: the intent derivation could not read the repository, so whether this file is an intent is unknown; fix the cause and rerun, or omit --intents-draft (DCW-V0-711)\n", p.prefix, path, err, target)
			p.exit(2)
		}
	}
	content := noIntentManifest
	if len(intents) > 0 {
		content = []byte(strings.Join(intents, "\n") + "\n")
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		p.say("%s: REFUSE intents-draft-unwritable %s: %v\n  fix: pass --intents-draft a writable file path, such as $(git rev-parse --git-dir)/intents.txt, then rerun (DCW-V0-711)\n", p.prefix, target, err)
		p.exit(2)
	}
	p.say("%s: NOTE intents-draft wrote %d intent scopes to %s; review it, then pass it as DOGFOOD_INTENTS_FILE (DCW-V0-711)\n", p.prefix, len(intents), target)
	if len(intents) > maxAggregateScopes {
		p.say("  the manifest admits at most %d scopes: keep the specs this change implements and drop the rest before use\n", maxAggregateScopes)
	}
}

// changedMarkdown is the sorted, distinct Markdown paths BASE..HEAD adds or
// modifies with an edit other than a citation repin. Without a readable
// canonical patch it falls back to Git's name list, unfiltered.
func (p *proposal) changedMarkdown() []string {
	seen := map[string]bool{}
	if parsed, ok := p.canonicalPatch(); ok {
		for _, group := range parsed.Groups {
			if group.NewPath == nil || !strings.HasSuffix(*group.NewPath, ".md") || len(group.Hunks) == 0 {
				continue
			}
			for _, hunk := range group.Hunks {
				if !repinOnly(hunk) {
					seen[*group.NewPath] = true
					break
				}
			}
		}
	} else {
		names, _ := p.git(true, "diff", "--no-renames", "--diff-filter=AM", "--name-only", p.base, p.target, "--")
		for _, path := range strings.Split(names, "\n") {
			if strings.HasSuffix(path, ".md") {
				seen[path] = true
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return bytes.Compare([]byte(paths[i]), []byte(paths[j])) < 0 })
	return paths
}
