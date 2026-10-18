# AGENTS.md — noop

Read this first on entry; it overrides the general rules above it. *Seeded by `ipsa sop init` on 2026-10-18: the layout from the tree, the rules copied from the parent.*

## Layout

- `k8s/`
- `wiki/`

## AI Interaction Rules

1. **`~/git/work/` is absolutely off-limits.** Never read, summarize, or reference it. This rule covers that directory only: ness's work **backup** under `/mnt/data/backups/work-backup/hot/<date>` is NOT covered, and he has authorized an agent to read and ingest it (ness, 2026-09-11; see `todo.md`, "One concerted pass over the new work backup").
2. **Read the local AGENTS.md first** when entering any subdirectory — it overrides general rules.
3. **Enforce identity:** Use `jharkness@gmail.com` for any git config or personal references.
4. **Multi-arch:** Assume AMD64 + ARM64 mixed environment in `lab/` and `projects/`.
5. **Rust runtime isolation:**
   - `projects/rust1` → `smol` only. Never introduce `tokio`.
   - `projects/my-server` → `tokio` + Axum only. No `smol`.
6. **Code preservation:** Do not delete unused code without explicit approval. *Unused* means not yet used, disabled or abandoned. **Fully superseded code (replaced by something that does the same job) may be removed without asking** (ness, 2026-08-17; Canon T02), in the commit that lands the replacement, saying what superseded it.
7. **Commits only after successful builds and passing tests.**

---

*Copied from `/home/ness/git/CLAUDE.md` on 2026-10-18; that file governs. Add to it here only what this project needs beyond it.*
