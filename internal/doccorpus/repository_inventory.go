package doccorpus

import (
	"cmp"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// RepositoryInventory is a manifest's optional repository inventory declaration (DCP-V1-700).
// Absent, the manifest and its artifact keep their bytes; present, even empty, the build derives
// repository, deployable-unit and package subjects and the reconciliation gaps from the declared
// inputs (DCP-V1-702 to DCP-V1-705).
type RepositoryInventory struct {
	Repositories []RepositoryDeclaration `json:"repositories,omitempty"`
	// FirstParty lists the name prefixes the project owns, such as an npm scope, an image
	// registry path or a gem name prefix; an unjoined reference under one is graded first-party
	// (DCI-V1-010).
	FirstParty []string `json:"first_party,omitempty"`
	// EnvironmentWords extends the closed environment vocabulary a deploy job's name is read
	// with (DCI-V1-028).
	EnvironmentWords []string `json:"environment_words,omitempty"`
	// DeployJobs declares the deployment environment of named CI jobs (DCI-V1-029).
	DeployJobs []DeployJobDeclaration `json:"deploy_jobs,omitempty"`
}

// DeployJobDeclaration is the project's own statement that one CI job of a covered repository
// deploys to an environment (DCI-V1-029). Repository is a root-commit identity or workspace
// alias, Path the workflow or CircleCI file and Job the job key as written.
type DeployJobDeclaration struct {
	Repository  string `json:"repository"`
	Path        string `json:"path"`
	Job         string `json:"job"`
	Environment string `json:"environment"`
}

// RepositoryDeclaration is the project's own statement about one covered repository: the
// repository's root-commit identity or workspace alias, a display name, further names it is
// known by (such as a package or image repository name) and whether it is archived. It grants
// no identity; the repository must be declared by the manifest.
type RepositoryDeclaration struct {
	Repository string   `json:"repository"`
	Name       string   `json:"name,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Archived   bool     `json:"archived,omitempty"`
}

// RepositoryListing is one covered repository in an info or validate result (DCP-V1-701).
type RepositoryListing struct {
	ID              string   `json:"id"`
	Role            string   `json:"role"`
	Name            string   `json:"name"`
	NameSource      string   `json:"name_source"`
	Alias           string   `json:"alias,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	Revision        string   `json:"revision"`
	Revisions       []string `json:"revisions,omitempty"`
	Root            string   `json:"root,omitempty"`
	Freshness       string   `json:"freshness"`
	FreshnessReason string   `json:"freshness_reason,omitempty"`
	Archived        string   `json:"archived"`
	ArchivedReason  string   `json:"archived_reason"`
}

const (
	maxInventoryAliases    = 16
	maxInventoryName       = 128
	maxInventoryFirstParty = 64
	maxInventoryWords      = 64
	maxInventoryDeployJobs = 256
)

var inventoryAlias = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:-]{0,127}$`)

// inventoryPrefix is a first-party name prefix; it may start with an npm scope's @ (DCI-V1-010).
var inventoryPrefix = regexp.MustCompile(`^[A-Za-z0-9@][A-Za-z0-9._/@:-]{0,127}$`)

// inventoryWord is a declared environment word, one lower-case token of the job-name reader
// (DCI-V1-028); inventoryJob is a declared CI job key and environment name (DCI-V1-029).
var (
	inventoryWord = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	inventoryJob  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// coveredRepository is one repository the manifest declares, primary first.
type coveredRepository struct {
	id, revision, alias, root, role string
	revisions                       []string
}

func (m Manifest) coveredRepositories() []coveredRepository {
	out := []coveredRepository{{id: m.Repository.ID, revision: m.Repository.Revision, role: "primary"}}
	for _, r := range m.AdditionalRepositories {
		out = append(out, coveredRepository{id: r.ID, revision: r.Revision, alias: r.Alias, root: r.Root, role: "additional", revisions: r.Revisions})
	}
	return out
}

// inventoryDeclarations indexes the declarations by root-commit identity.
func (m Manifest) inventoryDeclarations() map[string]RepositoryDeclaration {
	out := map[string]RepositoryDeclaration{}
	if m.RepositoryInventory != nil {
		for _, d := range m.RepositoryInventory.Repositories {
			out[m.repositoryOfAlias(d.Repository)] = d
		}
	}
	return out
}

// validateRepositoryInventory admits the declaration only with a native provider to own its
// records, at most one declaration per declared repository in repository order, a bounded name and
// sorted distinct aliases no other declaration uses (DCP-V1-700).
func validateRepositoryInventory(m Manifest) error {
	inv := m.RepositoryInventory
	if inv == nil {
		return nil
	}
	if nativeProviderID(m) == "" {
		return fail("repository inventory requires a native provider")
	}
	if inv.Repositories != nil && len(inv.Repositories) == 0 || len(inv.Repositories) > maxAdditionalRepositories+1 {
		return fail("invalid repository inventory declaration")
	}
	seen, aliases := map[string]bool{}, map[string]bool{}
	for i, d := range inv.Repositories {
		id := m.repositoryOfAlias(d.Repository)
		if !m.declaresRepository(id) || seen[id] || i > 0 && d.Repository <= inv.Repositories[i-1].Repository {
			return fail("invalid repository inventory declaration: " + d.Repository + " is undeclared, repeated or out of order")
		}
		seen[id] = true
		if d.Name != "" && (len(d.Name) > maxInventoryName || !textOK(d.Name) || strings.ContainsAny(d.Name, "\r\n\t")) {
			return fail("invalid repository inventory declaration: name of " + d.Repository)
		}
		if d.Aliases != nil && (len(d.Aliases) == 0 || len(d.Aliases) > maxInventoryAliases || !sort.StringsAreSorted(d.Aliases)) {
			return fail("invalid repository inventory declaration: aliases of " + d.Repository)
		}
		for _, alias := range d.Aliases {
			if !inventoryAlias.MatchString(alias) || aliases[alias] {
				return fail("invalid repository inventory declaration: alias " + alias)
			}
			aliases[alias] = true
		}
	}
	if inv.FirstParty != nil && (len(inv.FirstParty) == 0 || len(inv.FirstParty) > maxInventoryFirstParty || !sort.StringsAreSorted(inv.FirstParty)) {
		return fail("invalid repository inventory declaration: first_party")
	}
	for i, prefix := range inv.FirstParty {
		if !inventoryPrefix.MatchString(prefix) || i > 0 && prefix == inv.FirstParty[i-1] {
			return fail("invalid repository inventory declaration: first_party prefix " + prefix)
		}
	}
	if inv.EnvironmentWords != nil && (len(inv.EnvironmentWords) == 0 || len(inv.EnvironmentWords) > maxInventoryWords || !sort.StringsAreSorted(inv.EnvironmentWords)) {
		return fail("invalid repository inventory declaration: environment_words")
	}
	for i, word := range inv.EnvironmentWords {
		if !inventoryWord.MatchString(word) || i > 0 && word == inv.EnvironmentWords[i-1] {
			return fail("invalid repository inventory declaration: environment word " + word)
		}
	}
	if inv.DeployJobs != nil && (len(inv.DeployJobs) == 0 || len(inv.DeployJobs) > maxInventoryDeployJobs) {
		return fail("invalid repository inventory declaration: deploy_jobs")
	}
	jobs := map[string]bool{}
	for i, d := range inv.DeployJobs {
		id := m.repositoryOfAlias(d.Repository)
		key := deployJobKey(id, d.Path, d.Job)
		if !m.declaresRepository(id) || !isWorkflow(d.Path) && d.Path != circleCIConfig || !inventoryJob.MatchString(d.Job) || !inventoryJob.MatchString(d.Environment) ||
			jobs[key] || i > 0 && deployJobKey(d.Repository, d.Path, d.Job) <= deployJobKey(inv.DeployJobs[i-1].Repository, inv.DeployJobs[i-1].Path, inv.DeployJobs[i-1].Job) {
			return fail("invalid repository inventory declaration: deploy job " + d.Repository + ":" + d.Path + ":" + d.Job + " is malformed, undeclared, repeated or out of order")
		}
		jobs[key] = true
	}
	return nil
}

// deployJobKey identifies one declared deploy job by repository root-commit identity, path and job.
func deployJobKey(repository, path, job string) string {
	return repository + "\x00" + path + "\x00" + job
}

func nativeProviderID(m Manifest) string {
	for _, p := range m.Providers {
		if p.Kind == "native" {
			return p.ID
		}
	}
	return ""
}

// repositoryName is a covered repository's declared name, its workspace alias, else its root-commit
// identity, with the source of that name.
func repositoryName(r coveredRepository, d RepositoryDeclaration) (string, string) {
	switch {
	case d.Name != "":
		return d.Name, "repository_inventory"
	case r.alias != "":
		return r.alias, "workspace_alias"
	}
	return r.id, "undeclared"
}

func repositorySubjectID(native, id string) string { return native + ":repository:" + id }

// repositoryListing lists every covered repository for info and validate (DCP-V1-701). states is
// the live overlay's state of each checkout; without one each repository is unknown.
func repositoryListing(a *Artifact, states map[string]string) []RepositoryListing {
	m := a.Manifest
	declared := m.inventoryDeclarations()
	out := []RepositoryListing{}
	for _, r := range m.coveredRepositories() {
		d := declared[r.id]
		name, source := repositoryName(r, d)
		l := RepositoryListing{ID: r.id, Role: r.role, Name: name, NameSource: source, Alias: r.alias, Aliases: d.Aliases, Revision: r.revision, Revisions: r.revisions, Root: r.root, Freshness: "unknown", Archived: "unknown", ArchivedReason: "archive state is a hosting fact Git does not record; NOT_OBSERVED unless repository_inventory declares it"}
		if state, ok := states[r.id]; ok {
			l.Freshness = state
		} else {
			l.FreshnessReason = "no live reader observed this checkout"
		}
		if d.Archived {
			l.Archived, l.ArchivedReason = "declared", "repository_inventory declares this repository archived"
		}
		out = append(out, l)
	}
	return out
}

// inventoryReference is one deployed or consumed name a declared input states.
type inventoryReference struct {
	ecosystem, name, owner string
	revision, path         string
	line                   int
}

// inventoryFile is one declared input the inventory rules read.
type inventoryFile struct {
	revision, path, qualifier, text string
	lines                           []string
}

type inventoryBuild struct {
	c                 *compiler
	native            string
	refs              []inventoryReference
	published         map[string]map[string]string // ecosystem -> name -> subject ID
	deployable, packs map[string]bool              // repository ID -> evidence seen
	publishJobs       map[string][]string          // repository ID -> release or publish jobs tied to no literal directory
	publishTies       map[string]map[string]string // repository ID -> directory -> the first job publishing it
	publishTied       map[string][]string          // repository ID -> jobs tied to a directory, with the directories they publish
	unsupportedCI     map[string]bool
	words             map[string]bool   // declared environment words (DCI-V1-028)
	deployJobs        map[string]string // deployJobKey -> declared environment (DCI-V1-029)
	declaredSeen      map[string]bool   // deployJobKey of each declared job a CI input holds              // repository ID -> an unsupported CI file seen
}

var (
	dockerFrom      = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?\s*$`)
	chartName       = regexp.MustCompile(`^name:\s*["']?([^"'\s#]+)`)
	chartDependency = regexp.MustCompile(`^(?:\s*-\s+|\s+)name:\s*["']?([^"'\s#]+)`)
	workflowJob     = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*(?:#.*)?$`)
	workflowUses    = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*["']?([^"'\s#]+)`)
	goModule        = regexp.MustCompile(`^module\s+"?([^"\s]+)"?\s*(?://.*)?$`)
	goRequire       = regexp.MustCompile(`^(?:require\s+)?\s*"?([^"\s()]+)"?\s+v\S*\s*(//.*)?$`)
	workflowEnv     = regexp.MustCompile(`^    environment:\s*["']?([^"'\s#]*)["']?\s*(?:#.*)?$`)
	workflowEnvKey  = regexp.MustCompile(`^    environment:`)
	workflowEnvName = regexp.MustCompile(`^      name:\s*["']?([^"'\s#]+)["']?\s*(?:#.*)?$`)
	environmentName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)
	// A Ruby name is read only from a whole string literal: a double-quoted one holds no "#"
	// (interpolation) or escape, and nothing but an argument list or a comment follows it, so an
	// expression such as "acme" + "-lib" or a method call on the literal is no name (DCI-V1-007).
	gemspecNameCall = regexp.MustCompile(`^\s*\w+\.name\s*=`)
	gemspecName     = regexp.MustCompile(`^\s*\w+\.name\s*=\s*` + rubyString + `\s*(?:#.*)?$`)
	gemspecDepCall  = regexp.MustCompile(`^\s*\w+\.add_(?:runtime_)?dependency\b`)
	gemspecDepend   = regexp.MustCompile(`^\s*\w+\.add_(?:runtime_)?dependency\s*\(?\s*` + rubyString + `\s*(?:[,)#].*)?$`)
	gemfileGemCall  = regexp.MustCompile(`^\s*gem[\s(]`)
	gemfileGem      = regexp.MustCompile(`^\s*gem\s*\(?\s*` + rubyString + `\s*(?:[,)#].*)?$`)
	gemNameRule     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

const rubyString = `(?:"([^"\\#]*)"|'([^'\\]*)')`

// rubyLiteral reads the literal name of a line that call matches: ok is false when the line is
// no such call, and name is "" when the call's argument is not one whole literal gem name.
func rubyLiteral(call, literal *regexp.Regexp, line string) (name string, ok bool) {
	if !call.MatchString(line) {
		return "", false
	}
	if m := literal.FindStringSubmatch(line); m != nil && gemNameRule.MatchString(m[1]+m[2]) {
		return m[1] + m[2], true
	}
	return "", true
}

// rubyExpressions names the lines whose name argument is an expression, for one unreadable gap.
func rubyExpressions(what string, lines []int) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = fmt.Sprint(l)
	}
	return what + " at line " + strings.Join(parts, ", ") + " is an expression or interpolation, not a literal; no name is derived from it"
}

