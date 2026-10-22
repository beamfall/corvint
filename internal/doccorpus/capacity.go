package doccorpus

import (
	"context"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// CapacityCode is the typed refusal of a documented corpus capacity limit
// (DCP-V1-157). Its message names the limit and the observed count.
const CapacityCode = "corpus-capacity-exceeded"

// maxPinnedInputBytes bounds the distinct input bytes one build pins.
const maxPinnedInputBytes = 128 << 20

// corpusGitHooks are the test-only Git hooks one call carries in its context,
// so tests that bound a phase's operations (DCP-V1-155) or compare batched
// reads with the per-path reads (DCP-V1-216) can run in parallel. Production
// contexts never carry them.
type corpusGitHooks struct {
	budget      func() *gitrun.Budget
	prefetchOff bool
}

type corpusGitHooksKey struct{}

func corpusGitHooksOf(ctx context.Context) corpusGitHooks {
	hooks, _ := ctx.Value(corpusGitHooksKey{}).(corpusGitHooks)
	return hooks
}

// newGitBudget opens the Git operation budget of one corpus phase: the
// default budget unless a test context bounds it.
func newGitBudget(ctx context.Context) *gitrun.Budget {
	if budget := corpusGitHooksOf(ctx).budget; budget != nil {
		return budget()
	}
	return gitrun.NewDefaultBudget()
}

func capacityExceeded(limit string, count, bound int) error {
	return &Error{Code: CapacityCode, Message: fmt.Sprintf("%s %d exceeds the limit of %d", limit, count, bound)}
}

// Typed returns err with a Git capacity failure (operation budget or bounded
// Git output) typed as corpus-capacity-exceeded, so the CLI and the corpus
// MCP bridge report one code instead of internal-error or a generic refusal
// (DCP-V1-158). Every other error, and every corpus Error, is unchanged.
func Typed(err error) error {
	return gitCapacity(err, "")
}

// gitCapacity is Typed with the phase and input count named when known.
func gitCapacity(err error, phase string) error {
	var coded *Error
	if err == nil || errors.As(err, &coded) {
		return err
	}
	var limit string
	switch cemcode.CodeOf(err) {
	case cemcode.GitBudgetExceeded:
		limit = fmt.Sprintf("Git operation budget of %d operations exhausted", gitrun.DefaultOperations)
	case cemcode.GitOutputExceeded:
		limit = "Git output byte bound exceeded"
	default:
		return err
	}
	if phase != "" {
		limit += " " + phase
	}
	return &Error{Code: CapacityCode, Message: limit + " (" + err.Error() + ")"}
}

const (
	MaxProviderBytes       = 64 << 20
	SchemaV2               = "corvint-evidence-corpus/2"
	ManifestSchemaV2       = "corvint-corpus-input/2"
	AdoptionProviderSchema = "corvint-corpus-adoption-provider/1"
	MaxCorpusBytes         = 128 << 20
	MaxCorpusRecords       = 100000
	MaxCorpusJourneys      = 4096
)

func encodingLimit(value any) int {
	switch v := value.(type) {
	case Manifest:
		if v.Schema == ManifestSchemaV2 {
			return MaxCorpusBytes
		}
	case *Manifest:
		if v != nil && v.Schema == ManifestSchemaV2 {
			return MaxCorpusBytes
		}
	case Artifact:
		if v.Schema == SchemaV2 {
			return MaxCorpusBytes
		}
	case *Artifact:
		if v != nil && v.Schema == SchemaV2 {
			return MaxCorpusBytes
		}
	case ProviderRecord:
		if v.Schema == AdoptionProviderSchema || v.Schema == TestKnowledgeSchema || v.Schema == FlowRelationSchema || v.Schema == ClaimAuditSchema {
			return MaxProviderBytes
		}
	case *ProviderRecord:
		if v != nil && (v.Schema == AdoptionProviderSchema || v.Schema == TestKnowledgeSchema || v.Schema == FlowRelationSchema || v.Schema == ClaimAuditSchema) {
			return MaxProviderBytes
		}
	}
	return MaxBytes
}
func (m Manifest) recordLimit() int {
	if m.Schema == ManifestSchemaV2 {
		return MaxCorpusRecords
	}
	return MaxRecords
}
func (m Manifest) journeyLimit() int {
	if m.Schema == ManifestSchemaV2 {
		return MaxCorpusJourneys
	}
	return 128
}
