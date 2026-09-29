package doccorpus

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
		if v.Schema == AdoptionProviderSchema {
			return MaxProviderBytes
		}
	case *ProviderRecord:
		if v != nil && v.Schema == AdoptionProviderSchema {
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
