package workqueue

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const maxCollisionPaths = 4096

type indexCollisionSource struct {
	index    *contextindex.Index
	forward  map[string]map[string]struct{}
	reverse  map[string]map[string]struct{}
	unparsed map[string]struct{}
}

type goModuleRoot struct {
	module string
	root   string
}

// IndexCollisionSource returns a pure collision source over an already-built,
// immutable context index. Its reverse import table is computed once.
func IndexCollisionSource(index *contextindex.Index) CollisionSource {
	source := &indexCollisionSource{
		index:    index,
		forward:  map[string]map[string]struct{}{},
		reverse:  map[string]map[string]struct{}{},
		unparsed: map[string]struct{}{},
	}
	if index == nil {
		return source
	}
	modules := goModuleRoots(index)
	goPackages := map[string][]string{}
	for tracked := range index.Tracked {
		if strings.HasSuffix(tracked, ".go") {
			directory := path.Dir(tracked)
			goPackages[directory] = append(goPackages[directory], tracked)
		}
	}
	for importer, importedPaths := range index.Imports {
		for imported := range importedPaths {
			targets := []string{}
			if _, tracked := index.Tracked[imported]; tracked {
				targets = append(targets, imported)
			}
			if strings.HasSuffix(importer, ".go") {
				if directory, resolved := resolveGoImport(imported, modules); resolved {
					targets = append(targets, goPackages[directory]...)
				}
			}
			for _, target := range targets {
				if source.forward[importer] == nil {
					source.forward[importer] = map[string]struct{}{}
				}
				source.forward[importer][target] = struct{}{}
				if source.reverse[target] == nil {
					source.reverse[target] = map[string]struct{}{}
				}
				source.reverse[target][importer] = struct{}{}
			}
		}
	}
	for _, refusal := range index.Unparsed {
		if refusal.Facts == "imports" {
			source.unparsed[refusal.Path] = struct{}{}
		}
	}
	return source
}

func goModuleRoots(index *contextindex.Index) []goModuleRoot {
	modules := []goModuleRoot{}
	for sourcePath, source := range index.Sources {
		if path.Base(sourcePath) != "go.mod" {
			continue
		}
		text, valid, loaded := source.Text()
		if !loaded || !valid {
			continue
		}
		module, ok := modulePathFromGoMod(text)
		if !ok {
			continue
		}
		root := path.Dir(sourcePath)
		if root == "." {
			root = ""
		}
		modules = append(modules, goModuleRoot{module: module, root: root})
	}
	return modules
}

func modulePathFromGoMod(text string) (string, bool) {
	module := ""
	for _, raw := range strings.Split(text, "\n") {
		if comment := strings.Index(raw, "//"); comment >= 0 {
			raw = raw[:comment]
		}
		fields := strings.Fields(raw)
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}
		if module != "" || len(fields) != 2 || strings.Contains(raw, "/*") {
			return "", false
		}
		candidate := fields[1]
		if candidate[0] == '"' || candidate[0] == '`' {
			decoded, err := strconv.Unquote(candidate)
			if err != nil {
				return "", false
			}
			candidate = decoded
		}
		if candidate == "" || candidate == "." || candidate == ".." || path.IsAbs(candidate) || path.Clean(candidate) != candidate ||
			strings.ContainsAny(candidate, "\\ \t\r\n") || strings.HasPrefix(candidate, "../") {
			return "", false
		}
		module = candidate
	}
	return module, module != ""
}

func resolveGoImport(imported string, modules []goModuleRoot) (string, bool) {
	best := goModuleRoot{}
	bestLength := -1
	ambiguous := false
	for _, candidate := range modules {
		if imported != candidate.module && !strings.HasPrefix(imported, candidate.module+"/") {
			continue
		}
		if len(candidate.module) > bestLength {
			best, bestLength, ambiguous = candidate, len(candidate.module), false
		} else if len(candidate.module) == bestLength && candidate.root != best.root {
			ambiguous = true
		}
	}
	if bestLength < 0 || ambiguous {
		return "", false
	}
	suffix := strings.TrimPrefix(imported[bestLength:], "/")
	if suffix != "" && (suffix == ".." || path.IsAbs(suffix) || path.Clean(suffix) != suffix || strings.HasPrefix(suffix, "../")) {
		return "", false
	}
	directory := path.Join(best.root, suffix)
	if directory == "" {
		directory = "."
	}
	for _, candidate := range modules {
		if candidate.root != best.root && pathWithin(directory, candidate.root) && pathWithin(candidate.root, best.root) {
			return "", false
		}
	}
	return directory, true
}

func pathWithin(candidate, root string) bool {
	if root == "" {
		return true
	}
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}

func (source *indexCollisionSource) collisionCommitRevision() string {
	if source.index == nil {
		return ""
	}
	return source.index.CommitRevision
}

