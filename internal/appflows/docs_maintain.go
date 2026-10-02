package appflows

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const DocMaintenanceSchema = "flow-doc-maintenance/0"
const docMaintenanceProfile = "fixed-flow-template/0"

// DocMaintenanceOptions selects operator-owned outputs; Receipt is an explicitly
// named committed provenance record, never authentication or accepted intent.
type DocMaintenanceOptions struct {
	Flows         string
	Docs          DocOptions
	EvidenceFiles []string
	Receipt       string
	Adopt         bool
}

type DocIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Blob   string `json:"blob,omitempty"`
}

type DocDestination struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256,omitempty"`
}

type DocProposal struct {
	Schema          string            `json:"schema"`
	Profile         string            `json:"profile"`
	Revision        string            `json:"revision"`
	Tree            string            `json:"tree"`
	Sources         []DocIdentity     `json:"sources"`
	Evidence        []DocIdentity     `json:"evidence"`
	Flows           string            `json:"flows"`
	DocsRoot        string            `json:"docs_root"`
	Registry        string            `json:"registry"`
	Destinations    [2]DocDestination `json:"destinations"`
	Page            string            `json:"page"`
	Claims          string            `json:"claims"`
	ReceiptPath     string            `json:"receipt_path,omitempty"`
	PreviousReceipt string            `json:"previous_receipt,omitempty"`
	Adopt           bool              `json:"adopt"`
	Noop            bool              `json:"noop"`
	Digest          string            `json:"digest"`
}

type DocMaintenanceReceipt struct {
	Schema               string                             `json:"schema"`
	Profile              string                             `json:"profile"`
	Revision             string                             `json:"revision"`
	Tree                 string                             `json:"tree"`
	Sources              []DocIdentity                      `json:"sources"`
	Evidence             []DocIdentity                      `json:"evidence"`
	Flows                string                             `json:"flows"`
	DocsRoot             string                             `json:"docs_root"`
	Registry             string                             `json:"registry"`
	Outputs              [2]DocIdentity                     `json:"outputs"`
	Proposal             string                             `json:"proposal"`
	PreviousReceipt      string                             `json:"previous_receipt,omitempty"`
	HistoricalProvenance string                             `json:"historical_provenance"`
	Ownership            string                             `json:"ownership"`
	Publications         []doccorpus.MaintenancePublication `json:"publications"`
	Digest               string                             `json:"digest"`
}

type docMaintenancePlan struct {
	proposal DocProposal
	before   [2][]byte
	previous []byte
	receipt  DocMaintenanceReceipt
}

func PreviewDocMaintenance(ctx context.Context, root string, o DocMaintenanceOptions) (DocProposal, error) {
	p, err := planDocMaintenance(ctx, root, o)
	return p.proposal, err
}

