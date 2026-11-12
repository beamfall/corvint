package testsource

import (
	"context"

	"github.com/Beamfall/corvint/internal/testdiscovery"
)

// The static test-evidence producer behind `corvint docs corpus discover-tests`
// (DCX-V1-370..376, GitHub #1582). It reads committed Playwright or RSpec test files at one full
// commit through the TSA-V0 readers and emits, per example, its title path, tags, annotations,
// expectations, navigations and requests, statically reached modules, fixtures and shared
// groups, and the hooks that apply to it. Every fact carries a line anchor pinned to the blob;
// what the readers cannot classify is an unknown with a reason, never a guess. The record is
// generated trust: the producer sets no review verdict.

// TestDiscoverySchema is testdiscovery.TestDiscoverySchema, kept under its old name.
const TestDiscoverySchema = testdiscovery.TestDiscoverySchema

// maxDiscoveryFacts is testdiscovery.MaxDiscoveryFacts, kept under its old name.
const maxDiscoveryFacts = testdiscovery.MaxDiscoveryFacts

// discoverReasonComputedLookup is testdiscovery.DiscoverReasonComputedLookup, kept under its old name.
const discoverReasonComputedLookup = testdiscovery.DiscoverReasonComputedLookup

// discoverReasonImportUnresolved is testdiscovery.DiscoverReasonImportUnresolved, kept under its old name.
const discoverReasonImportUnresolved = testdiscovery.DiscoverReasonImportUnresolved

// discoverReasonRegistry is testdiscovery.DiscoverReasonRegistry, kept under its old name.
const discoverReasonRegistry = testdiscovery.DiscoverReasonRegistry

// discoverReasonTargetNotLiteral is testdiscovery.DiscoverReasonTargetNotLiteral, kept under its old name.
const discoverReasonTargetNotLiteral = testdiscovery.DiscoverReasonTargetNotLiteral

// discoverReasonTitleNotLiteral is testdiscovery.DiscoverReasonTitleNotLiteral, kept under its old name.
const discoverReasonTitleNotLiteral = testdiscovery.DiscoverReasonTitleNotLiteral

// discoverReasonMatcherUnread is testdiscovery.DiscoverReasonMatcherUnread, kept under its old name.
const discoverReasonMatcherUnread = testdiscovery.DiscoverReasonMatcherUnread

// discoverReasonMetadata is testdiscovery.DiscoverReasonMetadata, kept under its old name.
const discoverReasonMetadata = testdiscovery.DiscoverReasonMetadata

// discoverReasonProjectNotStatic is testdiscovery.DiscoverReasonProjectNotStatic, kept under its old name.
const discoverReasonProjectNotStatic = testdiscovery.DiscoverReasonProjectNotStatic

// discoverReasonLoadPath is testdiscovery.DiscoverReasonLoadPath, kept under its old name.
const discoverReasonLoadPath = testdiscovery.DiscoverReasonLoadPath

// discoverReasonFixtureBody is testdiscovery.DiscoverReasonFixtureBody, kept under its old name.
const discoverReasonFixtureBody = testdiscovery.DiscoverReasonFixtureBody

// discoverReasonFactBound is testdiscovery.DiscoverReasonFactBound, kept under its old name.
const discoverReasonFactBound = testdiscovery.DiscoverReasonFactBound

// discoverReasonFileTooLarge is testdiscovery.DiscoverReasonFileTooLarge, kept under its old name.
const discoverReasonFileTooLarge = testdiscovery.DiscoverReasonFileTooLarge

// discoverReasonScopedLoop is testdiscovery.DiscoverReasonScopedLoop, kept under its old name.
const discoverReasonScopedLoop = testdiscovery.DiscoverReasonScopedLoop

// discoverReasonScopedInline is testdiscovery.DiscoverReasonScopedInline, kept under its old name.
const discoverReasonScopedInline = testdiscovery.DiscoverReasonScopedInline

// discoverReasonSharedDefinition is testdiscovery.DiscoverReasonSharedDefinition, kept under its old name.
const discoverReasonSharedDefinition = testdiscovery.DiscoverReasonSharedDefinition

// discoverReasonFactUnanchored is testdiscovery.DiscoverReasonFactUnanchored, kept under its old name.
const discoverReasonFactUnanchored = testdiscovery.DiscoverReasonFactUnanchored

// TestDiscoveryInput is testdiscovery.TestDiscoveryInput, kept under its old name.
type TestDiscoveryInput = testdiscovery.TestDiscoveryInput

// TestDiscovery is testdiscovery.TestDiscovery, kept under its old name.
type TestDiscovery = testdiscovery.TestDiscovery

// DiscoveredExample is testdiscovery.DiscoveredExample, kept under its old name.
type DiscoveredExample = testdiscovery.DiscoveredExample

// DiscoveredAnnotation is testdiscovery.DiscoveredAnnotation, kept under its old name.
type DiscoveredAnnotation = testdiscovery.DiscoveredAnnotation

// BuildTestDiscovery is testdiscovery.BuildTestDiscovery, kept under its old name.
func BuildTestDiscovery(ctx context.Context, input TestDiscoveryInput) ([]byte, error) {
	return testdiscovery.BuildTestDiscovery(ctx, input)
}

// discoveryLocated is testdiscovery.DiscoveryLocated, kept under its old name.
const discoveryLocated = testdiscovery.DiscoveryLocated
