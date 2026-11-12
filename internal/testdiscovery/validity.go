package testdiscovery

// Uncertainty reasons of a ValidityReport (TSA-V0-046, TSA-V0-050).
const (
	ValiditySourceTooLarge   = "source-too-large"
	ValiditySourceUnparsed   = "source-unparsed"
	ValiditySerialNotLiteral = "serial-mode-not-literal"
	ValidityRSpecUnbalanced  = "rspec-block-unbalanced"
	ValidityRSpecTooDeep     = "rspec-depth-exceeded"
)