// unsupportedCI names the CI configurations no inventory rule reads, by repository path or base
// name (DCI-V1-008).
var unsupportedCI = map[string]string{
	".gitlab-ci.yml": "GitLab CI", "Jenkinsfile": "Jenkins",
	"azure-pipelines.yml": "Azure Pipelines", ".travis.yml": "Travis CI", "bitbucket-pipelines.yml": "Bitbucket Pipelines",
	".buildkite/pipeline.yml": "Buildkite", ".drone.yml": "Drone", ".woodpecker.yml": "Woodpecker", "buildspec.yml": "AWS CodeBuild", "cloudbuild.yaml": "Cloud Build",
}

// repositoryInventory derives the repository inventory records when the manifest declares one
// (DCP-V1-702 to DCP-V1-705). Every record is the native provider's; every deployable unit,
// package and reference is anchored at the declared input line its rule matched.
func (c *compiler) repositoryInventory() error {
	m := c.manifest
	if m.RepositoryInventory == nil {
		return nil
	}
	b := &inventoryBuild{c: c, native: nativeProviderID(m), published: map[string]map[string]string{}, deployable: map[string]bool{}, packs: map[string]bool{}, publishJobs: map[string][]string{}, publishTies: map[string]map[string]string{}, publishTied: map[string][]string{}, unsupportedCI: map[string]bool{}, words: map[string]bool{}, deployJobs: map[string]string{}, declaredSeen: map[string]bool{}}
	for _, w := range m.RepositoryInventory.EnvironmentWords {
		b.words[w] = true
	}
	for _, d := range m.RepositoryInventory.DeployJobs {
		b.deployJobs[deployJobKey(m.repositoryOfAlias(d.Repository), d.Path, d.Job)] = d.Environment
	}
	declared := m.inventoryDeclarations()
	for _, r := range m.coveredRepositories() {
		d := declared[r.id]
		name, _ := repositoryName(r, d)
		alternates := slices.Clone(d.Aliases)
		if r.alias != "" && !slices.Contains(alternates, r.alias) {
			alternates = append(alternates, r.alias)
		}
		sort.Strings(alternates)
		if len(alternates) == 0 {
			alternates = nil
		}
		id := repositorySubjectID(b.native, r.id)
		ev := Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "fresh", Anchors: []Anchor{}, Unknown: "repository identity is the manifest's root-commit declaration, verified against its checkout; no file line anchors it", Limitations: []string{"covered repository identity only; deployment and hosting state unknown"}}
		c.artifact.Subjects = append(c.artifact.Subjects, Subject{ID: id, Kind: "repository", Name: name, Provider: b.native, Evidence: ev, AlternateIDs: alternates})
		text := fmt.Sprintf("repository %s, root commit %s, pinned at %s", name, r.id, r.revision)
		if len(alternates) > 0 {
			text += ", also named " + strings.Join(alternates, ", ")
		}
		c.artifact.Claims = append(c.artifact.Claims, Claim{id + ":identity", id, text, b.native, ev})
		for _, alias := range d.Aliases {
			b.publish("alias", alias, id)
		}
		if d.Archived {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "archived-repository", "repository_inventory declares this covered repository archived"})
		}
	}
	var files []inventoryFile
	for _, in := range m.Inputs {
		if in.Provider != b.native || !m.nativeRevision(in.Revision) || c.nativeExclusion(in) != "" {
			continue
		}
		text, valid, loaded := c.sources[inputKey(in.Revision, in.Path)].Text()
		if !valid || !loaded {
			continue
		}
		files = append(files, inventoryFile{revision: in.Revision, path: in.Path, qualifier: m.nativeQualifier(in.Revision), text: text, lines: strings.Split(text, "\n")})
	}
	// CI files are read first, so a package manifest sees its repository's publish jobs
	// (DCI-V1-006, DCI-V1-021); the records are sorted afterwards, so the order changes no bytes.
	for _, workflows := range []bool{true, false} {
		for _, f := range files {
			if (isWorkflow(f.path) || f.path == circleCIConfig) != workflows {
				continue
			}
			if err := b.file(f); err != nil {
				return err
			}
		}
	}
	if err := b.reconcile(); err != nil {
		return err
	}
	for _, d := range m.RepositoryInventory.DeployJobs {
		if id := m.repositoryOfAlias(d.Repository); !b.declaredSeen[deployJobKey(id, d.Path, d.Job)] {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{repositorySubjectID(b.native, id), "deploy-declaration-unmatched", "repository_inventory deploy_jobs declares job " + d.Job + " of " + d.Path + " deploying to " + d.Environment + ", but no read workflow or CircleCI input of this repository has that job key; nothing is derived from the declaration"})
		}
	}
	for _, r := range m.coveredRepositories() {
		id := repositorySubjectID(b.native, r.id)
		if !b.deployable[r.id] {
			reason := "no declared native input of this repository matches a deployable-unit rule (Dockerfile, Helm Chart.yaml, GitHub workflow or CircleCI deploy job); its deployable units are unknown, not inferred absent"
			if b.unsupportedCI[r.id] {
				reason += "; it declares CI configuration no rule reads (inventory-ci-unsupported)"
			}
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "deployable-evidence-unknown", reason})
		}
		if !b.packs[r.id] {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "package-evidence-unknown", "no declared native input of this repository is a package.json, go.mod or *.gemspec; its published packages are unknown, not inferred absent"})
		}
	}
	return nil
}

