# Decision 0368: Ledger negative labels for context slot weights

Date: 2026-09-23. Status: proposed (experimental delivery; ticket V1-0088). Requires owner
ratification of the invariant-4 amendment below before it can be accepted or merged as delivered.

This decision:

- adds `LTA-V0-009` to `LTA-V0-012` to `docs/specs/learned-trace-admission-v0.md`;
- adds `eval --learn-slot-weights [--goldens FILE] [--admit]` and `eval --reset-slot-weights` to an
  existing verb, and no root verb;
- adds the optional `learned_slot_weights` packet member, present only while an admitted file exists;
- changes no default `context` or `query` bytes and no `protocol/**` wire.

## Context

The unplanned-read ledger (`URE-V0`) and the self-observation ledger (`SOL-V0`) record where a
packet fell short: files an agent read outside the packet, and changed files the packet did not rank.
Those are the only local negative labels Corvint has. The `context` slot order (TCP-V0-004) is fixed.

AGENTS.md invariant 4, `SOL-V0-003`'s last sentence, and the `URE-V0` non-goals say both ledgers are
never an input to learning. The ticket asks for exactly that input, so it conflicts with accepted
owner text. An agent cannot amend that text; this decision proposes the amendment and keeps the
delivery experimental until the owner rules.

## Decision

1. Only the operator-invoked `corvint eval --learn-slot-weights` reads the ledgers as labels, through
   their existing bounded readers. `context` and `query` never open them, and `internal/contextindex`
   does not depend on either ledger package or on `internal/slotlearn` (tested).
2. Labels are distinct unplanned-read paths and `OBSERVED` miss paths, capped at 256. Planned re-reads
   are counted, not labelled. A closed table maps each label to `test`, `documentation`, `lexical`
   or `definition`; at most four proposals raise the most-labelled slots by 1 and then 2 within -2..2.
3. The frozen held-out split of the retrieval golden decides admission. The gate scores held-out
   `query` rows through the `context` packet and reports both arms and each delta. It admits a
   proposal only when at least two more cases improve than regress and critical misses do not rise,
   and only with `--admit`. Otherwise it refuses and writes nothing.
4. The admitted trace is `.context-corvint/slot-weights.json` (gitignored, at most 4,096 bytes). It
   reorders admitted rows stably before corroboration and truncation, and the packet discloses its
   digest; `batch`'s `context` operation applies it identically. The file is operator-owned: the
   loader checks its shape, including the gate's evaluation block, not its provenance. A bad file
   fails `context` closed and names the rollback.
5. `corvint eval --reset-slot-weights` is the one-command rollback to the default order.

## Proposed amendment (owner ratification required)

Proposed replacement for the last clause of AGENTS.md invariant 4, mirrored in `SOL-V0-003` and the
`URE-V0` non-goals: "neither is ever an input to ranking, evidence, or authority, nor to learning
except through the operator-invoked `corvint eval --learn-slot-weights` step, whose output reaches
ranking only after the frozen held-out gate admits it (`LTA-V0-009` to `LTA-V0-012`)."

This change does not edit AGENTS.md, `SOL-V0` or `URE-V0`. If the owner declines, revert this
decision's code; nothing persistent needs cleanup beyond an optional reset.

## Evidence

First run on this repository, recorded in `docs/BUILD-LOG.md` under V1-0088: the real ledger gives
no negative labels, and a labelled synthetic ledger is refused as not distinguished. The frozen golden
describes the Atlas fixture: its held-out `query` rows carry no path-bearing selector that names a path
in this repository, so it cannot yet admit any proposal. A useful admission needs a golden whose
held-out `symbol:`/`file:` rows name this repository's paths.

## Rollback

Operator: `corvint eval --reset-slot-weights`. Code: remove `internal/slotlearn`,
`cmd/corvint/eval_slot_weights.go`, `internal/evalrepo/slot_weights.go`,
`internal/contextindex/slot_weights.go` and the three hooks (`eval.go`, `cmd/corvint/taskcontext.go`,
`internal/contextindex/taskcontext.go`). No data migration; the ledgers are unchanged.
