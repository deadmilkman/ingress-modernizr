# ROADMAP

This document tracks planned work. Remove items as they are completed.

## Helm 4 postrenderer plugin support

Goal: support Helm 4 without breaking Helm 3, using the same `ingress-modernizr` binary.

### Plan

- [x] Keep current stdin/stdout renderer behavior unchanged.
- [x] Add Helm 4 plugin metadata for `type: postrenderer/v1` with `runtime: subprocess`.
- [x] Add a plugin layout (recommended: `plugin/helm4/plugin.yaml`).
- [x] Decide binary delivery for plugin use (release packaging, copy, or symlink into plugin dir).
- [x] Add a Makefile target to build a plugin-ready layout.
- [x] Update README with Helm 3 executable mode and Helm 4 plugin mode.
- [x] Verify `--post-renderer-args` pass through correctly under Helm 4.
- [x] Verify `INGRESS2GATEWAY_BIN` works when invoked by Helm 4 plugin runtime.
- [x] Test and document Helm 4 hook behavior (Helm 4 post-renders hooks by default).
- [ ] Document Helm 3 vs Helm 4 behavior differences and any caveats.

Binary delivery decision: package `plugin/helm4/plugin.yaml` with a copied binary at `plugin/helm4/ingress-modernizr` for plugin distribution.

Verification note: with Helm `v4.1.1`, the fake `ingress2gateway` shim recorded `--providers=ingress-nginx` and `--namespace=apps` after the injected `print --input-file` args.

Verification note: under Helm `v4.1.1` plugin invocation, setting `INGRESS2GATEWAY_BIN` to `scripts/test/fake-ingress2gateway.sh` was honored and produced converted output.

Hook behavior note: with Helm `v4.1.1`, non-Ingress hooks remained hooks, while an Ingress hook was removed by current `kind: Ingress` filtering and replaced only by converted output.

### Testing plan

- [ ] Add unit tests for YAML stream behavior:
  - multi-document input
  - empty documents
  - malformed YAML errors
  - non-Ingress resources preserved
  - Ingress resources removed
  - converted resources appended
- [ ] Add a fake `ingress2gateway` helper for tests to verify:
  - `INGRESS2GATEWAY_BIN` override is honored
  - `print` is always invoked
  - leading user-supplied `print` is stripped
  - `--input-file` receives the full rendered stream
  - provider args (for example `--providers=ingress-nginx`) are forwarded
- [ ] Add Helm 3 integration checks with standalone executable post-renderer.
- [ ] Add Helm 4 integration checks with installed `postrenderer/v1` subprocess plugin.
- [ ] Prefer a pinned Helm 4 Docker image in CI if a reliable image exists.
- [ ] If no suitable Helm 4 image exists, download a pinned Helm 4 release binary in CI.
- [ ] Keep Helm 4 integration checks skippable for local development when Helm 4 is unavailable.
- [ ] Add explicit Helm 4 hook coverage (including behavior decision and documentation).

### Sample chart integration fixture

- [ ] Render `samples/sample` in tests and include it in Helm 3 and Helm 4 integration checks.
- [x] Fix `samples/sample` so default Ingress values match `templates/ingress.yaml`.
- [x] Verify fixed sample chart renders a valid `networking.k8s.io/v1` Ingress.
- [x] Confirm sample render still includes non-Ingress resources (Service, Deployment, ServiceAccount, test hook Pod).
- [ ] Use additional tiny inline fixtures for focused unit tests where sample chart would be too broad.

### Acceptance criteria

- [ ] `make build` still produces the standalone Helm 3-compatible binary.
- [ ] Helm 3 works with `--post-renderer ./dist/ingress-modernizr`.
- [ ] Helm 4 works with installed plugin and `--post-renderer ingress-modernizr`.
- [ ] Provider args reach `ingress2gateway` in both Helm versions.
- [ ] Existing stdin/stdout manual usage remains valid.
- [ ] Unit tests cover parser/filter/assembly behavior.
- [ ] Integration checks cover Helm 3 executable mode and Helm 4 plugin mode.
- [ ] CI pins the Helm 4 version used for plugin verification.
- [ ] Helm 4 hook behavior is tested and documented.
- [x] `samples/sample` renders a valid Ingress with default values.
