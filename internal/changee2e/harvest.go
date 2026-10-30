package changee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Harvester schemas, identity, ledger path and limit (CCE-V0-041..043).
const (
	ProposalsSchema   = "e2e-block-proposals/0"
	AcceptancesSchema = "e2e-acceptances/0"
	HarvesterVersion  = "corvint-test-e2e-harvest/0"
	AcceptancesPath   = ".corvint/e2e-acceptances.json"
	MaxHarvestFiles   = 2000
)

// Proposal is one harvested candidate block. It is never a block until the acceptance ledger
// admits its ID at the catalog revision (CCE-V0-043).
type Proposal struct {
	ID      string  `json:"id"`
	Block   Block   `json:"block"`
	Binding Binding `json:"binding"`
}

// Binding names the flow or screen a harvested proposal belongs to in the application map at the
// harvested revision, or the reason it has none (CCE-V0-050). Exactly one of a bound target with
// its basis, or Unbound, is set.
type Binding struct {
	Flow    string `json:"flow,omitempty"`
	Screen  string `json:"screen,omitempty"`
	Basis   string `json:"basis,omitempty"`
	Unbound string `json:"unbound,omitempty"`
}

// Withheld is a harvestable-looking line that is not proposed, with its reason (CCE-V0-042).
type Withheld struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// Proposals is the e2e-block-proposals/0 document.
type Proposals struct {
	Schema    string `json:"schema"`
	Harvester string `json:"harvester"`
	Revision  string `json:"revision"`
	// Roots are the map's aliased-root pins, one of which names the tests root read (CCE-V0-131).
	Roots     []appmap.RootPin `json:"roots,omitempty"`
	Files     int              `json:"files"`
	Proposals []Proposal       `json:"proposals"`
	Withheld  []Withheld       `json:"withheld"`
	Truncated int              `json:"truncated,omitempty"` // proposals dropped by the byte cap (CCE-V0-014)
	Digest    string           `json:"digest"`

	encoded []byte
}

// Encoded returns the sealed bytes of the proposals document.
func (p *Proposals) Encoded() []byte { return p.encoded }

var (
	harvestFile    = regexp.MustCompile(`\.(spec|test)\.(ts|tsx|js|jsx|mjs|cjs)$`)
	harvestOutcome = regexp.MustCompile(`^(?:await\s+)?expect\(\s*page\.getByTestId\(\s*'([^'\\]+)'\s*\)\s*\)\.(toHaveText|toContainText|toHaveValue)\(\s*'([^'\\]*)'\s*\)\s*;?$`)
	harvestGoto    = regexp.MustCompile(`^(?:await\s+)?page\.goto\(\s*'(/[^'\\]*)'\s*\)\s*;?$`)
	harvestCalls   = regexp.MustCompile(`\bexpect\(\s*page\.getByTestId\(|\bpage\.goto\(`)
)

// HarvestOptions are the harvester inputs.
type HarvestOptions struct {
	Root, Rev string
	Manifest  string
	// Roots binds the manifest's aliased roots; with a tests alias the test files are read from
	// that root at the map's pin (CCE-V0-131).
	Roots appmap.Roots
}

// Harvest proposes outcome and path blocks from the literal assertions and navigations of the
// tracked test files at a revision (CCE-V0-041, CCE-V0-042). It reads committed bytes only and
// its output is byte-identical for the same revision.
func Harvest(ctx context.Context, o HarvestOptions) (*Proposals, error) {
	if o.Root == "" {
		return nil, errNoRoot
	}
	root, err := resolveExisting(o.Root)
	if err != nil {
		return nil, err
	}
	rev, err := resolveRev(ctx, root, o.Rev)
	if err != nil {
		return nil, err
	}
	// The map is read only to bind proposals; without it every proposal is unbound (CCE-V0-050).
	manifest := o.Manifest
	if manifest == "" {
		manifest = "appmap.json"
	}
	m, err := appmap.BuildRoots(ctx, root, manifest, rev, o.Roots)
	if err != nil {
		if o.Roots.Aliased() {
			// Without the map's pin the tests root and its revision are unknown (CCE-V0-131).
			return nil, refuse("e2e-map-unavailable", "%v", err)
		}
		m = nil
	}
	p, err := harvestAt(ctx, rev, testsRootOf(root, rev, m, o.Roots), m)
	if err != nil {
		return nil, err
	}
	if m != nil {
		p.Roots = m.Roots
	}
	p.encoded, err = capList(&p.Proposals, &p.Truncated, func() ([]byte, error) { return seal(p, &p.Digest) })
	return p, err
}

// testsRoot is where the map's test files were read: --root at the evaluated revision, or the
// aliased tests root at its pin (CCE-V0-131).
type testsRoot struct{ dir, rev, repo string }

