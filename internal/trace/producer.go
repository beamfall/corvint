package trace

// SchemaVersionV3 adds the closed producer member to either verification shape
// (LTPM-V0-015). Producer is part of the row identity, so the same observation
// recorded by two producers yields two distinct trace IDs.
const SchemaVersionV3 = 3

// The closed producer set. ProducerUnknown is a read-side label for schema-1
// and schema-2 rows; it is never stored.
const (
	ProducerCLI     = "cli"
	ProducerDogfood = "dogfood"
	ProducerPiTool  = "pi-tool"
	ProducerUnknown = "UNKNOWN"
)

// Producers lists the closed stored set followed by the read-side label, in
// report order.
var Producers = []string{ProducerCLI, ProducerDogfood, ProducerPiTool, ProducerUnknown}

// ValidProducer reports whether value may be stored in a schema-3 row.
func ValidProducer(value string) bool {
	return value == ProducerCLI || value == ProducerDogfood || value == ProducerPiTool
}

// ProducerName returns the stored producer, or ProducerUnknown for rows written
// before schema 3.
func (record Record) ProducerName() string {
	if record.SchemaVersion == SchemaVersionV3 {
		return record.Producer
	}
	return ProducerUnknown
}

// ReportableProducer reports whether value names a producer a report may count
// or exclude: the stored set plus the read-side UNKNOWN label.
func ReportableProducer(value string) bool {
	return ValidProducer(value) || value == ProducerUnknown
}

// CountProducers returns one count per reportable producer, zeros included
// (LTPM-V0-016).
func CountProducers(producers []string) map[string]int {
	counts := make(map[string]int, len(Producers))
	for _, name := range Producers {
		counts[name] = 0
	}
	for _, name := range producers {
		counts[name]++
	}
	return counts
}

// RecordProducers returns each record's read-side producer, in order.
func RecordProducers(records []Record) []string {
	names := make([]string, len(records))
	for offset, record := range records {
		names[offset] = record.ProducerName()
	}
	return names
}

// WithoutProducers returns the records whose producer is not excluded, in
// order. It copies; the supplied records and the store are never changed.
func WithoutProducers(records []Record, excluded []string) []Record {
	kept := make([]Record, 0, len(records))
	for _, record := range records {
		if !containsProducer(excluded, record.ProducerName()) {
			kept = append(kept, record)
		}
	}
	return kept
}

func containsProducer(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}
