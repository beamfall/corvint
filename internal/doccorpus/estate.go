// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// EstateSchema names the estate input: the corpus artifacts one estate receipt reconciles
// (DCI-V1-001).
const EstateSchema = "corvint-corpus-estate/1"

// EstateReceiptSchema names the estate receipt (DCI-V1-002).
const EstateReceiptSchema = "corvint-corpus-estate-receipt/1"

// maxEstateCorpora bounds one estate. Each corpus keeps its own eight-revision bound
// (DCP-V1-175); the estate only reads verified artifacts one at a time and keeps their
// inventory records, so it raises no per-corpus bound (DCI-V1-001).
const maxEstateCorpora = 64

// maxEstateTargets bounds the join targets of all the receipt's references together. Each
// reference copies every record that declares its name, so repeated names would otherwise grow
// the receipt multiplicatively; the budget is checked before any reference is expanded.
const maxEstateTargets = 16384

// EstateInput is the estate file `docs corpus estate --input` reads.
type EstateInput struct {
	Schema  string        `json:"schema"`
	Corpora []EstateEntry `json:"corpora"`
}

// EstateEntry names one corpus: the primary checkout it was built from, as a clean relative path
// from --root that may climb with "../" ("." for --root itself, as workspace roots may), and its
// artifact path relative to that checkout. Its additional
// repositories resolve from that checkout as in any read.
type EstateEntry struct {
	Root     string `json:"root"`
	Artifact string `json:"artifact"`
}

func (e EstateEntry) label() string { return path.Join(e.Root, e.Artifact) }

