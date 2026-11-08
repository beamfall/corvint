# Swift package boundary repair

Source review after the first V1-5113 build commit found an existing invariant
that the new file placement would violate: `TestCoreNameLiterals_LGPV0009` walks
the registry package and its subdirectories, and its runner-name literal
baseline is zero. No test execution established this failure; the test source
and new literals establish the conflict directly.

Move the Swift implementation, fixtures and tests to
`internal/testrunner/swiftselection`, outside the registry's core tree. It imports
and implements the existing registry SPI without duplicating its types or
making the registry depend on a language. No existing assertion, ratchet baseline,
manifest or selection rule changes. The public configured factory is now
`swiftselection.NewSwiftSelectionPlugin`; all behavior and FULL reasons from the
first entry remain unchanged. This supersedes that entry's registry placement.

RSP-V0-001 through RSP-V0-005 retain their positive and negative fixture tests.
The lane handback records fresh default and Darwin vet results for the final
package. Whole-package tests and independent review remain NOT_RUN under the
owner's final BUILD-ONLY instruction. This is an additive repair commit, not a
rewrite of the first commit.
