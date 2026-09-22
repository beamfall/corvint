package authority

import "context"

type WorktreeState string

const (
	WorktreeClean WorktreeState = "CLEAN"
	WorktreeMixed WorktreeState = "MIXED"
)

// Snapshot is the complete path-free repository identity admitted before a
// dashboard scan. All fields are immutable values.
type Snapshot struct {
	DirtyPathCount   uint64
	DirtyPathsSHA256 string
	HeadRevision     string
	ObjectFormat     string
	TreeRevision     string
	WorktreeState    WorktreeState
}

type Witness struct {
	Kind         string
	ObjectFormat string
	ObjectID     string
	ObjectType   string
	Revision     *string
}

// PathWitness is a private in-process association. Adapters use it to prove
// complete byte-exact path coverage; it never enters the snapshot model.
type PathWitness struct {
	Path    string
	Witness Witness
}

type TraceCode string

const (
	TraceQualified         TraceCode = "QUALIFIED"
	TraceObjectUnavailable TraceCode = "OBJECT_UNAVAILABLE"
	TraceAncestryBound     TraceCode = "ANCESTRY_BOUND"
	TracePathUntracked     TraceCode = "PATH_UNTRACKED"
	TraceInvalidIdentity   TraceCode = "INVALID_IDENTITY"
	TraceInterrupted       TraceCode = "INTERRUPTED"
)

type TraceQualification struct {
	Code          TraceCode
	ObjectFormat  string
	TreeRevision  string
	Witnesses     []Witness
	PathWitnesses []PathWitness
}

type FinishCode string

const (
	FinishStable      FinishCode = "STABLE"
	FinishChanged     FinishCode = "CHANGED"
	FinishUnavailable FinishCode = "UNAVAILABLE"
)

type FinishResult struct {
	Code FinishCode
}

type TraceQualifier interface {
	QualifyTrace(context.Context, string, []string) TraceQualification
}

// Authority owns exactly one initialized repository scan lifetime. Finish is
// terminal and performs the final identity check and cleanup.
type Authority interface {
	TraceQualifier
	Snapshot() Snapshot
	Finish(context.Context) FinishResult
}