func (b *inventoryBuild) publish(ecosystem, name, subject string) {
	if b.published[ecosystem] == nil {
		b.published[ecosystem] = map[string]string{}
	}
	if _, ok := b.published[ecosystem][name]; !ok {
		b.published[ecosystem][name] = subject
	}
}

func (b *inventoryBuild) repository(f inventoryFile) string {
	id, _ := b.c.manifest.revisionRepository(b.c.root, f.revision)
	return id
}

func (b *inventoryBuild) fileSubject(f inventoryFile) string {
	return b.native + ":file:" + f.qualifier + f.path
}

func (b *inventoryBuild) unreadable(f inventoryFile, reason string) {
	b.c.artifact.Gaps = append(b.c.artifact.Gaps, Gap{b.fileSubject(f), "inventory-evidence-unreadable", reason})
}

func (b *inventoryBuild) evidence(f inventoryFile, line int, rule, limitation string) (Evidence, error) {
	anchor, err := b.c.anchor(f.revision, f.path, line, line, "syntax", "source", "repository inventory rule "+rule)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{limitation}}, nil
}

// record adds one deployable-unit or package subject with its derivation claim.
func (b *inventoryBuild) record(f inventoryFile, kind, id, name string, line int, rule, text string) error {
	ev, err := b.evidence(f, line, rule, "derived by a declared-evidence rule from one input line; deployment target, runtime state and build applicability unknown")
	if err != nil {
		return err
	}
	b.c.artifact.Subjects = append(b.c.artifact.Subjects, Subject{ID: id, Kind: kind, Name: name, Provider: b.native, Evidence: ev})
	b.c.artifact.Claims = append(b.c.artifact.Claims, Claim{id + ":derivation", id, fmt.Sprintf("%s, derived by rule %s from %s:%d", text, rule, f.qualifier+f.path, line), b.native, ev})
	return nil
}

