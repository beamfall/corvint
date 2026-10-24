package doccorpus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// CapacityCode is the typed refusal of a documented corpus capacity limit
// (DCP-V1-157). Its message names the limit and the observed count.
const CapacityCode = "corpus-capacity-exceeded"

// maxPinnedInputBytes bounds the distinct input bytes one build pins other than provider record
// shards, and maxProviderRecordBytes the distinct provider record shard bytes, which are charged
// apart so that the size of the supplied record encoding cannot crowd out source inputs
// (DCX-V1-472).
const (
	maxPinnedInputBytes    = 128 << 20
	maxProviderRecordBytes = 128 << 20
)

// inputCharge is the distinct input bytes one build pins, kept per class with each input's size
// so that a refusal can name the largest contributors (DCX-V1-472).
type inputCharge struct {
	sources, providers int
	sizes              []chargedInput
}

type chargedInput struct {
	path     string
	provider bool
	size     int
}

func (c *inputCharge) add(in Input, size int) {
	provider := in.Purpose == "provider"
	if provider {
		c.providers += size
	} else {
		c.sources += size
	}
	c.sizes = append(c.sizes, chargedInput{path: in.Path, provider: provider, size: size})
}

// check refuses a charge past either bound, naming both totals and the three largest inputs of
// the class that is over.
func (c *inputCharge) check() error {
	limit, count, bound, provider := "", 0, 0, false
	switch {
	case c.sources > maxPinnedInputBytes:
		limit, count, bound = "aggregate pinned input bytes", c.sources, maxPinnedInputBytes
	case c.providers > maxProviderRecordBytes:
		limit, count, bound, provider = "aggregate provider record bytes", c.providers, maxProviderRecordBytes, true
	default:
		return nil
	}
	class := []chargedInput{}
	for _, s := range c.sizes {
		if s.provider == provider {
			class = append(class, s)
		}
	}
	sort.SliceStable(class, func(i, j int) bool {
		if class[i].size != class[j].size {
			return class[i].size > class[j].size
		}
		return class[i].path < class[j].path
	})
	largest := []string{}
	for _, s := range class[:min(3, len(class))] {
		largest = append(largest, fmt.Sprintf("%s (%d)", boundedText(s.path, 256), s.size))
	}
	err := capacityExceeded(limit, count, bound).(*Error)
	err.Message += fmt.Sprintf("; source and other inputs %d bytes (limit %d), provider record shards %d bytes (limit %d); largest: %s",
		c.sources, maxPinnedInputBytes, c.providers, maxProviderRecordBytes, strings.Join(largest, ", "))
	return err
}

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
	case *ExtractorInventory:
		// DCX-V1-560: a --schema 2 inventory carries an adoption provider record of up to
		// MaxProviderBytes plus up to MaxCorpusRecords gaps.
		if v != nil && v.Provider.Schema == AdoptionProviderSchema {
			return MaxCorpusBytes
		}
	case ExtractorInventory:
		if v.Provider.Schema == AdoptionProviderSchema {
			return MaxCorpusBytes
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
