package contextindex

// Trust classes (decision 0346; TCP-V0-023, FPK-V0-032). Every evidence row
// of a `context` packet and every `prove` proof row carries exactly one,
// derived from the `authority` label the row already carries and from nothing
// else, so the class adds no input and cannot disagree with the label.
const (
	// TrustProjectAuthority is content the project owns as instruction,
	// specification, decision, contract, or ledger (AGENTS.md invariant 3).
	TrustProjectAuthority = "project-authority"
	// TrustRepositoryContent is content pinned to a blob of the tree at the
	// packet's revision and read by Corvint's own grammars or conventions.
	TrustRepositoryContent = "repository-content"
	// TrustRepositoryHistory is a relation read from commit history: a reason
	// to read a file, never proof of its content.
	TrustRepositoryHistory = "repository-history"
	// TrustExternalProvider is content fetched from a provider command or MCP
	// server (`internal/extevidence`).
	TrustExternalProvider = "external-provider"
	// TrustToolOutput is content a tool produced that no blob of the tree
	// pins: a learned ledger, an unverified analyzer contract, generated
	// documentation, and every label this table does not name.
	TrustToolOutput = "tool-output"
)

// trustByAuthority is the closed derivation table. A label absent from it is
// tool-output, the least trusted class, so an unlisted label can never satisfy
// an authority, governance, or basis requirement by omission (invariant 2).
var trustByAuthority = map[string]string{
	"project-instructions":          TrustProjectAuthority,
	"instruction-reference":         TrustProjectAuthority,
	"repository-spec":               TrustProjectAuthority,
	"accepted-spec":                 TrustProjectAuthority,
	"accepted-decision":             TrustProjectAuthority,
	"non-binding-decision":          TrustProjectAuthority,
	"accepted-contract":             TrustProjectAuthority,
	"partially-superseded-contract": TrustProjectAuthority,
	"document-reference":            TrustProjectAuthority,
	"canonical-ledger":              TrustProjectAuthority,
	"source-marker":                 TrustProjectAuthority,
	"test-marker":                   TrustProjectAuthority,
	SyntaxAuthority:                 TrustRepositoryContent,
	"git-tree":                      TrustRepositoryContent,
	"test-convention":               TrustRepositoryContent,
	"task-text":                     TrustRepositoryContent,
	"directory":                     TrustRepositoryContent,
	"vocabulary":                    TrustRepositoryContent,
	"affected-selection":            TrustRepositoryContent,
	"cem-supported":                 TrustRepositoryContent,
	"cem-mechanical":                TrustRepositoryContent,
	"cem-unknown":                   TrustRepositoryContent,
	"git-history":                   TrustRepositoryHistory,
	"external-provider":             TrustExternalProvider,
	UnverifiedContractAuthority:     TrustToolOutput,
	UnverifiedLedgerAuthority:       TrustToolOutput,
	"local-task-trace":              TrustToolOutput,
	"generated-documentation":       TrustToolOutput,
}

// TrustClass derives the one trust class of a row from its authority label.
func TrustClass(authority string) string {
	if class, ok := trustByAuthority[authority]; ok {
		return class
	}
	return TrustToolOutput
}

// TrustTainted reports whether a class can satisfy no authority, governance,
// or basis requirement: content fetched from a provider or produced by a tool
// is read, never trusted, however it is labelled.
func TrustTainted(class string) bool {
	return class == TrustExternalProvider || class == TrustToolOutput
}

// governanceRows are the reserved rows that may satisfy TCP-V0-009's
// governance receipt and TCP-V0-011's `critical` selectors. A reserved row
// whose trust class is tainted satisfies neither; governanceRefused names it.
func (compiler *taskContextCompiler) governanceRows() []contextRow {
	rows := make([]contextRow, 0, len(compiler.reserved))
	for _, row := range compiler.reserved {
		if TrustTainted(TrustClass(row.authority)) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

// governanceRefused lists, in reservation order, every reserved row a tainted
// trust class kept from satisfying governance, each naming its relation, path,
// and class (TCP-V0-023). Empty on every packet the generators produce today.
func (compiler *taskContextCompiler) governanceRefused() []any {
	refused := make([]any, 0)
	for _, row := range compiler.reserved {
		class := TrustClass(row.authority)
		if !TrustTainted(class) {
			continue
		}
		refused = append(refused, map[string]any{
			"relation": row.kind, "path": row.path, "trust": class,
			"reason": "a " + class + " row cannot satisfy governance",
		})
	}
	return refused
}