func (b *inventoryBuild) reference(f inventoryFile, ecosystem, name, owner string, line int) {
	b.refs = append(b.refs, inventoryReference{ecosystem: ecosystem, name: name, owner: owner, revision: f.revision, path: f.path, line: line})
}

// The CI job rules (DCI-V1-009, DCI-V1-023) and the CircleCI configuration path they read.
const (
	githubJobRule   = "github-workflow-job/3"
	circleCIJobRule = "circleci-job/1"
	circleCIConfig  = ".circleci/config.yml"
)

func isWorkflow(p string) bool {
	return path.Dir(p) == ".github/workflows" && (path.Ext(p) == ".yml" || path.Ext(p) == ".yaml")
}

// file applies the closed rule set to one declared input (DCP-V1-703, DCI-V1-006 to DCI-V1-009).
func (b *inventoryBuild) file(f inventoryFile) error {
	base := path.Base(f.path)
	if format := unsupportedCI[f.path]; format != "" || base == "Jenkinsfile" {
		if format == "" {
			format = "Jenkins"
		}
		b.unsupportedCI[b.repository(f)] = true
		b.c.artifact.Gaps = append(b.c.artifact.Gaps, Gap{b.fileSubject(f), "inventory-ci-unsupported", format + " configuration is read by no inventory rule; its jobs, deployments and environments are NOT_OBSERVED"})
		return nil
	}
	switch {
	case base == "Dockerfile" || strings.HasPrefix(base, "Dockerfile.") || strings.HasSuffix(strings.ToLower(base), ".dockerfile"):
		return b.dockerfile(f)
	case base == "Chart.yaml":
		return b.chart(f)
	case isWorkflow(f.path):
		return b.workflow(f, githubJobRule)
	case f.path == circleCIConfig:
		return b.workflow(f, circleCIJobRule)
	case base == "package.json":
		return b.packageJSON(f)
	case base == "go.mod":
		return b.goMod(f)
	case strings.HasSuffix(base, ".gemspec") && base != ".gemspec":
		return b.gemspec(f)
	case base == "Gemfile":
		expressions := []int{}
		for i, line := range f.lines {
			if name, ok := rubyLiteral(gemfileGemCall, gemfileGem, line); name != "" {
				b.reference(f, "gem", name, repositorySubjectID(b.native, b.repository(f)), i+1)
			} else if ok {
				expressions = append(expressions, i+1)
			}
		}
		if len(expressions) > 0 {
			b.unreadable(f, rubyExpressions("Gemfile gem", expressions))
		}
	}
	return nil
}