// testsRootOf names the aliased root the map pinned for its tests input, else root at rev.
func testsRootOf(root, rev string, m *appmap.Map, roots appmap.Roots) testsRoot {
	if m != nil {
		for _, pin := range m.Roots {
			for _, in := range pin.Inputs {
				if in == "tests" {
					return testsRoot{dir: roots.Repos[pin.Repo], rev: pin.Revision, repo: pin.Repo}
				}
			}
		}
	}
	return testsRoot{dir: root, rev: rev}
}

// harvestAt lists the test files of src; the document's revision is rev, the --root revision.
func harvestAt(ctx context.Context, rev string, src testsRoot, m *appmap.Map) (*Proposals, error) {
	out, err := git(ctx, src.dir, "ls-tree", "-r", "-z", "--full-tree", src.rev)
	if err != nil {
		return nil, refuse("e2e-revision-invalid", "list %s: %v", src.rev, err)
	}
	type file struct{ path, oid string }
	var files []file
	for _, rec := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" || !harvestFile.MatchString(p) {
			continue
		}
		files = append(files, file{p, fields[2]})
	}
	if len(files) > MaxHarvestFiles {
		return nil, refuse("e2e-harvest-over-limit", "%d test files exceed %d", len(files), MaxHarvestFiles)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	p := &Proposals{Schema: ProposalsSchema, Harvester: HarvesterVersion, Revision: rev, Files: len(files),
		Proposals: []Proposal{}, Withheld: []Withheld{}}
	seen := map[string]bool{}
	for _, f := range files {
		data, err := git(ctx, src.dir, "cat-file", "blob", f.oid)
		if err != nil {
			return nil, err
		}
		for i, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
			if !harvestCalls.MatchString(line) {
				continue
			}
			b, reason := harvestLine(line)
			if reason != "" {
				p.Withheld = append(p.Withheld, Withheld{Path: f.path, Line: i + 1, Reason: reason})
				continue
			}
			b.Blob, b.Relation = f.oid, "harvested-proposal"
			b.Span = Span{Repo: src.repo, Path: f.path, Start: i + 1, End: i + 1, SHA256: sha256Hex([]byte(raw))}
			b.Provenance = fmt.Sprintf("harvested:%s:%d-%d", f.oid, i+1, i+1)
			b.ID = proposalID(b)
			if seen[b.ID] {
				continue // the first occurrence by path, then line, carries the provenance
			}
			seen[b.ID] = true
			p.Proposals = append(p.Proposals, Proposal{ID: b.ID, Block: b, Binding: bindProposal(m, b)})
		}
	}
	sort.Slice(p.Proposals, func(i, j int) bool { return p.Proposals[i].ID < p.Proposals[j].ID })
	return p, nil
}

// harvestLine reads one line naming a harvestable call; reason is set when it is withheld.
func harvestLine(line string) (Block, string) {
	if m := harvestOutcome.FindStringSubmatch(line); m != nil {
		if reason := outcomeAdmissionReason(m[2], m[1], m[3]); reason != "" {
			return Block{}, reason
		}
		if secretShaped.MatchString(m[3]) || secretscreen.MatchString(m[3]) {
			return Block{}, "harvest-secret-shaped"
		}
		return Block{Kind: KindOutcome, Matcher: m[2], Locator: m[1], Value: m[3]}, ""
	}
	if m := harvestGoto.FindStringSubmatch(line); m != nil {
		if secretShaped.MatchString(m[1]) || secretscreen.MatchString(m[1]) {
			return Block{}, "harvest-secret-shaped"
		}
		return Block{Kind: KindPath, URL: m[1]}, ""
	}
	return Block{}, "harvest-not-literal"
}

// proposalID names a proposal by its content alone, so moving the line keeps the ID and an
// acceptance stays valid, while any change to what the block asserts or opens changes it.
func proposalID(b Block) string {
	sum := sha256Hex([]byte(strings.Join([]string{b.Kind, b.Locator, b.Matcher, b.Value, b.URL}, "\x00")))
	return b.Kind + ":harvested:" + sum[:16]
}

type acceptances struct {
	Schema      string `json:"schema"`
	Acceptances []struct {
		ProposalID string `json:"proposal_id"`
		Reason     string `json:"reason"`
	} `json:"acceptances"`
}

