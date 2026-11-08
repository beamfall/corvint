# V1-4846: exact-node help and machine catalog reachability

The owner requested every inventory node's own help and machine catalog entry in the
2026-11-08 clicat charter. CAP-V0-8800 strengthens the existing CAP-V0-9300 discovery
profile and CAP-V0-045/047 catalog gate. The supplied base is
`9761ea1f459c0d3b3c963953e24a76eaeda16370`.

The base already contains 55 binary trees, 618 nodes and an empty CLI ratchet. Those are
static inventory counts, not runtime qualification. Its whole-profile process test only
checked individual usage/flag/input/example fields on root nodes. A literal `help` node
therefore escaped detection when `help --help` returned the root page. Exact-node help now
takes precedence over the general `help COMMAND` alias. The adapter and shared generated
help handler keep unknown paths and native compatibility dispatch unchanged.

The added table checks all inventory nodes and both help spellings, every node's fields,
unknown children, and a reader that fails on stdin access. It first checks the independent
binary/dispatch inventory and root machine links; dropping a row, tree or link cannot reduce
the checked denominator silently. The existing compiled profile test now checks individual
fields at every depth. The catalog gate also rejects missing, duplicate, stale and mismatched
tree links and malformed/unsupported root catalogs. Synthetic negative cases retain the
existing read-only gate assertion.

Decision: retain the native compatibility exceptions already accepted in CAP-V0-7700/9300.
Do not add MCP descriptors to an unrelated existing catalog field or invent CLI flags for
protocol tools. The frozen root catalog has CLI commands and tree references, not per-tool
MCP descriptors. Existing `tools/list` responses and their goldens remain untouched. The
literal universal-native-help and root-MCP-entry acceptance is PARTIAL pending owner approval
of an exact catalog extension and any native compatibility changes. Documentation now states
that boundary and points to configured-server `tools/list`; shipped skills already point to
the capability page and require no package changes.

Validation in this build-only lane: formatting and vet of `internal/cliapp`,
`internal/cliprofile`, `tools/capabilities-check`, and `conformance/cli-errors-v0`.
All returned 0. The spec generator's write/check, spec README/worktree checks and
`make -s line-citations-check` returned 0; citation repin found no moved anchors.
The full `spec-requirements` and `spec-requirements-check` Make targets invoke Go tests;
only their shell generation/drift checks ran, preserving the build-only restriction.
Runtime tests, compiled process conformance, independent review, affected selection and CEM
binding/checking are NOT_RUN or NOT_PRODUCED, delegated to the orchestrator by the final
build-only instructions. The handback records exact commands and check results. No test pass
or complete ticket disposition is claimed. The native context/dogfood route is deferred under
that instruction; source/spec reading is used, and live providers, learning and performance
evaluation are not applicable to this discovery-only correction.

Rollback: revert this change's help precedence, root-link gate and added assertions, retaining
the preexisting command trees, protocol controls and zero ratchet. No wire schema, error code,
exit code, stderr contract, committed golden or core-freeze output changed.