// gemspec reads a gem specification (gemspec/1, DCI-V1-007): its name line is a package, because a
// gemspec exists to build a gem, and each runtime dependency is a reference.
func (b *inventoryBuild) gemspec(f inventoryFile) error {
	id := b.native + ":package:" + f.qualifier + f.path
	owner := repositorySubjectID(b.native, b.repository(f))
	b.packs[b.repository(f)] = true
	problems, named := []string{}, false
	for i, line := range f.lines {
		name, ok := rubyLiteral(gemspecNameCall, gemspecName, line)
		if !ok || named {
			continue
		}
		named = true
		if name == "" {
			problems = append(problems, rubyExpressions("gemspec name", []int{i + 1})+"; no gem is derived")
			continue
		}
		owner = id
		b.publish("gem", name, id)
		if err := b.record(f, "package", id, name, i+1, "gemspec/1", "Ruby gem "+name+"; the registry it is pushed to is NOT_OBSERVED"); err != nil {
			return err
		}
	}
	if !named {
		problems = append(problems, "gemspec declares no single-line literal name; no gem is derived")
	}
	expressions := []int{}
	for i, line := range f.lines {
		if name, ok := rubyLiteral(gemspecDepCall, gemspecDepend, line); name != "" {
			b.reference(f, "gem", name, owner, i+1)
		} else if ok {
			expressions = append(expressions, i+1)
		}
	}
	if len(expressions) > 0 {
		problems = append(problems, rubyExpressions("gemspec dependency", expressions))
	}
	if len(problems) > 0 {
		b.unreadable(f, strings.Join(problems, "; "))
	}
	return nil
}

// workflowEnvironment is the literal environment a job names in its block after line start, with
// its 1-based line; "" when it names none or only an expression (DCI-V1-009). keyed is true when
// the job has an environment key at all, literal or not.
func workflowEnvironment(lines []string, start int) (name string, line int, keyed bool) {
	for i := start; i < len(lines); i++ {
		text := lines[i]
		if strings.TrimSpace(text) != "" && !strings.HasPrefix(text, "   ") && !strings.HasPrefix(strings.TrimSpace(text), "#") {
			return "", 0, false
		}
		match := workflowEnv.FindStringSubmatch(text)
		if match == nil {
			if workflowEnvKey.MatchString(text) {
				return "", 0, true
			}
			continue
		}
		if match[1] == "" {
			for j := i + 1; j < len(lines) && strings.HasPrefix(lines[j], "      "); j++ {
				if name := workflowEnvName.FindStringSubmatch(lines[j]); name != nil && environmentName.MatchString(name[1]) {
					return name[1], j + 1, true
				}
			}
			return "", 0, true
		}
		if environmentName.MatchString(match[1]) {
			return match[1], i + 1, true
		}
		return "", 0, true
	}
	return "", 0, false
}

func (b *inventoryBuild) dockerfile(f inventoryFile) error {
	id := b.native + ":deployable:" + f.qualifier + f.path
	stages := map[string]bool{}
	first := 0
	for i, line := range f.lines {
		match := dockerFrom.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if first == 0 {
			first = i + 1
		}
		if image := match[1]; !strings.EqualFold(image, "scratch") && !stages[strings.ToLower(image)] {
			b.reference(f, "image", image, id, i+1)
		}
		if match[2] != "" {
			stages[strings.ToLower(match[2])] = true
		}
	}
	if first == 0 {
		b.unreadable(f, "Dockerfile holds no single-line FROM instruction; no image build is derived")
		return nil
	}
	b.deployable[b.repository(f)] = true
	return b.record(f, "deployable_unit", id, f.qualifier+f.path, first, "dockerfile/1", "container image build; the published image name is NOT_OBSERVED in a Dockerfile")
}

