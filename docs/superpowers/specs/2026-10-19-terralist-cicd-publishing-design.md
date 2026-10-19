# Terralist CI/CD Publishing Design

## Intent and Why

Migrate the release and publishing pipeline for `terraform-provider-caddy` from the public OpenTofu Registry (via GitHub mirror) to a private Terralist registry instance hosted at `terralist.sebclem.fr`. Release orchestration will run natively on Gitea Actions on `git.sebclem.fr`, removing reliance on GitHub Actions and external repository mirroring.

## Goals

1. **Native Gitea CI/CD:** Automate releases via Gitea Actions triggered on version tag pushes (`v*`).
2. **GoReleaser Packaging & Gitea Release:** Cross-compile provider binaries, inject the new provider registry address (`terralist.sebclem.fr/sebclem/caddy`), generate `SHA256SUMS`, sign with GPG, and create a public release on Gitea (`git.sebclem.fr/sebclem/terraform-provider-caddy`).
3. **Terralist Provider Ingestion:** Notify Terralist (`https://terralist.sebclem.fr/v1/api/providers/sebclem/caddy/:version/upload`) with metadata pointing to the public Gitea release asset URLs, allowing Terralist to ingest the provider packages and verify GPG signatures.
4. **Codebase Updates:** Update embedded default addresses in `main.go` and `.goreleaser.yaml`.
5. **Documentation:** Rewrite `docs/releasing.md` detailing the Gitea Actions workflow, prerequisite secrets, and verification steps.

## Non-Goals

- Publishing releases to the OpenTofu public registry or maintaining a GitHub mirror.
- Automating Terralist authority creation or server-side GPG public key enrollment (these are assumed prerequisites on the Terralist instance).
- Modifying provider resource/data-source implementations or Caddy API client logic.

## Anti-Goals

- Breaking local builds, unit tests, or existing acceptance test fixtures.
- Exposing private keys, passphrases, or Terralist API tokens in code or workflow logs.
- Overwriting existing release artifacts on Gitea or Terralist.

## Constraints

- **Repository Visibility:** Gitea repository `sebclem/terraform-provider-caddy` on `git.sebclem.fr` is public, allowing Terralist to download release assets over HTTPS without needing Gitea download credentials.
- **Terralist API Authentication:** Terralist expects an API key with provider publish permissions passed in headers: `Authorization: Bearer x-api-key:<API_KEY>` or `X-API-Key: <API_KEY>`.
- **GPG Verification:** Terralist requires that `SHA256SUMS` and its detached signature (`.sig`) verify against a public key configured in the `sebclem` authority on Terralist.
- **Provider Protocol:** Provider supports Terraform protocol version 6.0 (declared in `terraform-registry-manifest.json`).

## Architecture and Workflow

```
[ Git Tag Push (v*)] 
        │
        ▼
[ Gitea Actions (.gitea/workflows/release.yml) ]
        │
        ├── 1. Checkout & Setup Go
        ├── 2. Import GPG Private Key (from secrets)
        ├── 3. GoReleaser (target Gitea)
        │       ├── Build cross-platform binaries with ldflags
        │       ├── Generate SHA256SUMS & GPG signature (.sig)
        │       └── Create Release & upload assets to git.sebclem.fr
        │
        └── 4. Register with Terralist
                ├── Construct JSON payload mapping release asset URLs
                └── POST /v1/api/providers/sebclem/caddy/:version/upload
                        │
                        ▼
            [ Terralist (terralist.sebclem.fr) ]
                ├── Download assets from public Gitea release
                ├── Verify GPG signature against authority key
                └── Publish provider version in registry
```

## Detailed Component Specifications

### 1. Provider Build & Address Configuration
- **File:** `main.go`
  - Update default `address` fallback from `local.sebclem.fr/sebclem/caddy` to `terralist.sebclem.fr/sebclem/caddy`.
- **File:** `.goreleaser.yaml`
  - Update `ldflags`: `-s -w -X main.version={{ .Version }} -X main.address=terralist.sebclem.fr/sebclem/caddy`.
  - Update `release` section to target Gitea:
    ```yaml
    release:
      gitea:
        host: git.sebclem.fr
        owner: sebclem
        name: terraform-provider-caddy
      draft: false
      prerelease: auto
      extra_files:
        - glob: terraform-registry-manifest.json
          name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'
    ```

### 2. Gitea Actions Release Workflow
- **File:** `.gitea/workflows/release.yml`
  - **Triggers:** Push to tags matching `v*`.
  - **Concurrency:** Ensure no concurrent runs cancel in-flight releases.
  - **Steps:**
    1. Checkout repository with full depth (`fetch-depth: 0`).
    2. Setup Go environment.
    3. Import GPG signing key (`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`).
    4. Run GoReleaser (`release --clean`) using `GITEA_TOKEN: ${{ secrets.GITHUB_TOKEN }}` (Gitea Actions provides this automatically) or a dedicated secret.
    5. Construct Terralist upload JSON payload:
       - Extracts version without `v` prefix.
       - Formulates asset download URLs using public Gitea format:
         `https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v${VERSION}/...`
       - Maps each platform (os/arch) to its binary archive filename, download URL, and SHA256 checksum parsed from `dist/terraform-provider-caddy_${VERSION}_SHA256SUMS`.
       - Specifies `shasums.url` and `shasums.signature_url`.
       - Sets `protocols: ["6.0"]`.
    6. Send HTTP POST to `https://terralist.sebclem.fr/v1/api/providers/sebclem/caddy/${VERSION}/upload` with `X-API-Key: ${{ secrets.TERRALIST_API_KEY }}`.
    7. Verify HTTP 200/201 response.
- **Cleanup:** Remove `.github/workflows/release.yml`.

### 3. Documentation
- **File:** `docs/releasing.md`
  - Update release process documentation:
    - Overview of native Gitea Actions + Terralist workflow.
    - Required Gitea repository secrets:
      - `GPG_PRIVATE_KEY`: ASCII-armored private key matching the Terralist authority public key.
      - `GPG_PASSPHRASE`: Passphrase for the private key.
      - `TERRALIST_API_KEY`: Terralist API key with write permissions to the `sebclem` authority.
    - Release execution: Tagging `vX.Y.Z` and pushing to `git.sebclem.fr`.
    - Verification: Using the provider with `terralist.sebclem.fr/sebclem/caddy`.

## What's Left to the Builder

- The exact scripting language/utility used in the Gitea Actions workflow to generate and submit the Terralist payload (e.g. bash + jq or a small Python helper script).
- Testing and verifying local GoReleaser dry-run (`goreleaser release --snapshot --skip=publish --clean`) to ensure artifact naming and checksum generation succeed cleanly.
