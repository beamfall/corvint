package wire

// Frozen numeric limits (SPEC §1). Every value is validated before any
// effect; exceeding one fails closed with LIMIT_EXCEEDED.
const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB

	MaxIdentifierBytes = 128
	MaxLabelBytes      = 64
	MaxLocalTokenBytes = 64
	MaxRequestIDBytes  = 64
	// MaxPathTextBytes bounds a PathText: the absolute filesystem path
	// recorded as head.primaryWorktree and manifest.primaryWorktree (§2,
	// B2 resolution). It is separate from the Identifier bound.
	MaxPathTextBytes = 4096

	MaxTitleBytes     = 512
	MaxBodyBytes      = 64 * KiB
	MaxCriterionBytes = 4 * KiB
	MaxProseBytes     = 64 * KiB

	MaxTicketFileBytes       = 128 * KiB
	MaxReleaseFileBytes      = 256 * KiB
	MaxMutationEnvelopeBytes = 256 * KiB
	MaxOutcomeBytes          = 64 * KiB

	MaxAcceptanceCriteria = 64
	MaxDependencies       = 64
	MaxRequirementRefs    = 64
	MaxResources          = 64
	MaxApprovals          = 64
	MaxTouchPaths         = 256
	MaxLabels             = 32
	MaxHolds              = 16
	MaxRequiredGates      = 32
	MaxCapabilities       = 32

	MaxTicketsPerQueue  = 10000
	MaxReleasesPerQueue = 1000
	MaxIntentTreeBytes  = 256 * MiB
	MaxQueueFileBytes   = 1 * MiB
	MaxPolicyFileBytes  = 256 * KiB
	MaxImportMapBytes   = 8 * MiB
	MaxImportMapEntries = 10000

	MaxReceiptFileBytes     = 1 * MiB
	MaxInlinePostEntryBytes = 64 * KiB
	MaxInlinePostEntries    = 8
	MaxAttemptRecordBytes   = 64 * KiB
	MaxJournalHeadBytes     = 4 * KiB
	MaxBarrierBytes         = 4 * KiB
	MaxCommandResultBytes   = 64 * KiB // excluding paginated items
	MaxListResultBytes      = 16 * MiB
	MaxReservationSetBytes  = 64 * MiB
	MaxEvidenceBlobBytes    = 64 * MiB
	MaxEvidenceStoreBytes   = 16 * GiB
	MaxImportPlanBytes      = 16 * MiB
	MaxArchiveBytes         = 64 * GiB
	MaxPinnedBytes          = 16 * MiB // largest pinned document (an import plan or plan)

	// Archive capacity (§1, §3.5; B4 resolution, unverified implementation
	// freeze). The journal saturates at 1,000,000 receipts, so a manifest
	// must carry more `files` entries than the ordinary 10,000 decoded-array
	// bound. These three numbers are independent hard aggregate budgets;
	// they are checked on input bytes, entry counts and decoded nodes before
	// any manifest is materialized, and no other document is parsed under
	// them (ParseWith is opt-in; Parse keeps MaxJSONArrayElements).
	//
	// MaxArchiveFiles bounds the `files` array of one taskman-archive/0
	// manifest and the number of files `archive export` will emit.
	MaxArchiveFiles = 2100000
	// MaxArchiveManifestBytes bounds manifest.json on the wire. Arithmetic
	// (largest valid entry, every byte counted): `{"bytes":"` 9 + Size 20 +
	// `","path":"` 10 + encoded path ≤256 (128 Identifier bytes, each `"`
	// escaped to two bytes; backslash and controls are refused by Path) +
	// `","sha256":"` 12 + Digest 64 + `"}` 2 + `,` 1 = 374 bytes per entry;
	// 374 × 2,100,000 = 785,400,000. Envelope outside `files`: thirteen
	// fixed keys and punctuation < 300 bytes, five Digests 320, three Sizes
	// 60, profile 17, queueId ≤128, primaryWorktree ≤8,192 (4,096 PathText
	// bytes, each `"` escaped), `[`/`]`/LF 3: < 9,100 bytes. Worst case
	// 785,409,100 < 768 MiB = 805,306,368; the proposed 512 MiB
	// (536,870,912) would not hold it, so the cap is 768 MiB.
	MaxArchiveManifestBytes = 768 * MiB
	// MaxArchiveScanEntries bounds the directory entries `archive export`
	// enumerates under the state dir (directories, skipped temp names and
	// unexpected names included) before any entry is stat'ed or opened. It
	// is counted separately from the exported-file count, which is bounded
	// by MaxArchiveFiles; the 4,096 headroom covers the fixed layout
	// directories and a bounded number of crashed temp files.
	MaxArchiveScanEntries = MaxArchiveFiles + 4096

	MaxGateArgv         = 16
	MaxRuntimeArgv      = 16
	MaxActiveAttempts   = 64
	MaxWorkersTotal     = 256
	MaxAdmissionsPerRev = 3
	MaxRepairRounds     = 2
	MaxMalformedRetry   = 1
	MaxGateRerunStale   = 1
	MaxReconcileAttempt = 3
	MinEvidenceDays     = 30
	MaxLaneWallMinutes  = 240
	MaxGateTimeoutSecs  = 120 * 60

	PageDefault = 100
	PageMax     = 1000

	MaxJSONDepth         = 24
	MaxJSONArrayElements = 10000
	MaxJSONNodes         = 250000

	MaxCountValue = 2147483647
	VersionSuffix = "/0"
)