func (b *inventoryBuild) chart(f inventoryFile) error {
	id := b.native + ":deployable:" + f.qualifier + f.path
	name, at := "", 0
	inDependencies := false
	type dependency struct {
		name string
		line int
	}
	var dependencies []dependency
	for i, line := range f.lines {
		if match := chartName.FindStringSubmatch(line); match != nil && at == 0 {
			name, at = match[1], i+1
		}
		switch {
		case strings.HasPrefix(line, "dependencies:"):
			inDependencies = true
		case line != "" && line[0] != ' ' && line[0] != '-' && line[0] != '#':
			inDependencies = false
		case inDependencies:
			if match := chartDependency.FindStringSubmatch(line); match != nil {
				dependencies = append(dependencies, dependency{match[1], i + 1})
			}
		}
	}
	if at == 0 {
		b.unreadable(f, "Chart.yaml declares no top-level single-line name; no chart is derived")
		return nil
	}
	for _, d := range dependencies {
		b.reference(f, "chart", d.name, id, d.line)
	}
	b.deployable[b.repository(f)] = true
	b.publish("chart", name, id)
	return b.record(f, "deployable_unit", id, name, at, "helm-chart/1", "Helm chart "+name)
}

// workflow reads the jobs of a GitHub workflow (github-workflow-job/3) or of a CircleCI
// configuration (circleci-job/1, DCI-V1-023): a deploy job is a deployable unit, a release or
// publish job is publish evidence, and an action it uses is a reference.
func (b *inventoryBuild) workflow(f inventoryFile, rule string) error {
	repository := repositorySubjectID(b.native, b.repository(f))
	inJobs, owner, root := false, repository, parseCI(f.lines)
	for i, line := range f.lines {
		if strings.HasPrefix(line, "jobs:") && strings.TrimSpace(strings.TrimPrefix(line, "jobs:")) == "" {
			inJobs = true
			continue
		}
		if line != "" && line[0] != ' ' && line[0] != '#' {
			inJobs, owner = false, repository
			continue
		}
		if !inJobs {
			continue
		}
		if match := workflowJob.FindStringSubmatch(line); match != nil {
			owner = repository
			job := strings.ToLower(match[1])
			declared, isDeclared := b.deployJobs[deployJobKey(b.repository(f), f.path, match[1])]
			switch {
			case isDeclared || deployJobName(match[1]):
				owner = b.native + ":deployable:" + f.qualifier + f.path + ":" + match[1]
				b.deployable[b.repository(f)] = true
				b.declaredSeen[deployJobKey(b.repository(f), f.path, match[1])] = true
				if err := b.deployJob(f, root, owner, match[1], i+1, rule, declared); err != nil {
					return err
				}
			case strings.Contains(job, "release") || strings.Contains(job, "publish"):
				if err := b.publishJob(f, root, repository, match[1], i+1, rule); err != nil {
					return err
				}
			}
			continue
		}
		if match := workflowUses.FindStringSubmatch(line); match != nil {
			uses, _, _ := strings.Cut(match[1], "@")
			if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
				continue
			}
			parts := strings.Split(uses, "/")
			if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
				b.reference(f, "action", parts[0]+"/"+parts[1], owner, i+1)
			}
		}
	}
	return nil
}

// deployJob records a deploy job with the environment the manifest declares for it (DCI-V1-029),
// else the environment it names (DCI-V1-009), else, when it has no environment key, with the one
// environment its job key, workflow name or file name names as weaker, labelled evidence
// (DCI-V1-022), else with its environment NOT_OBSERVED. A CircleCI job's environment key holds
// variables, so it names no deployment environment (DCI-V1-023).
func (b *inventoryBuild) deployJob(f inventoryFile, root *ciNode, id, job string, line int, rule, declared string) error {
	environment, at, keyed, source := "", 0, false, ""
	if rule == githubJobRule {
		environment, at, keyed = workflowEnvironment(f.lines, line)
	}
	if declared != "" {
		text := "CI job " + job + " to environment " + declared + ", declared by repository_inventory deploy_jobs"
		if keyed && environment != declared {
			text += "; its environment key names " + cmp.Or(environment, "no literal environment")
		}
		return b.record(f, "deployable_unit", id, job+" ("+declared+")", line, rule, text)
	}
	if !keyed {
		environment, source, at = inferredEnvironment(root, f, job, b.words)
	}
	switch {
	case environment == "" && keyed:
		return b.record(f, "deployable_unit", id, job, line, rule, "CI deploy job "+job+"; its deployment environment is NOT_OBSERVED (the job names no literal environment)")
	case environment == "":
		return b.record(f, "deployable_unit", id, job, line, rule, "CI deploy job "+job+"; its deployment environment is NOT_OBSERVED (no environment key, and its job key, workflow name and file name name no single environment)")
	case source != "":
		if err := b.record(f, "deployable_unit", id, job+" ("+environment+")", line, rule, "CI deploy job "+job+", environment "+environment+" inferred from "+source+" (no environment key; name-derived, weaker evidence)"); err != nil {
			return err
		}
		limitation := "environment " + environment + " is inferred from " + source + ", not declared by an environment key"
		s := &b.c.artifact.Subjects[len(b.c.artifact.Subjects)-1]
		s.Evidence.Limitations = append(slices.Clone(s.Evidence.Limitations), limitation)
		claim := &b.c.artifact.Claims[len(b.c.artifact.Claims)-1]
		claim.Evidence.Limitations = append(slices.Clone(claim.Evidence.Limitations), limitation)
	default:
		if err := b.record(f, "deployable_unit", id, job+" ("+environment+")", line, rule, "CI deploy job "+job+" to environment "+environment); err != nil {
			return err
		}
	}
	if at == 0 {
		return nil
	}
	anchor, err := b.c.anchor(f.revision, f.path, at, at, "syntax", "source", "repository inventory rule "+rule+" environment")
	if err != nil {
		return err
	}
	s := &b.c.artifact.Subjects[len(b.c.artifact.Subjects)-1]
	s.Evidence.Anchors = append(slices.Clone(s.Evidence.Anchors[:1]), anchor)
	claim := &b.c.artifact.Claims[len(b.c.artifact.Claims)-1]
	claim.Evidence.Anchors = append(slices.Clone(claim.Evidence.Anchors[:1]), anchor)
	return nil
}

