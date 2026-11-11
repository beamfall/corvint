package testdiscovery

const (
	// ExampleJoinSchema names the report.
	ExampleJoinSchema = "corvint-example-join/0"
	// MaxJoinExamples bounds the receipt examples one join reads.
	MaxJoinExamples = 2000
	// MaxJoinFiles bounds the distinct committed files one join reads.
	MaxJoinFiles = 256
	// ExampleIdentityRSpec keys an RSpec example by its spec path and scoped id.
	ExampleIdentityRSpec = "rspec-scoped-id"
	// ExampleIdentityJS keys a JS example by its file, project and full title.
	ExampleIdentityJS = "js-title-path"

	JoinReasonSourceUnbound     = "example-source-unbound"
	JoinReasonIdentityUnbound   = "example-identity-unbound"
	JoinReasonIdentityAmbiguous = "example-identity-ambiguous"
	// JoinReasonFileAmbiguous: a JS anchor file names more than one retained test-file digest and
	// none of them is the anchor file itself (V1-2085).
	JoinReasonFileAmbiguous = "example-file-ambiguous"
	// JoinReasonFileOutsideRepository: the one retained digest a JS anchor names lies outside the
	// repository root, so no committed file can bind it (V1-2085).
	JoinReasonFileOutsideRepository = "example-file-outside-repository"
)
