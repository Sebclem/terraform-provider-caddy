# Null Root Configuration Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the Caddy Terraform/OpenTofu provider to automatically initialize an empty root configuration object `{}` when Caddy is started with a null configuration document, allowing seamless resource creation without requiring an external bootstrap Caddyfile.

**Architecture:** When `Client.Mutate` targets root scope `["config"]` and encounters `snap.Value == nil`, it executes a conditional `POST` with an empty object `{}` using the server's ETag, then re-fetches the snapshot to proceed with the mutation against the initialized root object. The provider engine and settings resources are updated to allow root object creation, and the specification documents in `spec/design.md` and related docs are aligned.

**Tech Stack:** Go (Go 1.24+ / standard library), Terraform Plugin Framework, OpenTofu.

**Spec:** `/home/sclement/Documents/Gitea/terraform-provider-caddy/spec/design.md`

## Global Constraints

- Never perform full configuration replacement (`/load`) or touch the admin listener configuration.
- Mutations must remain guarded by ETags (`If-Match`) and serialized per endpoint using `endpointLocks`.
- Preserve existing resource schemas, attribute behaviors, and identity generation.
- All Go unit tests in `terraform-provider-caddy` (`go test ./...`) must pass cleanly.

## Review Focus

- Null root detection: verify `snap.Value == nil` on `scope = ["config"]` triggers `POST /config/` with `{}` and succeeds.
- Non-root null value: verify `snap.Value == nil` on non-root paths (e.g. `@id` lookups) does not attempt to initialize `{}` on those paths.
- Concurrency: verify that multiple concurrent resource creations on a null-root instance do not conflict or cause duplicate initializations beyond normal retry semantics.
- Readback after empty root write: ensure `verifiedWrite` / `afterWrite` state does not trigger false `ErrUncertainWrite` when evaluating the actual resource mutation.
- Documentation consistency: verify `spec/design.md`, `README.md`, and docs correctly describe the null-root initialization behavior.

---

### Task 1: Update Specification and Documentation

**Files:**
- Modify: `spec/design.md:36`
- Modify: `README.md:9-17`
- Modify: `docs/review.md:12`
- Modify: `docs/migration.md:9`

**Interfaces:**
- Consumes: User requirement that Caddy is bootstrapped with a null config and admin is managed by env var.
- Produces: Updated specification declaring automatic empty root initialization.

- [ ] **Step 1: Update `spec/design.md`**
  Update line 36 of `spec/design.md` to state that when Caddy starts with a null configuration document (with admin managed via environment variables like `CADDY_ADMIN`), the provider automatically initializes an empty root configuration object (`{}`) via `POST /config/`.

- [ ] **Step 2: Update `README.md`, `docs/review.md`, and `docs/migration.md`**
  Update documentation in `README.md`, `docs/review.md`, and `docs/migration.md` to replace the requirement for an external admin-only Caddyfile with documentation of automatic empty root initialization when Caddy exposes a null root.

- [ ] **Step 3: Verify documentation changes**
  Check git diff to ensure clear, consistent documentation of the null root behavior across all documentation files.

- [ ] **Step 4: Commit documentation changes**
  ```bash
  git add spec/design.md README.md docs/review.md docs/migration.md
  git commit -m "docs: document automatic empty root bootstrap for null Caddy configuration"
  ```

---

### Task 2: Implement Empty Root Bootstrap in `internal/caddy/mutate.go` and Update `internal/provider/engine.go`

**Files:**
- Modify: `internal/caddy/mutate.go:40-47`
- Modify: `internal/provider/engine.go:95-98`
- Test: `internal/caddy/mutate_test.go`
- Test: `internal/provider/bootstrap_test.go`

**Interfaces:**
- Consumes: `c.Get`, `c.Write`, `snap.Value`, `snap.ETag`
- Produces: `Client.Mutate` seamlessly handling `snap.Value == nil` by initializing `{}` when `scope` is `["config"]`.

- [ ] **Step 1: Write failing test in `internal/provider/bootstrap_test.go`**
  Update `TestBootstrapDiagnostic` in `internal/provider/bootstrap_test.go` to `TestNullRootBootstrap`, simulating a Caddy server that initially returns `null` for `GET /config/`, accepts `POST /config/` with `{}` to transition its state to an empty object `{}`, and accepts subsequent object creation (`POST /config/apps`). Verify that `createObject` succeeds without error and that the empty root write occurred.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/provider -run TestNullRootBootstrap`
  Expected: FAIL with "Caddy has no configuration document..."

- [ ] **Step 3: Implement empty root bootstrap in `internal/caddy/mutate.go`**
  In `(*Client).Mutate`:
  When `snap.Value == nil && (len(scope) == 0 || (len(scope) == 1 && scope[0] == "config"))`:
  Execute `err = c.Write(ctx, "POST", scope, map[string]any{}, snap.ETag)`. If err is non-nil, return it; otherwise `continue` to refresh snapshot and proceed with reconciliation.
  In `internal/provider/engine.go`:
  Remove the `s.Value == nil` error check from `createObject` (or keep a defensive check only for corrupted non-object values).

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/provider -run TestNullRootBootstrap`
  Expected: PASS

- [ ] **Step 5: Run all package tests**
  Run: `go test ./...` in `terraform-provider-caddy`
  Expected: all tests PASS

- [ ] **Step 6: Commit code changes**
  ```bash
  git add internal/caddy/mutate.go internal/provider/engine.go internal/provider/bootstrap_test.go
  git commit -m "feat: automatically initialize empty root configuration when Caddy config is null"
  ```

---

### Task 3: Build and Verify Provider with OpenTofu Infrastructure

**Files:**
- Output binary: `opentofu-proxy-config/.tools/build/terraform-provider-caddy`
- Mirror binary: `opentofu-proxy-config/.tools/provider-mirror/local.sebclem.fr/sebclem/caddy/0.1.0/linux_amd64/terraform-provider-caddy_v0.1.0`

**Interfaces:**
- Consumes: Built provider binary from Task 2.
- Produces: Working `tofu plan` and clean behavior against Caddy instance with null/empty root.

- [ ] **Step 1: Rebuild provider using `scripts/setup-provider.sh`**
  Run `./scripts/setup-provider.sh` from `/home/sclement/Documents/Gitea/opentofu-proxy-config`.

- [ ] **Step 2: Run `tofu plan` in `opentofu-proxy-config`**
  Run `tofu plan` to verify that the newly built provider runs and produces a valid plan without errors.

- [ ] **Step 3: Reset Caddy configuration on test server to null and test `tofu apply`**
  Reset live Caddy `/config/` on `reverse-proxy.home:2019` to null (or test in unit/acceptance tests) to verify end-to-end that the provider successfully initializes the empty root object and creates Caddy resources.