// publishJob records a release or publish job as publish evidence (DCI-V1-006); it is not a
// deployable unit. A literal npm, pnpm or yarn publish step ties it to the directory it publishes;
// a job with no such step, or one whose publish directory is not literal, is only a candidate
// (DCI-V1-021).
func (b *inventoryBuild) publishJob(f inventoryFile, root *ciNode, repository, job string, line int, rule string) error {
	ties, opaque := publishTies(root, job, rule)
	limitation := "a release or publish job name with no literal publish step directory; the registry, the artifact it publishes and whether it runs are unknown"
	if len(ties) > 0 {
		limitation = "a literal publish step of a release or publish job; the registry and whether the job runs are unknown"
	}
	ev, err := b.evidence(f, line, rule, limitation)
	if err != nil {
		return err
	}
	repo := b.repository(f)
	at := fmt.Sprintf("%s at %s:%d", job, f.qualifier+f.path, line)
	var dirs []string
	for _, t := range ties {
		if b.publishTies[repo] == nil {
			b.publishTies[repo] = map[string]string{}
		}
		if _, ok := b.publishTies[repo][t.dir]; !ok {
			b.publishTies[repo][t.dir] = fmt.Sprintf("%s runs a publish step at line %d", at, t.line)
		}
		if dir := publishDirectory(t.dir); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	text := "CI release or publish job " + at
	switch {
	case len(ties) == 0:
		text += ", a candidate only: no npm, pnpm or yarn publish step of it names a literal directory"
	case opaque:
		text += ", publishing " + strings.Join(dirs, ", ") + "; another publish step's directory is not literal, so it is a candidate for this repository's other package manifests"
	default:
		text += ", publishing " + strings.Join(dirs, ", ")
	}
	if len(ties) == 0 || opaque {
		b.publishJobs[repo] = append(b.publishJobs[repo], at)
	}
	if len(ties) > 0 {
		b.publishTied[repo] = append(b.publishTied[repo], at+" (publishing "+strings.Join(dirs, ", ")+")")
	}
	b.c.artifact.Claims = append(b.c.artifact.Claims, Claim{repository + ":publish-job:" + f.qualifier + f.path + ":" + job, repository, text, b.native, ev})
	return nil
}

// publishDirectory names a repository directory a publish step publishes.
func publishDirectory(dir string) string {
	if dir == "." {
		return "the repository root"
	}
	return "directory " + dir
}

func (b *inventoryBuild) packageJSON(f inventoryFile) error {
	var document struct {
		Name          *string           `json:"name"`
		Private       bool              `json:"private"`
		Dependencies  map[string]string `json:"dependencies"`
		PublishConfig jsontext.Value    `json:"publishConfig"`
	}
	if err := json.Unmarshal([]byte(f.text), &document); err != nil {
		b.unreadable(f, "package.json is not one JSON object with a string name, boolean private and string dependencies; no package is derived")
		return nil
	}
	b.packs[b.repository(f)] = true
	owner := repositorySubjectID(b.native, b.repository(f))
	if document.Name != nil && !document.Private {
		line := jsonMemberLine(f.lines, 0, "name", *document.Name)
		if line == 0 {
			b.unreadable(f, "package.json name is not on one line; no package is derived")
		} else {
			owner = b.native + ":package:" + f.qualifier + f.path
			b.publish("npm", *document.Name, owner)
			kind, text := "package_manifest", "npm package manifest "+*document.Name+"; publish evidence NOT_OBSERVED (no publishConfig and no release or publish job in this repository)"
			if at := jsonMemberLine(f.lines, 0, "publishConfig", ""); len(document.PublishConfig) > 0 && document.PublishConfig.Kind() == '{' && at > 0 {
				kind, text = "package", fmt.Sprintf("npm package %s, publish evidence publishConfig at %s:%d", *document.Name, f.qualifier+f.path, at)
			} else if tie := b.publishTies[b.repository(f)][path.Dir(f.path)]; tie != "" {
				kind, text = "package", "npm package "+*document.Name+", publish evidence CI job "+tie+" in this manifest's directory"
			} else if jobs, tied := b.publishJobs[b.repository(f)], b.publishTied[b.repository(f)]; len(jobs)+len(tied) > 0 {
				// Jobs exist, so the text names them rather than saying the repository has none.
				text = "npm package manifest " + *document.Name + "; publish evidence NOT_OBSERVED: no release or publish job ties this manifest's directory by a literal npm, pnpm or yarn publish step"
				if len(jobs) > 0 {
					text += "; candidate CI release or publish job " + strings.Join(jobs, ", ")
				}
				if len(tied) > 0 {
					text += "; observed jobs tied to other directories: " + strings.Join(tied, ", ")
				}
			}
			if err := b.record(f, kind, owner, *document.Name, line, "package-json/3", text); err != nil {
				return err
			}
		}
	}
	start := jsonMemberLine(f.lines, 0, "dependencies", "")
	for _, name := range slices.Sorted(maps.Keys(document.Dependencies)) {
		if line := jsonMemberLine(f.lines, max(start-1, 0), name, ""); start > 0 && line > 0 {
			b.reference(f, "npm", name, owner, line)
		} else {
			b.unreadable(f, "package.json dependency "+name+" is not on one line after its dependencies member; it is not reconciled")
		}
	}
	return nil
}

// jsonMemberLine is the first 1-based line from line after holding the quoted member name and,
// when value is set, its quoted string value; 0 when none does.
func jsonMemberLine(lines []string, after int, name, value string) int {
	key, _ := json.Marshal(name)
	want := ""
	if value != "" {
		quoted, _ := json.Marshal(value)
		want = string(quoted)
	}
	for i := after; i < len(lines); i++ {
		at := strings.Index(lines[i], string(key))
		if at < 0 || !strings.HasPrefix(strings.TrimSpace(lines[i][at+len(key):]), ":") {
			continue
		}
		if want == "" || strings.Contains(lines[i][at+len(key):], want) {
			return i + 1
		}
	}
	return 0
}

func (b *inventoryBuild) goMod(f inventoryFile) error {
	b.packs[b.repository(f)] = true
	owner := repositorySubjectID(b.native, b.repository(f))
	inRequire := false
	for i, line := range f.lines {
		trimmed := strings.TrimSpace(line)
		if match := goModule.FindStringSubmatch(trimmed); match != nil && !strings.HasPrefix(owner, b.native+":package:") {
			owner = b.native + ":package:" + f.qualifier + f.path
			b.publish("go", match[1], owner)
			if err := b.record(f, "package", owner, match[1], i+1, "go-mod/1", "Go module "+match[1]); err != nil {
				return err
			}
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "require (") || trimmed == "require(":
			inRequire = true
			continue
		case inRequire && trimmed == ")":
			inRequire = false
			continue
		case !inRequire && !strings.HasPrefix(trimmed, "require "):
			continue
		}
		if match := goRequire.FindStringSubmatch(trimmed); match != nil && !strings.Contains(match[2], "indirect") {
			b.reference(f, "go", match[1], owner, i+1)
		}
	}
	return nil
}

