## 2026-09-03 — ipsa loop runs
- [x] add a /healthz endpoint (ipsa sdlc run 20260903T173148Z-3c853c11)

# Projects Noop TODOs

- [x] **Update Node Selection:** Switched `pipelinerun.yaml` to use the `workload.ness2u.xyz/gitlab-runner: "true"` label.
- [x] **Internal Build Path:** Updated `pipeline.yaml` to push via the internal cluster service (`registry.gitlab.svc.cluster.local:5000`).
- [ ] **Unified Registry Manifests:** Currently, `deployment.yaml` must use `ness-linux3.nessh:30500` for `containerd` pull compatibility. Unify this to a single `registry.local` endpoint once the infrastructure DNS/Gateway is finalized.
