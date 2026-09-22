## 2026-09-03 — ipsa loop runs
- [x] add a /healthz endpoint (ipsa sdlc run 20260903T173148Z-3c853c11)

# Projects Noop TODOs

- [x] **Update Node Selection:** Switched `pipelinerun.yaml` to use the `workload.ness2u.xyz/gitlab-runner: "true"` label.
- [x] **Internal Build Path:** Updated `pipeline.yaml` to push via the internal cluster service (`registry.gitlab.svc.cluster.local:5000`).
- [ ] **Unified Registry Manifests:** Currently, `deployment.yaml` must use `ness-linux3.nessh:30500` for `containerd` pull compatibility. Unify this to a single `registry.local` endpoint once the infrastructure DNS/Gateway is finalized.

  > [!NOTE]
  > **Answered 2026-09-22 by `ansible-root-04`, verified from the nodes and the live cluster — the
  > two addresses are ONE registry, and this item's premise is stale.**
  >
  > `ness-linux3.nessh:30500` (pull, `k8s/deployment.yaml:22`) and
  > `registry.gitlab.svc.cluster.local:5000` (push, `k8s/tekton/pipeline.yaml:16`) are the **same
  > Docker Distribution instance**: Deployment `local-registry` / Service `registry` in namespace
  > `gitlab`, ClusterIP `10.43.72.129:5000`, NodePort `30500`, pod on **ness-server2**, store
  > `hostPath /srv/registry` (a bind mount of `/mnt/anotherdrive/registry`). The namespace is named
  > `gitlab` for historical reasons only — there is no GitLab here, just `registry:2`.
  > Source: `lab/ansible-root/roles/k3s_cluster_setup/templates/registry.yaml`; live:
  > `kubectl -n gitlab get svc,deploy,pod -o wide`.
  >
  > **The estate's registry name is `registry.nessh:30500`** — `group_vars/all/k3s.yml:29`, *"the
  > stable name — use this for anything new"*. Measured across the live cluster: **46 container
  > image refs use it**, against **17 legacy** (`ness-linux3:30500` 13, `100.222.0.7:30500` 3,
  > `ness-linux3.nessh:30500` 1 — this repo's).
  >
  > ⚠ **linux3 has not hosted the registry since 2026-09-08** (it moved to server2: linux3 was dev
  > box + build host + registry behind a 62 ms WiFi link every pull crossed). The linux3 names
  > survive only as **containerd mirror aliases** in `/etc/rancher/k3s/registries.yaml`, every one
  > of them pointing at `http://10.45.1.11:30500` (server2's LAN address) first, then the overlay.
  > So *"`deployment.yaml` must use `ness-linux3.nessh:30500` for containerd pull compatibility"* is
  > **no longer true**: the alias resolves to server2 like all the others, and switching this repo
  > to `registry.nessh:30500` costs nothing and removes one of the 17.
  >
  > **Which address to use, by where the client runs** — the distinction is DNS, not registries:
  > - **Pull, anywhere in the fleet:** any alias works; containerd rewrites before DNS is consulted.
  >   Use `registry.nessh:30500` for new manifests.
  > - **Push from inside the cluster** (Tekton, as `pipeline.yaml` does): either
  >   `registry.gitlab.svc.cluster.local:5000` or `registry.nessh:30500`.
  > - **Push from outside the cluster** (podman/buildah on a workstation): **must** be
  >   `registry.nessh:30500` — the `.svc.cluster.local` name does not resolve off-cluster. Pushes
  >   need a real record, provided by the ansible-managed `/etc/hosts` entry: `10.45.1.11` on
  >   `cgnat_hosts` (LAN), `100.222.0.9` (overlay) elsewhere.
  >
  > **What is left of this item** is therefore not "unify once DNS/Gateway is finalised" — the
  > stable name already exists and 46 refs use it. It is a one-line change here plus a decision
  > about whether the 17 legacy aliases are ever retired from `registries.yaml`. **Not made here.**