// readAcceptances reads and validates the acceptance ledger at rev; ok is false when it is absent.
func readAcceptances(ctx context.Context, root, rev string) (l acceptances, ok bool, err error) {
	data, _, ok, err := blobAt(ctx, root, rev, AcceptancesPath)
	if err != nil || !ok {
		return l, false, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil || dec.More() {
		return l, true, refuse("e2e-acceptances-invalid", "%s: %v", AcceptancesPath, err)
	}
	if l.Schema != AcceptancesSchema {
		return l, true, refuse("e2e-acceptances-invalid", "%s: schema %q", AcceptancesPath, l.Schema)
	}
	accepted := map[string]bool{}
	for _, a := range l.Acceptances {
		if !strings.Contains(a.ProposalID, ":harvested:") || accepted[a.ProposalID] || !nonEmptyText(a.Reason) {
			return l, true, refuse("e2e-acceptances-invalid", "%s: acceptance %q", AcceptancesPath, a.ProposalID)
		}
		accepted[a.ProposalID] = true
	}
	return l, true, nil
}

// addAcceptances admits accepted harvested proposals at rev (CCE-V0-043). Without the ledger
// nothing is harvested and the catalog is unchanged.
func (c *Catalog) addAcceptances(ctx context.Context, root, rev string) error {
	l, ok, err := readAcceptances(ctx, root, rev)
	if err != nil || !ok {
		return err
	}
	p, err := harvestAt(ctx, rev, c.tests, c.m)
	if err != nil {
		return err
	}
	proposed := map[string]Block{}
	for _, pr := range p.Proposals {
		proposed[pr.ID] = pr.Block
	}
	// A harvested ID holds a second colon, which no declared or map-derived block ID does, so an
	// accepted proposal cannot shadow another block.
	for _, a := range l.Acceptances {
		b, ok := proposed[a.ProposalID]
		if !ok {
			c.Unadmitted = append(c.Unadmitted, Unadmitted{Kind: proposalKind(a.ProposalID), ID: a.ProposalID, Reason: "acceptance-stale"})
			continue
		}
		b.Relation = "accepted-proposal"
		c.Blocks = append(c.Blocks, b)
	}
	return nil
}

func proposalKind(id string) string {
	kind, _, _ := strings.Cut(id, ":")
	return kind
}

// acceptancesInDiff lists the head catalog's accepted proposals that the base ledger does not
// accept (CCE-V0-044): an acceptance added by the change under test is flagged, never trusted
// silently. A base ledger that does not validate accepts nothing.
// A ledger read from an aliased test root has no base side in --root's diff, so every acceptance
// it holds is flagged (CCE-V0-130).
func acceptancesInDiff(ctx context.Context, root, base string, head *Catalog) ([]string, error) {
	l, err := acceptances{}, error(nil)
	if head.tests.repo == "" {
		l, _, err = readAcceptances(ctx, root, base)
	}
	var r *Refusal
	if errors.As(err, &r) {
		l, err = acceptances{}, nil
	}
	if err != nil {
		return nil, err
	}
	atBase := map[string]bool{}
	for _, a := range l.Acceptances {
		atBase[a.ProposalID] = true
	}
	var added []string
	for _, b := range head.Blocks {
		if b.Relation == "accepted-proposal" && !atBase[b.ID] {
			added = append(added, b.ID)
		}
	}
	return added, nil
}

// bindProposal binds a harvested block through the application map (CCE-V0-050). A path block
// binds to the screen the map resolved for the same navigation line of the same test file; an
// outcome block binds to the single flow declaring an outcome on the same locator, else to the
// screen the map gives its test file. Anything else is unbound with its reason; nothing is
// guessed from names. The site-knowledge IR is not consulted by this slice.
func bindProposal(m *appmap.Map, b Block) Binding {
	if m == nil {
		return Binding{Unbound: "app-map-unavailable"}
	}
	var file *appmap.TestFile
	for i := range m.Files {
		if m.Files[i].Path == b.Span.Path && m.Files[i].Anchor.Repo == b.Span.Repo {
			file = &m.Files[i]
			break
		}
	}
	if b.Kind == KindPath {
		if file == nil {
			return Binding{Unbound: "test-file-not-mapped"}
		}
		for _, g := range file.Gotos {
			if g.Line == b.Span.Start {
				if g.Screen == "" {
					return Binding{Unbound: "goto-unresolved"}
				}
				return Binding{Screen: g.Screen, Basis: "app-map-goto"}
			}
		}
		return Binding{Unbound: "goto-not-mapped"}
	}
	var flows []string
	for _, f := range m.Flows {
		for _, oc := range f.Outcomes {
			if oc.Locator == b.Locator {
				flows = append(flows, f.FlowID)
				break
			}
		}
	}
	switch {
	case len(flows) == 1:
		return Binding{Flow: flows[0], Basis: "flow-outcome-locator"}
	case len(flows) > 1:
		return Binding{Unbound: "flow-ambiguous"}
	case file != nil && file.Screen != "":
		return Binding{Screen: file.Screen, Basis: "test-file-screen"}
	}
	return Binding{Unbound: "no-flow-or-screen"}
}
