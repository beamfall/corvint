# Experimental heading navigation comparison

This is a standalone offline experiment, not a production retrieval feature.
Run from the repository root, with an output directory that does not exist:

```sh
GOTOOLCHAIN=local go run ./tools/heading-nav-bench \
  --fixture tools/heading-nav-bench/testdata/pilot.json \
  --budget 6000 --output /tmp/heading-navigation-trial
```

The fixture has `profile: heading-navigation-fixture/0`, a full immutable
`revision`, RFC3339 `timestamp`, sorted unique Markdown `paths`, and `cases`.
Each case has `id`, `query`, `gold: [{path,start,end}]` (inclusive lines), and
`answer_notes`. An empty gold list has undefined recall, never success.

Both arms invoke the actual native `doccorpus.Query` search on a corpus built
from merged per-file `Inventory` manifests. Subjects are collapsed to documents
in first-result order. The baseline is native lexical document ranking followed
by a sequential prefix-read consumer policy, not an upper bound on Corvint's
production retrieval. The heading arm uses the same ranked documents. Within
each document it sorts non-overlapping section bodies by query term overlap,
using `contextindex.EvidenceTerms`; source order breaks ties. Preamble and
ancestor introductory bodies precede selected child bodies. No LLM chooses
sections. Body content also contributes lexical terms; this is an offline policy
comparison, not a claim that an agent can discover those terms without reading.

The flat parser supports ATX and conservative single-line setext headings,
ignores backtick and tilde fenced code, and partitions preamble, parent intros,
and residual lines. Container headings and full CommonMark interpretation are
unsupported. Duplicate titles remain distinguished by path, line and ancestry.

Packets contain the question, pinned revision, blob identities, original exact
line spans and source text, selected heading labels, omissions and limitations.
The exact JSON byte budget includes all metadata, escaped source text and the
self-reported byte count and final LF. Heading metadata is capped at one quarter of the
budget. Lines are never partially emitted. A section/document stops at its first
unfittable line; subsequent sections/documents may still fit. All scoped but unread
lines and omitted heading labels are counted, including unranked documents.
An envelope that cannot fit is an error. Gold and answer notes never enter the
retrieval function or packets.

`report.json` contains fixture/corpus digests, builder identity limitations,
timings, bytes and full-gold-span union recall for each disjoint arm. All gold
spans are considered critical. Outline labels alone do not count as reading.
Answer correctness, authority errors and unsupported answers are `NOT_OBSERVED`
until a separate paired agent reading trial. The source revision pins the
corpus, not an uncommitted harness; retain harness commit/diff separately.
Existing output directories are refused. An interrupted/failed run can leave a
partial output directory; only a run with `report.json` is complete.

## Retained pilot and reader replay

`results/pilot/` retains the frozen first valid local pilot: original packets,
retrieval report, paired reader responses/usage, and the explicit root assessment.
`testdata/pilot.json` is author-visible development data, not a blind holdout.
`testdata/rubric.json` freezes answer atoms and scoring before readers run.
The first pretrial fixture digest was
`a658120d19473fd9bda118f76b3ea0a46a9bbc40b9a0e70ade51ffe4dfa6c9cd`;
independent review corrected an incomplete SDD gold span before any trial,
yielding the retained digest in `report.json`. No results informed that repair.

To replay reading, start a fresh ephemeral Codex CLI process for **each** question
and arm in an empty external directory. Use the argument template and exact prompt
assembly in `reader-report.json`, `testdata/reader-prompt.txt`, and the retained
answer schema. Alternate which arm runs first per case. Do not provide fixture
answers, prior cases or repository context. Any tool use invalidates the reader;
retain failed attempts. This prompt constraint is not a sandbox enforcement claim.
The original run used Codex CLI 0.153.2 and gpt-5.6-sol/low, with a 180-second
per-reader timeout and owned-process cleanup. Raw CLI event logs remain private;
the report retains answers, exact packet/prompt/rubric hashes, usage observations,
wall times, zero tool-event counts and output hashes. Reported model usage includes
harness overhead; it is not a billing receipt. One run cannot establish a speed or
cost improvement, and unequal cache hits make such attribution especially unsafe.
