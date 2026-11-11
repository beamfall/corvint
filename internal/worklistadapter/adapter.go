// Package worklistadapter is the repository-work-queue-adapter/0 producer for a
// committed JSON worklist. Corvint's self-dogfood adapter and `corvint work
// adapter` both run it; every authority it emits comes from the committed policy.
package worklistadapter

const (
	PolicyPath      = ".corvint/work-queue-policy.json"
	WorklistProfile = "corvint-worklist/0"
	// RepositoryMapping is the mappingVersion of the adoptable repository worklist.
	RepositoryMapping = "repository-worklist-v0"
)

type mapping struct{ worklist, route string }

// mappings is the closed set of policy mappingVersion values this producer
// implements: where the committed worklist lives and which route its tickets need.
var mappings = map[string]mapping{
	"decision-0046-v0": {worklist: "docs/worklist.json", route: "codex"},
	RepositoryMapping:  {worklist: ".corvint/worklist.json", route: "agent"},
}

type Worklist struct {
	Profile string `json:"profile"`
	Tickets []Item `json:"tickets"`
}

type Item struct {
	Body       string   `json:"body"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	TouchPaths []string `json:"touchPaths"`
}