// ApplyDocMaintenance always rederives. No serialized replacement bytes are
// accepted. Receipt bytes exist only after both publications succeed.
func ApplyDocMaintenance(ctx context.Context, root string, o DocMaintenanceOptions, expected string) ([]byte, error) {
	if !sha256Pattern.MatchString(expected) {
		return nil, errors.New("expected proposal must be a SHA-256")
	}
	p, err := planDocMaintenance(ctx, root, o)
	if err != nil {
		return nil, err
	}
	if p.proposal.Digest != expected {
		return nil, errors.New("maintenance proposal changed; preview again")
	}
	if p.proposal.Noop {
		return p.previous, nil
	}
	files := [2]doccorpus.MaintenanceFile{
		{Path: o.Docs.Page, Before: p.before[0], Next: []byte(p.proposal.Page)},
		{Path: o.Docs.Claims, Before: p.before[1], Next: []byte(p.proposal.Claims)},
	}
	var receipt []byte
	_, err = publishMaintenancePair(ctx, root, files, func(publications []doccorpus.MaintenancePublication) error {
		current, e := planDocMaintenance(ctx, root, o)
		if e != nil {
			return e
		}
		if current.proposal.Digest != expected {
			return errors.New("maintenance inputs changed during staging")
		}
		r := DocMaintenanceReceipt{Schema: DocMaintenanceSchema, Profile: docMaintenanceProfile, Revision: p.proposal.Revision, Tree: p.proposal.Tree,
			Sources: p.proposal.Sources, Evidence: p.proposal.Evidence, Flows: o.Flows, DocsRoot: o.Docs.DocsRoot, Registry: o.Docs.Registry,
			Outputs:  [2]DocIdentity{{Path: o.Docs.Page, SHA256: Digest(files[0].Next)}, {Path: o.Docs.Claims, SHA256: Digest(files[1].Next)}},
			Proposal: expected, PreviousReceipt: p.proposal.PreviousReceipt, HistoricalProvenance: "RECORDED", Ownership: "operator-selected; not authenticated or accepted intent", Publications: publications}
		if o.Adopt {
			r.HistoricalProvenance = "UNKNOWN"
		}
		receipt, e = encodeMaintenanceReceipt(r)
		return e
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func maintenanceDigest(v any) (string, error) {
	raw, err := doccorpus.Encode(v)
	if err != nil {
		return "", err
	}
	return Digest(raw), nil
}

func encodeMaintenanceReceipt(r DocMaintenanceReceipt) ([]byte, error) {
	var err error
	r.Digest = ""
	r.Digest, err = maintenanceDigest(r)
	if err != nil {
		return nil, err
	}
	raw, err := doccorpus.Encode(r)
	if err != nil {
		return nil, err
	}
	var decoded DocMaintenanceReceipt
	if err = Decode(raw, &decoded); err != nil {
		return nil, err
	}
	return raw, nil
}

func planDocMaintenance(ctx context.Context, root string, o DocMaintenanceOptions) (p docMaintenancePlan, err error) {
	if err = validDocOptions(o.Docs); err != nil {
		return p, err
	}
	for _, name := range []string{o.Docs.Page, o.Docs.Claims, o.Receipt, o.Docs.Registry} {
		if name != "" && (!safePath(name) || name == "." || gitPath(name)) {
			return p, errors.New("unsafe maintenance path")
		}
	}
	if strings.EqualFold(o.Docs.Page, o.Docs.Claims) || (o.Receipt != "" && (strings.EqualFold(o.Receipt, o.Docs.Page) || strings.EqualFold(o.Receipt, o.Docs.Claims) || o.Adopt)) {
		return p, errors.New("duplicate maintenance path or adoption with receipt")
	}
	set, err := LoadIntentsAt(ctx, root, o.Flows, "HEAD")
	if err != nil {
		return p, err
	}
	tree, err := git(ctx, root, "rev-parse", "--verify", set.Revision+"^{tree}")
	if err != nil {
		return p, err
	}
	p.proposal = DocProposal{Schema: "flow-doc-maintenance-proposal/0", Profile: docMaintenanceProfile, Revision: set.Revision, Tree: strings.TrimSpace(string(tree)), Flows: o.Flows, DocsRoot: o.Docs.DocsRoot, Registry: o.Docs.Registry, Adopt: o.Adopt, ReceiptPath: o.Receipt, Sources: []DocIdentity{}, Evidence: []DocIdentity{}}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return p, err
	}
	defer repository.Close()
	var infos [2]os.FileInfo
	for i, name := range []string{o.Docs.Page, o.Docs.Claims} {
		raw, info, e := maintenanceFile(repository, name, true)
		if e != nil {
			return p, e
		}
		p.before[i], infos[i] = raw, info
		p.proposal.Destinations[i] = DocDestination{Path: name, Exists: info != nil}
		if info != nil {
			p.proposal.Destinations[i].SHA256 = Digest(raw)
		}
	}
	if (infos[0] == nil) != (infos[1] == nil) {
		return p, errors.New("mixed maintenance destination existence")
	}
	if infos[0] != nil && os.SameFile(infos[0], infos[1]) {
		return p, errors.New("aliased maintenance destinations")
	}
	if infos[0] == nil && (o.Adopt || o.Receipt != "") {
		return p, errors.New("new outputs cannot adopt or name a previous receipt")
	}
	if err = cleanMaintenancePaths(ctx, root, []string{o.Docs.Page, o.Docs.Claims}); err != nil {
		return p, err
	}
	if infos[0] != nil {
		// A clean ignored/untracked file is not a committed output.
		for i, name := range []string{o.Docs.Page, o.Docs.Claims} {
			raw, e := committedFile(ctx, root, set.Revision, name)
			if e != nil || raw == nil || !bytes.Equal(raw, p.before[i]) {
				return p, errors.New("maintenance outputs must match committed bytes")
			}
		}
		var eligible DocClaims
		if !bytes.HasPrefix(p.before[0], []byte(docPageHeader+"\n# Application flows\n")) || Decode(p.before[1], &eligible) != nil || eligible.Schema != DocClaimsSchema || eligible.Page != o.Docs.Page || !maintenanceClaimsValid(eligible) {
			return p, errors.New("maintenance refuses authored or invalid generated outputs")
		}
		if o.Receipt == "" {
			if !o.Adopt {
				return p, errors.New("existing outputs need a committed maintenance receipt or explicit --adopt")
			}
			var claims DocClaims
			if !bytes.HasPrefix(p.before[0], []byte(docPageHeader+"\n# Application flows\n")) || Decode(p.before[1], &claims) != nil || claims.Schema != DocClaimsSchema || claims.Page != o.Docs.Page || !maintenanceClaimsValid(claims) {
				return p, errors.New("adoption requires a generated-shaped page and valid claims; historical provenance UNKNOWN")
			}
		} else {
			raw, info, e := maintenanceFile(repository, o.Receipt, false)
			if e != nil {
				return p, e
			}
			if os.SameFile(info, infos[0]) || os.SameFile(info, infos[1]) {
				return p, errors.New("aliased maintenance receipt")
			}
			committed, e := committedFile(ctx, root, set.Revision, o.Receipt)
			if e != nil || !bytes.Equal(committed, raw) {
				return p, errors.New("maintenance receipt must match committed bytes")
			}
			if e = cleanMaintenancePaths(ctx, root, []string{o.Receipt}); e != nil {
				return p, e
			}
			if e = Decode(raw, &p.receipt); e != nil {
				return p, e
			}
			r := p.receipt
			digest := r.Digest
			r.Digest = ""
			canonical, canonicalErr := doccorpus.Encode(p.receipt)
			expectedDigest, digestErr := maintenanceDigest(r)
			if canonicalErr != nil || digestErr != nil || r.Schema != DocMaintenanceSchema || r.Profile != docMaintenanceProfile || !sha256Pattern.MatchString(digest) || digest != expectedDigest || !bytes.Equal(raw, canonical) || !oidPattern.MatchString(r.Revision) || !oidPattern.MatchString(r.Tree) || !sha256Pattern.MatchString(r.Proposal) || r.Ownership != "operator-selected; not authenticated or accepted intent" || (r.HistoricalProvenance != "RECORDED" && r.HistoricalProvenance != "UNKNOWN") {
				return p, errors.New("invalid maintenance receipt integrity or profile")
			}
			for i, name := range []string{o.Docs.Page, o.Docs.Claims} {
				if r.Outputs[i].Path != name || r.Outputs[i].SHA256 != Digest(p.before[i]) {
					return p, errors.New("maintenance receipt output mismatch")
				}
			}
			p.previous = raw
			p.proposal.PreviousReceipt = Digest(raw)
		}
	}
	sources, paths, err := maintenanceSources(ctx, root, set, o)
	if err != nil {
		return p, err
	}
	if err = cleanMaintenancePaths(ctx, root, paths); err != nil {
		return p, err
	}
	p.proposal.Sources = sources
	for _, name := range o.EvidenceFiles {
		raw, e := ReadFile(name)
		if e != nil {
			return p, e
		}
		p.proposal.Evidence = append(p.proposal.Evidence, DocIdentity{Path: name, SHA256: Digest(raw)})
	}
	o.Docs.Evidence, err = readRunEvidence(o.EvidenceFiles)
	if err != nil {
		return p, err
	}
	// Detect raw evidence changes across decode, including changes that decode to equal records.
	for _, e := range p.proposal.Evidence {
		raw, e2 := ReadFile(e.Path)
		if e2 != nil || Digest(raw) != e.SHA256 {
			return p, errors.New("maintenance evidence changed during read")
		}
	}
	page, claims, err := RenderDocs(ctx, root, set, o.Docs)
	if err != nil {
		return p, err
	}
	if len(page) > doccorpus.MaxBytes || len(claims) > doccorpus.MaxBytes {
		return p, errors.New("maintenance output exceeds byte bound")
	}
	p.proposal.Page, p.proposal.Claims = string(page), string(claims)
	if len(p.previous) > 0 && bytes.Equal(page, p.before[0]) && bytes.Equal(claims, p.before[1]) && p.receipt.Flows == o.Flows && p.receipt.DocsRoot == o.Docs.DocsRoot && p.receipt.Registry == o.Docs.Registry && slices.Equal(p.receipt.Sources, sources) && slices.Equal(p.receipt.Evidence, p.proposal.Evidence) {
		p.proposal.Noop = true
	}
	now, e := ResolveRevision(ctx, root, "HEAD")
	if e != nil || now != set.Revision {
		return p, errors.New("maintenance HEAD moved during derivation")
	}
	p.proposal.Digest, err = maintenanceDigest(p.proposal)
	if err != nil {
		return p, err
	}
	if _, err = doccorpus.Encode(p.proposal); err != nil {
		p.proposal.Digest = ""
		return p, err
	}
	return p, nil
}

func maintenanceFile(root *os.Root, name string, absent bool) ([]byte, os.FileInfo, error) {
	if err := realParents(root, name); err != nil {
		return nil, nil, err
	}
	info, err := root.Lstat(name)
	if absent && os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil, errors.New("maintenance input must be a regular file")
	}
	raw, err := testvaliditydoc.ReadFile(root, name)
	return raw, info, err
}

func cleanMaintenancePaths(ctx context.Context, root string, paths []string) error {
	args := append([]string{"--literal-pathspecs", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--"}, paths...)
	raw, err := git(ctx, root, args...)
	if err != nil {
		return err
	}
	if len(raw) != 0 {
		return errors.New("maintenance inputs or outputs are dirty; commit or restore them first")
	}
	return nil
}

func maintenanceSources(ctx context.Context, root string, set IntentSet, o DocMaintenanceOptions) ([]DocIdentity, []string, error) {
	paths := []string{set.Dir + "/"}
	for _, f := range set.Flows {
		for _, l := range f.Links {
			if l.Target.Path != "" {
				paths = append(paths, l.Target.Path)
			}
		}
	}
	if o.Docs.Registry != "" {
		paths = append(paths, o.Docs.Registry)
	}
	if o.Docs.DocsRoot != "" {
		docs, err := committedMarkdown(ctx, root, set.Revision, o.Docs.DocsRoot, o.Docs.Page)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range docs {
			if d.path != o.Receipt {
				paths = append(paths, d.path)
			}
		}
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer repository.Close()
	sources := []DocIdentity{}
	seen := map[string]bool{}
	for _, name := range paths {
		if name == o.Docs.Page || name == o.Docs.Claims || name == o.Receipt {
			return nil, nil, errors.New("maintenance output or receipt aliases generation input")
		}
		entries, err := docTree(ctx, root, set.Revision, name)
		if err != nil {
			return nil, nil, err
		}
		if len(entries) == 0 {
			if err = maintenanceAbsent(repository, name); err != nil {
				return nil, nil, err
			}
			sources = append(sources, DocIdentity{Path: name, SHA256: "ABSENT"})
		}
		for _, entry := range entries {
			if seen[entry.name] {
				continue
			}
			seen[entry.name] = true
			if entry.name == o.Docs.Page || entry.name == o.Docs.Claims || entry.name == o.Receipt {
				return nil, nil, errors.New("maintenance destination inside generation inputs")
			}
			raw, err := committedBlob(ctx, root, entry)
			if err != nil {
				return nil, nil, err
			}
			live, info, e := maintenanceFile(repository, entry.name, false)
			if e != nil || !bytes.Equal(live, raw) {
				return nil, nil, errors.New("maintenance generation input differs from committed bytes")
			}
			for _, dest := range []string{o.Docs.Page, o.Docs.Claims, o.Receipt} {
				if dest == "" {
					continue
				}
				di, e := repository.Lstat(dest)
				if e == nil && os.SameFile(info, di) {
					return nil, nil, errors.New("maintenance destination aliases generation input")
				}
			}
			sources = append(sources, DocIdentity{Path: entry.name, Blob: entry.oid, SHA256: Digest(raw)})
			if len(sources) > 4096 {
				return nil, nil, errors.New("maintenance source count exceeded")
			}
		}
	}
	slices.SortFunc(sources, func(a, b DocIdentity) int { return strings.Compare(a.Path, b.Path) })
	return sources, paths, nil
}

func maintenanceClaimsValid(c DocClaims) bool {
	if c.Claims == nil {
		return false
	}
	seen := map[string]bool{}
	for _, claim := range c.Claims {
		if claim.ID == "" || seen[claim.ID] || claim.Flow == "" || claim.Variation == "" || claim.Outcome == "" || !safePath(claim.Source) || claim.Evidence == nil {
			return false
		}
		switch claim.State {
		case ClaimProven, ClaimStale, ClaimUnproven, ClaimContradicted:
		default:
			return false
		}
		seen[claim.ID] = true
	}
	return true
}

// Check each component without following symlinks. A missing parent establishes
// absence; a symlink, non-directory parent or other read failure never does.
func maintenanceAbsent(repository *os.Root, name string) error {
	current, err := repository.OpenRoot(".")
	if err != nil {
		return err
	}
	defer func() { _ = current.Close() }()
	components := strings.Split(strings.TrimSuffix(name, "/"), "/")
	for i, component := range components {
		before, e := current.Lstat(component)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return errors.New("cannot establish absent maintenance input")
		}
		if i == len(components)-1 || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return errors.New("committed-absent maintenance input exists or has unsafe parents")
		}
		next, e := current.OpenRoot(component)
		if e != nil {
			return e
		}
		opened, e := next.Stat(".")
		if e != nil || !os.SameFile(before, opened) {
			_ = next.Close()
			return errors.New("absent maintenance input parent changed")
		}
		_ = current.Close()
		current = next
	}
	return errors.New("cannot establish absent maintenance input")
}

// Package-private test seams: tests replace these to interleave a competing
// change between derivation steps. Production always uses the real functions.
var (
	publishMaintenancePair = doccorpus.PublishMaintenancePair
	readRunEvidence        = ReadRunEvidence
)