func (source *indexCollisionSource) Closure(paths []string) ([]string, bool) {
	if source.index == nil {
		return nil, false
	}
	closure := map[string]struct{}{}
	for _, declared := range paths {
		closure[declared] = struct{}{}
		if strings.HasSuffix(declared, "/") {
			for tracked := range source.index.Tracked {
				if strings.HasPrefix(tracked, declared) {
					closure[tracked] = struct{}{}
				}
			}
			continue
		}
		if _, tracked := source.index.Tracked[declared]; !tracked {
			continue
		}
		if _, refused := source.unparsed[declared]; refused {
			continue
		}
		forward, hasForward := source.forward[declared]
		reverse, hasReverse := source.reverse[declared]
		if !hasForward && !hasReverse {
			continue
		}
		for imported := range forward {
			if _, tracked := source.index.Tracked[imported]; tracked {
				closure[imported] = struct{}{}
			}
		}
		for importer := range reverse {
			if _, tracked := source.index.Tracked[importer]; tracked {
				closure[importer] = struct{}{}
			}
		}
	}
	if len(closure) > maxCollisionPaths {
		return nil, false
	}
	result := make([]string, 0, len(closure))
	for path := range closure {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, true
}

func DeriveCollisions(snapshot *Snapshot, source CollisionSource) CollisionClosure {
	result := CollisionClosure{
		Complete:       true,
		State:          StateValidated,
		TicketGroupIDs: map[string][]string{},
	}
	if snapshot == nil || source == nil {
		return incompleteCollisionClosure(result)
	}
	if pinned, ok := source.(interface{ collisionCommitRevision() string }); ok {
		if pinned.collisionCommitRevision() != snapshot.RepositorySource.Commit {
			return incompleteCollisionClosure(result)
		}
	}
	ready := make([]TicketSummary, 0, len(snapshot.Tickets))
	closures := make(map[string][]string, len(snapshot.Tickets))
	for _, ticket := range snapshot.Tickets {
		if ticket.Lifecycle != "READY" {
			continue
		}
		closure, complete := source.Closure(ticket.TouchPaths)
		if !complete || len(closure) > maxCollisionPaths || !validClosurePaths(closure) {
			return incompleteCollisionClosure(result)
		}
		ready = append(ready, ticket)
		closures[ticket.TicketID] = sortedUnique(closure)
		result.TicketGroupIDs[ticket.TicketID] = append([]string(nil), ticket.CollisionGroupIDs...)
	}
	result.Groups = append(result.Groups, adapterCollisionGroups(ready)...)
	derived := derivedCollisionGroups(snapshot, ready, closures)
	result.Groups = append(result.Groups, derived...)
	for _, group := range derived {
		for _, ticketID := range group.MemberTicketIDs {
			result.TicketGroupIDs[ticketID] = append(result.TicketGroupIDs[ticketID], group.ID)
		}
	}
	for ticketID, groups := range result.TicketGroupIDs {
		result.TicketGroupIDs[ticketID] = sortedUnique(groups)
	}
	canonicalSort(result.Groups, collisionGroupValue)
	return result
}

func incompleteCollisionClosure(result CollisionClosure) CollisionClosure {
	result.Complete = false
	result.State = StateUnknown
	result.Unknowns = []string{UnknownCollisionClosureIncomplete}
	result.Groups = []CollisionGroup{}
	result.TicketGroupIDs = map[string][]string{}
	return result
}

func validClosurePaths(paths []string) bool {
	seen := map[string]struct{}{}
	for _, path := range paths {
		if ValidatePath(path) != nil {
			return false
		}
		if duplicate(seen, path) {
			return false
		}
	}
	return true
}

func adapterCollisionGroups(tickets []TicketSummary) []CollisionGroup {
	members := map[string][]string{}
	for _, ticket := range tickets {
		for _, groupID := range ticket.CollisionGroupIDs {
			members[groupID] = append(members[groupID], ticket.TicketID)
		}
	}
	result := make([]CollisionGroup, 0, len(members))
	for groupID, ticketIDs := range members {
		result = append(result, CollisionGroup{
			ID:              groupID,
			MemberTicketIDs: sortedUnique(ticketIDs),
			Source:          "ADAPTER",
		})
	}
	return result
}

func derivedCollisionGroups(snapshot *Snapshot, tickets []TicketSummary, closures map[string][]string) []CollisionGroup {
	members := map[string][]string{}
	for _, ticket := range tickets {
		for _, path := range closures[ticket.TicketID] {
			members[path] = append(members[path], ticket.TicketID)
		}
	}
	authority, queue, ok := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	if !ok {
		return nil
	}
	result := []CollisionGroup{}
	for path, ticketIDs := range members {
		if len(ticketIDs) < 2 {
			continue
		}
		pathCopy := path
		result = append(result, CollisionGroup{
			ID:              derivedCollisionID(authority, queue, path),
			MemberTicketIDs: sortedUnique(ticketIDs),
			Path:            &pathCopy,
			Source:          "CORVINT_INDEX",
		})
	}
	return result
}

func derivedCollisionID(authority, queue, path string) string {
	digest := sha256.Sum256([]byte(path))
	return "collision:" + authority + ":" + queue + ":corvint-" + hex.EncodeToString(digest[:])[:32]
}
