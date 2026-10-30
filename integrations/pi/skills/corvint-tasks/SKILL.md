---
name: corvint-tasks
description: Inspect the native Corvint Tasks backlog, tickets, gates and releases. Use when asked about open work, a ticket's state, blockers or release readiness.
license: Apache-2.0
---

# Corvint Tasks

Reads use `corvint_tasks` with an `operation` and `input`:

- `search`: `status`, `kind`, `priority`, `owner`, `milestone`, `label`, `text`, `offset`,
  `limit` (1 to 100) and `summary` (boolean). Example:
  `{"operation":"search","input":{"status":"OPEN","label":"bugs","summary":true,"limit":20}}`.
- `list` (`status`, `offset`, `limit`), `ticket`, `blockers`, `attempt`, `gate`, `release`,
  `readiness` (each with `id`), `queue`, `roadmap`, `gates`, `releases`, `plan`, `audit`, `help`.

The model cannot write the task store. Claims, renewals, submissions, gate runs and completion go
only through the operator's explicit `/corvint-tasks` command, which checks native capability and
reconciles before it dispatches. Suggest that command to the user; never claim a ticket is done.

Other Corvint capabilities: the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
