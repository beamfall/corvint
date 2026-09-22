package tcq

// Resource bounds from TCQ-V0-041. Exceeding any TCQ-owned bound is
// `tcq-resource-exhausted` and yields no partial output.
const (
	maxOCMBytes         = 1 << 20
	maxCommandBytes     = 64 << 10
	maxObservationBytes = 4 << 20
	maxResultBytes      = 4 << 20
	maxReportBytes      = 4 << 20

	maxSelectedEdges         = 512
	maxUnits                 = 512
	maxIssues                = 4096
	maxSourceBlobBytes       = 1 << 20
	maxSourceBytes           = 16 << 20
	maxPythonNodes           = 100_000
	maxGoTokens              = 100_000
	maxCollisionUnitsPerBlob = 10_000
	maxCollisionUnits        = 20_000

	maxTestcases         = 10_000
	maxXMLElements       = 50_000
	maxXMLDepth          = 32
	maxXMLAttributes     = 64
	maxXMLAttributeBytes = 1024

	maxJSONNumberBytes = 64
)

// jsonBounds pairs the depth and aggregate member limits TCQ-V0-041 assigns to
// one artifact kind.
type jsonBounds struct {
	bytes   int
	depth   int
	members int
}

var (
	ocmBounds         = jsonBounds{maxOCMBytes, 16, 20_000}
	commandBounds     = jsonBounds{maxCommandBytes, 8, 256}
	observationBounds = jsonBounds{maxObservationBytes, 8, 30_000}
	resultBounds      = jsonBounds{maxResultBytes, 12, 50_000}
)

// cemBounds is the inherited CEM 0.2 ceiling TCQ-V0-041 defers to.
var cemBounds = jsonBounds{4 << 20, 32, 100_000}
