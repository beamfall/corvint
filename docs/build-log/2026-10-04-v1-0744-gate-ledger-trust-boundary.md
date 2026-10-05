## 2026-10-04 V1-0744: the gate-ledger record directory is the trust boundary

Human-owned intent: the owner asked to start V1-0744. That ticket asked whether gate-ledger
should skip a step for a record that carries only `schema` and `key`. On 2026-10-04 the owner chose
the owner-only directory as the accepted boundary.

### Question

The question was filed from the V1-0712 output-class inventory as a suspected defect, not a
confirmed one.

- `lookup` (`tools/gate-ledger/main.go`) reads a record with an unbounded `os.ReadFile` and decodes
  it leniently.
- It returns HIT when `schema` is `gate-ledger/1` and `key` matches.
- So a two-field file written into the ledger directory skips that gate step.

### Decision

The private, owner-owned record directory that `GL-V0-006` requires is the trust boundary.

- Only a process running as the owner can write there, and such a process can forge a complete
  record as easily as a two-field one.
- Strict decoding, a byte bound, or required fields would only reject stray files. They would not
  close the forgery path.
- `record` writes by atomic rename, so the tool never leaves a partial record itself.

The spec already said that only key equality matters. `gate-ledger-v0.md` now states this boundary
and its reason in the trust-boundary section. Code, requirements and tests are unchanged.

### Evidence and limits

- Spec text only; no runtime behavior changed. The doc gates pass.
- The dogfood CEM loop was not run: `NOT_PRODUCED`, a documentation-only decision record with no
  code hunk.
- Ticket acceptance criterion 2 applies only if records must be self-consistent. Under this
  decision it does not apply.