// EstateCorpus is one verified corpus of an estate.
type EstateCorpus struct {
	Artifact       string `json:"artifact"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Repository     string `json:"repository"`
	Revision       string `json:"revision"`
	Inventory      bool   `json:"inventory"`
}

// EstateRepository is one repository some corpus of the estate covers, keyed by its root-commit
// identity, with every name and pinned revision the covering corpora give it.
type EstateRepository struct {
	ID        string   `json:"id"`
	Names     []string `json:"names"`
	Revisions []string `json:"revisions"`
	Corpora   []string `json:"corpora"`
}

// EstateTarget is one corpus record a reference joins by exact name.
type EstateTarget struct {
	Corpus     string `json:"corpus"`
	Repository string `json:"repository"`
	Subject    string `json:"subject"`
}

// EstateReference is one reference a corpus left outside itself, reconciled across the estate.
type EstateReference struct {
	Corpus    string `json:"corpus"`
	Claim     string `json:"claim"`
	Owner     string `json:"owner"`
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	State     string `json:"state"`
	// Grade is the corpus's own grading of the unjoined reference: first-party under a declared
	// prefix, else ungraded (DCI-V1-010).
	Grade   string         `json:"grade"`
	Targets []EstateTarget `json:"targets"`
	Reason  string         `json:"reason"`
}

// EstateReceipt is the query-time reconciliation of several verified corpora (DCI-V1-002). It
// is derived from the artifacts it names and is never an input to any corpus.
type EstateReceipt struct {
	Schema       string             `json:"schema"`
	Corpora      []EstateCorpus     `json:"corpora"`
	Repositories []EstateRepository `json:"repositories"`
	References   []EstateReference  `json:"references"`
	Unknown      []string           `json:"unknown"`
	Limitations  []string           `json:"limitations"`
}

// ParseEstate admits an estate file: the schema and 1 to maxEstateCorpora distinct corpora with
// canonical relative paths, sorted so the receipt does not depend on their order (DCI-V1-001).
func ParseEstate(data []byte) (EstateInput, error) {
	var in EstateInput
	// One whole closed document: trailing data, a repeated member and an unknown member refuse.
	if err := json.Unmarshal(data, &in, json.RejectUnknownMembers(true)); err != nil || in.Schema != EstateSchema {
		return EstateInput{}, fail("estate file must be one " + EstateSchema + " object with schema and corpora")
	}
	if len(in.Corpora) == 0 {
		return EstateInput{}, fail("estate names no corpus")
	}
	if len(in.Corpora) > maxEstateCorpora {
		return EstateInput{}, capacityExceeded("estate corpus count", len(in.Corpora), maxEstateCorpora)
	}
	in.Corpora = slices.Clone(in.Corpora)
	sort.Slice(in.Corpora, func(i, j int) bool { return in.Corpora[i].label() < in.Corpora[j].label() })
	for i, e := range in.Corpora {
		if !estateRoot(e.Root) || LocalInputPathRule(e.Artifact) != "" {
			return EstateInput{}, fail("estate corpus " + e.label() + " needs a canonical relative root (or \".\") and artifact path")
		}
		if i > 0 && e.label() == in.Corpora[i-1].label() {
			return EstateInput{}, fail("estate corpus " + e.label() + " is repeated")
		}
	}
	return in, nil
}

// estateRoot admits a clean relative slash path: ".", a local path, or one that climbs with "../".
func estateRoot(root string) bool {
	climbed := root
	for strings.HasPrefix(climbed, "../") {
		climbed = strings.TrimPrefix(climbed, "../")
	}
	return root == "." || root == path.Clean(root) && !strings.ContainsAny(root, "\\\x00") && (climbed == ".." || LocalInputPathRule(climbed) == "")
}

// namedRefusal prefixes a typed refusal's message with what it is about, keeping its code.
func namedRefusal(prefix string, err error) error {
	var de *Error
	if errors.As(err, &de) {
		return &Error{Code: de.Code, Message: prefix + ": " + de.Message}
	}
	var ce *contextindex.Error
	if errors.As(err, &ce) {
		return &contextindex.Error{Code: ce.Code, Message: prefix + ": " + ce.Message, Cause: err}
	}
	return err
}

// estatePublisher maps an inventory rule to the ecosystem its records publish under.
var estatePublisher = map[string]string{"package-json/": "npm", "go-mod/": "go", "helm-chart/": "chart", "gemspec/": "gem"}

// estateLabels inverts the reference label of reconcile (DCP-V1-704).
var estateLabels = map[string]string{"container image": "image", "Helm chart": "chart", "GitHub action or reusable workflow": "action", "npm package": "npm", "Go module": "go", "Ruby gem": "gem"}

type estateReference struct {
	EstateReference
	key string
}

// Estate reads each artifact through the verified read and reconciles their inventories
// (DCI-V1-002, DCI-V1-003). A refused artifact refuses the estate, naming its path.
func Estate(ctx context.Context, root string, in EstateInput) (EstateReceipt, error) {
	return estate(ctx, root, in, maxEstateTargets)
}

func estate(ctx context.Context, root string, in EstateInput, targetBudget int) (EstateReceipt, error) {
	receipt := EstateReceipt{Schema: EstateReceiptSchema, Corpora: []EstateCorpus{}, Repositories: []EstateRepository{}, References: []EstateReference{}, Unknown: []string{}, Limitations: []string{
		"joins are exact names of declared packages, charts and aliases across verified corpora; version compatibility and run-time use are unknown",
		"each corpus is reconciled at its own pinned revisions; the corpora need not share a time",
	}}
	repositories := map[string]*EstateRepository{}
	published := map[string]map[string][]EstateTarget{}
	refs := []estateReference{}
	for _, entry := range in.Corpora {
		label := entry.label()
		a, err := OpenReadFile(ctx, filepath.Join(root, filepath.FromSlash(entry.Root)), entry.Artifact, ReadOptions{})
		if err != nil {
			return EstateReceipt{}, namedRefusal("estate corpus "+label, err)
		}
		m := a.Manifest
		receipt.Corpora = append(receipt.Corpora, EstateCorpus{label, a.ManifestSHA256, m.Repository.ID, m.Repository.Revision, m.RepositoryInventory != nil})
		declared := m.inventoryDeclarations()
		native := nativeProviderID(m)
		for _, r := range m.coveredRepositories() {
			e := repositories[r.id]
			if e == nil {
				e = &EstateRepository{ID: r.id}
				repositories[r.id] = e
			}
			name, _ := repositoryName(r, declared[r.id])
			e.Names = append(e.Names, name)
			if r.alias != "" {
				e.Names = append(e.Names, r.alias)
			}
			e.Names = append(e.Names, declared[r.id].Aliases...)
			e.Revisions = append(e.Revisions, r.revision)
			e.Revisions = append(e.Revisions, r.revisions...)
			e.Corpora = append(e.Corpora, label)
			if m.RepositoryInventory != nil {
				for _, alias := range declared[r.id].Aliases {
					publishEstate(published, "alias", alias, EstateTarget{label, r.id, repositorySubjectID(native, r.id)})
				}
			}
		}
		if m.RepositoryInventory == nil {
			receipt.Unknown = append(receipt.Unknown, "corpus "+label+" declares no repository_inventory; its packages and references are NOT_OBSERVED")
			continue
		}
		for _, s := range a.Subjects {
			if s.Provider != native || s.Kind != "package" && s.Kind != "package_manifest" && s.Kind != "deployable_unit" || len(s.Evidence.Anchors) == 0 {
				continue
			}
			rule := strings.TrimPrefix(s.Evidence.Anchors[0].Reason, "repository inventory rule ")
			for prefix, ecosystem := range estatePublisher {
				if strings.HasPrefix(rule, prefix) {
					publishEstate(published, ecosystem, s.Name, EstateTarget{label, s.Evidence.Anchors[0].Repository, s.ID})
				}
			}
		}
		outside, unknown := estateOutside(label, a)
		receipt.Unknown = append(receipt.Unknown, unknown...)
		for _, c := range a.Claims {
			grade := outside[c.ID]
			if grade == "" {
				continue
			}
			ecosystem, name, ok := parseEstateReference(native, c)
			if !ok {
				receipt.Unknown = append(receipt.Unknown, "corpus "+label+" reference "+c.ID+" is not in the reference claim form; it is not reconciled")
				continue
			}
			key, target := ecosystem, name
			switch ecosystem {
			case "image":
				key, target = "alias", imageRepository(name)
			case "action":
				key = "alias"
			}
			refs = append(refs, estateReference{EstateReference{Corpus: label, Claim: c.ID, Owner: c.Subject, Ecosystem: ecosystem, Name: name, Grade: grade}, key + "\x00" + target})
		}
	}
	total := 0
	for _, ref := range refs {
		ecosystem, name, _ := strings.Cut(ref.key, "\x00")
		if total += len(published[ecosystem][name]); total > targetBudget {
			return EstateReceipt{}, namedRefusal("estate corpus "+ref.Corpus+" reference "+ref.Claim, capacityExceeded("estate reference target count", total, targetBudget))
		}
	}
	for _, ref := range refs {
		ecosystem, name, _ := strings.Cut(ref.key, "\x00")
		targets := published[ecosystem][name]
		owners := map[string]bool{}
		for _, t := range targets {
			owners[t.Repository] = true
		}
		ref.Targets = slices.Clone(targets)
		if ref.Targets == nil {
			ref.Targets = []EstateTarget{}
		}
		switch {
		case len(owners) == 0:
			ref.State, ref.Reason = "outside-estate", "no corpus of the estate declares "+name+", so its source repository is outside the estate or undeclared (NOT_OBSERVED)"
		case len(owners) == 1:
			ref.State, ref.Reason = "resolved", "exact name join to a declared package, chart or alias of another corpus; version compatibility unknown"
		default:
			ref.State, ref.Reason = "ambiguous", "several covered repositories of the estate declare "+name+"; no join is chosen"
		}
		receipt.References = append(receipt.References, ref.EstateReference)
	}
	for _, id := range slices.Sorted(maps.Keys(repositories)) {
		e := repositories[id]
		e.Names, e.Revisions, e.Corpora = estateSet(e.Names), estateSet(e.Revisions), estateSet(e.Corpora)
		receipt.Repositories = append(receipt.Repositories, *e)
	}
	sort.Slice(receipt.References, func(i, j int) bool {
		a, b := receipt.References[i], receipt.References[j]
		return a.Corpus < b.Corpus || a.Corpus == b.Corpus && a.Claim < b.Claim
	})
	return receipt, nil
}

// estateOutside grades the reference claims a corpus left outside itself. A reference gap's
// reason is the claim's ID, ": ", its text and "; ", and its subject is the claim's owner, so a gap
// is matched to its claim by those fields, never by splitting the reason at a separator a path may
// hold. A gap that names no reference claim is an unknown entry, never dropped (DCI-V1-003).
func estateOutside(label string, a *Artifact) (map[string]string, []string) {
	claims := map[string][]Claim{}
	for _, c := range a.Claims {
		claims[c.Subject] = append(claims[c.Subject], c)
	}
	outside, unknown := map[string]string{}, []string{}
	for _, g := range a.Gaps {
		grade := map[string]string{"outside-corpus-reference": "ungraded", "first-party-reference-outside-corpus": "first-party"}[g.Kind]
		if grade == "" {
			continue
		}
		matched := false
		for _, c := range claims[g.Subject] {
			if strings.HasPrefix(g.Reason, c.ID+": "+c.Text+"; ") {
				outside[c.ID], matched = grade, true
				break
			}
		}
		if !matched {
			unknown = append(unknown, "corpus "+label+" "+g.Kind+" gap on "+g.Subject+" names no reference claim; it is not reconciled")
		}
	}
	return outside, unknown
}

func publishEstate(published map[string]map[string][]EstateTarget, ecosystem, name string, t EstateTarget) {
	if published[ecosystem] == nil {
		published[ecosystem] = map[string][]EstateTarget{}
	}
	if !slices.Contains(published[ecosystem][name], t) {
		published[ecosystem][name] = append(published[ecosystem][name], t)
		sort.Slice(published[ecosystem][name], func(i, j int) bool {
			a, b := published[ecosystem][name][i], published[ecosystem][name][j]
			return a.Corpus+"\x00"+a.Subject < b.Corpus+"\x00"+b.Subject
		})
	}
}

// parseEstateReference recovers the ecosystem and name of a reference claim
// "consumes LABEL NAME at LOCATION", accepting a split only when it rebuilds the claim ID
// NATIVE:reference:LOCATION:ECOSYSTEM:NAME, because a name or location may hold " at ".
func parseEstateReference(native string, c Claim) (string, string, bool) {
	for label, ecosystem := range estateLabels {
		rest, ok := strings.CutPrefix(c.Text, "consumes "+label+" ")
		if !ok {
			continue
		}
		for i := strings.Index(rest, " at "); i >= 0; {
			name, location := rest[:i], rest[i+len(" at "):]
			if c.ID == native+":reference:"+location+":"+ecosystem+":"+name {
				return ecosystem, name, true
			}
			next := strings.Index(rest[i+1:], " at ")
			if next < 0 {
				break
			}
			i += 1 + next
		}
	}
	return "", "", false
}

func estateSet(values []string) []string {
	out := slices.Compact(slices.Sorted(slices.Values(values)))
	if out == nil {
		out = []string{}
	}
	return out
}
