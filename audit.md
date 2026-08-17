# Audit — noop

## 2026-08-17: SOP Sync (ledger-wide audit)
- `.mimir/` gitignored; seeded this audit log. Project is stable/complete for
  current needs (chaos/dummy server with /download and /throughput endpoints).

## Earlier (from git history)
- Per-connection rolling throughput history (10 req / 10s), ASCII-encodable
  download/throughput outputs with exact byte counts, pacing fixes.
- Tekton pipeline moved to gitlab-runner-labeled nodes; internal registry push path.