// imageRepository is an image reference without its tag or digest.
func imageRepository(image string) string {
	image, _, _ = strings.Cut(image, "@")
	if slash, colon := strings.LastIndex(image, "/"), strings.LastIndex(image, ":"); colon > slash {
		image = image[:colon]
	}
	return image
}

// reconcile joins each reference to the covered record that publishes its exact name: a package
// of the same ecosystem, a chart, or for an image or action a declared repository alias. A joined
// reference is a depends_on relation; any other is an outside-corpus-reference gap (DCP-V1-704).
// A reference's ID names its ecosystem and name after its line, because one line may hold several
// references, such as two package.json dependencies (DCP-V1-706).
func (b *inventoryBuild) reconcile() error {
	seen := map[string]bool{}
	for _, ref := range b.refs {
		f := inventoryFile{revision: ref.revision, path: ref.path, qualifier: b.c.manifest.nativeQualifier(ref.revision)}
		id := fmt.Sprintf("%s:reference:%s%s:%d:%s:%s", b.native, f.qualifier, ref.path, ref.line, ref.ecosystem, ref.name)
		if seen[id] {
			continue
		}
		seen[id] = true
		ev, err := b.evidence(f, ref.line, ref.ecosystem+"-reference/1", "declared reference text only; the version used and whether it is deployed or consumed at run time are unknown")
		if err != nil {
			return err
		}
		label := map[string]string{"image": "container image", "chart": "Helm chart", "action": "GitHub action or reusable workflow", "npm": "npm package", "go": "Go module", "gem": "Ruby gem"}[ref.ecosystem]
		text := fmt.Sprintf("consumes %s %s at %s:%d", label, ref.name, f.qualifier+ref.path, ref.line)
		b.c.artifact.Claims = append(b.c.artifact.Claims, Claim{id, ref.owner, text, b.native, ev})
		name, ecosystem := ref.name, ref.ecosystem
		switch ecosystem {
		case "image":
			name, ecosystem = imageRepository(name), "alias"
		case "action":
			ecosystem = "alias"
		}
		target, ok := b.published[ecosystem][name]
		if ok && target == ref.owner {
			continue
		}
		if !ok {
			if prefix := b.firstParty(name); prefix != "" {
				b.c.artifact.Gaps = append(b.c.artifact.Gaps, Gap{ref.owner, "first-party-reference-outside-corpus", id + ": " + text + "; " + name + " is under the declared first-party prefix " + prefix + ", but no covered repository declares it, so its first-party source repository is not covered (NOT_OBSERVED)"})
				continue
			}
			b.c.artifact.Gaps = append(b.c.artifact.Gaps, Gap{ref.owner, "outside-corpus-reference", id + ": " + text + "; no covered repository declares " + name + ", so its source repository is outside the corpus or undeclared (NOT_OBSERVED)"})
			continue
		}
		joined := ev
		joined.Limitations = []string{"exact name join of a declared reference to a covered repository's declared package, chart or alias; version compatibility unknown"}
		b.c.artifact.Relations = append(b.c.artifact.Relations, Relation{id + ":resolves", ref.owner, target, "depends_on", b.native, joined})
	}
	return nil
}

// firstParty is the longest declared first-party prefix of name, "" when none (DCI-V1-010).
func (b *inventoryBuild) firstParty(name string) string {
	best := ""
	for _, prefix := range b.c.manifest.RepositoryInventory.FirstParty {
		if strings.HasPrefix(name, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	return best
}
