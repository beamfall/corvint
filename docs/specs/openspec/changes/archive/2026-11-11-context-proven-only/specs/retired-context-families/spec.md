## REMOVED Requirements

### Requirement: TCP-V0 task context packet (whole family)

**Reason**: `corvint context --task` and its ranked packet are cut; the 2026-11-10 replay found the
packet named fewer changed files than a term grep. **Migration**: `corvint affected --base` and
`corvint impact` for a change; `corvint context --path` for a file.

### Requirement: CRK-V0 context ranking (whole family)

**Reason**: the ranking stack is cut with the packet. **Migration**: none.

### Requirement: LTA-V0 learned trace admission (whole family)

**Reason**: Corvint 1.0 learns nothing from use (decision 0596, invariant 5); the trace packages
are deleted. **Migration**: none.

### Requirement: URE-V0 unplanned read events (whole family)

**Reason**: the unplanned-read ledger fed only ranking evaluation; `internal/unplannedread` is
deleted. **Migration**: none; an existing `.corvint/unplanned-reads.jsonl` is left in place.

### Requirement: RBD-V0 retrieval bench diagnostics (whole family)

**Reason**: `tools/retrieval-bench` scored only the cut ranking. **Migration**: none.

### Requirement: CWT-V0 confidently wrong trial (whole family)

**Reason**: `tools/cw-trial` measured the cut packet. **Migration**: none.

### Requirement: HNE-V0 heading navigation experiment (whole family)

**Reason**: `tools/heading-nav-bench` is cut with the context benchmarks. **Migration**: none.

### Requirement: CEP-V0 context evolution program (whole family)

**Reason**: the program evolved the cut packet with the cut bench tools. **Migration**: none.

### Requirement: TRJ-V0 trial replay journal (whole family)

**Reason**: never started; it served only the retired trials. **Migration**: none.
