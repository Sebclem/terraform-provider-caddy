# Terralist CI/CD Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement native Gitea Actions CI/CD to build, sign, create a Gitea release, and publish provider versions to the private Terralist registry at `terralist.sebclem.fr`.

**Architecture:** Update provider compilation targets and GoReleaser configuration to target `terralist.sebclem.fr/sebclem/caddy` and Gitea releases on `git.sebclem.fr`. Create a tested Python publish helper that formats the Terralist payload from release checksums and posts it to Terralist's API. Wire up the pipeline in `.gitea/workflows/release.yml` and update documentation in `docs/releasing.md`.

**Tech Stack:** Go 1.22+, GoReleaser, Gitea Actions, Python 3 (standard library), Terralist REST API.

**Spec:** `docs/superpowers/specs/2026-10-19-terralist-cicd-publishing-design.md`

## Global Constraints

- Target registry address is `terralist.sebclem.fr/sebclem/caddy`.
- Gitea repository host is `git.sebclem.fr`, owner `sebclem`, repository `terraform-provider-caddy`.
- Terralist API endpoint is `POST https://terralist.sebclem.fr/v1/api/providers/sebclem/caddy/:version/upload`.
- Release asset URLs follow `https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v${VERSION}/${FILENAME}`.
- Terralist authentication uses header `Authorization: Bearer x-api-key:<API_KEY>` or `X-API-Key: <API_KEY>`.
- Provider protocol version is `6.0`.

## Review Focus

1. Normalization of version tag: A tag provided as `v0.2.0` or `0.2.0` must consistently format the release URL tag with `v` (e.g. `v0.2.0`) while formatting the Terralist API URL and payload version without `v` (e.g. `0.2.0`).
2. Missing or mismatched `.sig` file: If the GPG signature file is missing from the checksums directory, the publish helper must fail fast with a descriptive error before calling the Terralist API.
3. Terralist API non-200/201 response: The publish helper must raise an exception detailing the HTTP status code and response body so Gitea Actions fails visibly.
4. Filtering non-platform files in SHA256SUMS: The checksums file may contain entries such as `terraform-registry-manifest.json` or `manifest.json`. The helper must filter only platform ZIP archives (`.zip`) into the `platforms` array.
5. Missing required environment variables or secrets: The helper must validate that `TERRALIST_API_KEY` is present and fail cleanly if absent.

---

### Task 1: Update Provider Address and GoReleaser Configuration

**Files:**
- Modify: `main.go:10-12`
- Modify: `.goreleaser.yaml:14,61-65`
- Test: `main_test.go`

**Interfaces:**
- Produces: Provider package with `address = "terralist.sebclem.fr/sebclem/caddy"` and `.goreleaser.yaml` targeting Gitea release host `git.sebclem.fr`.

- [ ] **Step 1: Write the failing test in `main_test.go`**

```go
package main

import (
	"testing"
)

func TestProviderDefaultAddress(t *testing.T) {
	expected := "terralist.sebclem.fr/sebclem/caddy"
	if address != expected {
		t.Errorf("expected address %q, got %q", expected, address)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestProviderDefaultAddress -v .`
Expected: FAIL with `expected address "terralist.sebclem.fr/sebclem/caddy", got "local.sebclem.fr/sebclem/caddy"`

- [ ] **Step 3: Update `main.go` and `.goreleaser.yaml`**

In `main.go`:
```go
var version = "0.1.0"
var address = "terralist.sebclem.fr/sebclem/caddy"
```

In `.goreleaser.yaml`:
Update `ldflags`:
```yaml
    ldflags:
      - -s -w -X main.version={{ .Version }} -X main.address=terralist.sebclem.fr/sebclem/caddy
```
Update `release` section:
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

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestProviderDefaultAddress -v . && go test ./... && go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go .goreleaser.yaml
git commit -m "build: update provider registry address to terralist.sebclem.fr and target gitea release"
```

---

### Task 2: Implement and Test Terralist Publish Helper Script

**Files:**
- Create: `scripts/test_publish_terralist.py`
- Create: `scripts/publish-terralist.py`

**Interfaces:**
- Consumes: `dist/terraform-provider-caddy_${VERSION}_SHA256SUMS`, `dist/terraform-provider-caddy_${VERSION}_SHA256SUMS.sig`, and `TERRALIST_API_KEY`.
- Produces: CLI script `scripts/publish-terralist.py` that formats Terralist payload and submits it via HTTP POST.

- [ ] **Step 1: Write the failing tests in `scripts/test_publish_terralist.py`**

```python
import os
import unittest
from unittest.mock import patch, MagicMock
import sys

# Import the module under test
import importlib.util
spec = importlib.util.spec_from_file_location("publish_terralist", os.path.join(os.path.dirname(__file__), "publish-terralist.py"))
publish_terralist = importlib.util.module_from_spec(spec)
# Will load in step 3 once file exists

class TestPublishTerralist(unittest.TestCase):
    def setUp(self):
        self.sample_sums = (
            "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2  terraform-provider-caddy_0.2.0_darwin_amd64.zip\n"
            "b5d4045c3f466fa91fe2cc6abe79232a1a57cdf104f7cf4e730109d43433eabb  terraform-provider-caddy_0.2.0_linux_amd64.zip\n"
            "a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f708192a3b4c5d6e7f809  terraform-provider-caddy_0.2.0_manifest.json\n"
        )

    def test_parse_platforms(self):
        from publish_terralist import parse_platforms
        platforms = parse_platforms(self.sample_sums, "0.2.0", "git.sebclem.fr", "sebclem", "terraform-provider-caddy")
        self.assertEqual(len(platforms), 2)
        self.assertEqual(platforms[0]["os"], "darwin")
        self.assertEqual(platforms[0]["arch"], "amd64")
        self.assertEqual(platforms[0]["shasum"], "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2")
        self.assertEqual(platforms[0]["filename"], "terraform-provider-caddy_0.2.0_darwin_amd64.zip")
        self.assertEqual(platforms[0]["download_url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_darwin_amd64.zip")

    def test_build_payload(self):
        from publish_terralist import build_payload
        payload = build_payload(self.sample_sums, "0.2.0", "git.sebclem.fr", "sebclem", "terraform-provider-caddy")
        self.assertEqual(payload["protocols"], ["6.0"])
        self.assertEqual(payload["shasums"]["url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_SHA256SUMS")
        self.assertEqual(payload["shasums"]["signature_url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_SHA256SUMS.sig")
        self.assertEqual(len(payload["platforms"]), 2)

    def test_normalize_version(self):
        from publish_terralist import normalize_version
        self.assertEqual(normalize_version("v0.2.0"), "0.2.0")
        self.assertEqual(normalize_version("0.2.0"), "0.2.0")

    @patch("urllib.request.urlopen")
    def test_publish_success(self, mock_urlopen):
        from publish_terralist import publish_to_terralist
        mock_response = MagicMock()
        mock_response.getcode.return_value = 200
        mock_response.read.return_value = b'{"success": true}'
        mock_urlopen.return_value.__enter__.return_value = mock_response

        payload = {"protocols": ["6.0"]}
        publish_to_terralist("https://terralist.sebclem.fr", "secret-key", "sebclem", "caddy", "0.2.0", payload)
        self.assertTrue(mock_urlopen.called)

    @patch("urllib.request.urlopen")
    def test_publish_failure(self, mock_urlopen):
        from publish_terralist import publish_to_terralist
        import urllib.error
        mock_urlopen.side_effect = urllib.error.HTTPError("https://...", 400, "Bad Request", {}, None)

        payload = {"protocols": ["6.0"]}
        with self.assertRaises(RuntimeError):
            publish_to_terralist("https://terralist.sebclem.fr", "secret-key", "sebclem", "caddy", "0.2.0", payload)

if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python3 -m unittest scripts/test_publish_terralist.py`
Expected: FAIL (file or functions not found)

- [ ] **Step 3: Implement `scripts/publish-terralist.py`**

Create `scripts/publish-terralist.py`:
- Functions:
  - `normalize_version(version: str) -> str`: Strips leading `v` if present.
  - `parse_platforms(checksum_text: str, version: str, host: str, owner: str, repo: str) -> list[dict]`:
    - Regex parses lines matching `^([a-f0-9]{64})\s+terraform-provider-([^_]+)_([^_]+)_([^_]+)_([^.]+)\.zip$`.
    - Skips non-zip files (like manifests).
    - Constructs `download_url`: `https://{host}/{owner}/{repo}/releases/download/v{version}/{filename}`.
    - Returns array of `{ os, arch, download_url, filename, shasum }`.
  - `build_payload(checksum_text: str, version: str, host: str, owner: str, repo: str) -> dict`:
    - Returns `{ "protocols": ["6.0"], "shasums": { "url": "...", "signature_url": "..." }, "platforms": [...] }`.
  - `publish_to_terralist(terralist_url: str, api_key: str, namespace: str, provider_name: str, version: str, payload: dict) -> None`:
    - Constructs URL: `{terralist_url}/v1/api/providers/{namespace}/{provider_name}/{version}/upload`.
    - Sends HTTP POST with JSON data and header `X-API-Key: {api_key}` (and `Authorization: Bearer x-api-key:{api_key}`).
    - Validates response code, prints status, and raises `RuntimeError` on failure.
  - `main()`:
    - Parses CLI args / env vars (`VERSION`, `TERRALIST_API_KEY`, `TERRALIST_URL`, `DIST_DIR`, `GITEA_HOST`, `REPO_OWNER`, `REPO_NAME`).
    - Verifies presence of `SHA256SUMS` and `SHA256SUMS.sig` in `DIST_DIR`.
    - Builds payload and invokes `publish_to_terralist`.

- [ ] **Step 4: Run test to verify it passes**

Run: `python3 -m unittest scripts/test_publish_terralist.py`
Expected: PASS (all tests pass)

- [ ] **Step 5: Commit**

```bash
git add scripts/publish-terralist.py scripts/test_publish_terralist.py
git commit -m "feat(ci): add Terralist publishing helper script with automated tests"
```

---

### Task 3: Create Gitea Actions Workflow and Retire GitHub Actions

**Files:**
- Create: `.gitea/workflows/release.yml`
- Delete: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: GoReleaser configuration (`.goreleaser.yaml`), GPG secrets, `TERRALIST_API_KEY`, and `scripts/publish-terralist.py`.
- Produces: Complete Gitea Actions release pipeline triggered on `v*` tags.

- [ ] **Step 1: Create `.gitea/workflows/release.yml`**

Create `.gitea/workflows/release.yml` with:
- Trigger:
  ```yaml
  name: Release provider

  on:
    push:
      tags:
        - 'v*'

  concurrency:
    group: provider-release-${{ github.ref }}
    cancel-in-progress: false

  jobs:
    release:
      runs-on: ubuntu-latest
      steps:
        - name: Check out full history
          uses: actions/checkout@v4
          with:
            fetch-depth: 0

        - name: Set up Go
          uses: actions/setup-go@v5
          with:
            go-version-file: go.mod
            cache: true

        - name: Set up Python
          uses: actions/setup-python@v5
          with:
            python-version: '3.x'

        - name: Import release signing key
          uses: crazy-max/ghaction-import-gpg@v6
          id: import-gpg
          with:
            gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}
            passphrase: ${{ secrets.GPG_PASSPHRASE }}

        - name: Run GoReleaser
          uses: goreleaser/goreleaser-action@v6
          with:
            version: v2.18.3
            args: release --clean
          env:
            GITEA_TOKEN: ${{ secrets.GITHUB_TOKEN }}
            GPG_FINGERPRINT: ${{ steps.import-gpg.outputs.fingerprint }}

        - name: Publish to Terralist
          env:
            TERRALIST_API_KEY: ${{ secrets.TERRALIST_API_KEY }}
            TERRALIST_URL: https://terralist.sebclem.fr
            VERSION: ${{ github.ref_name }}
          run: |
            python3 scripts/publish-terralist.py \
              --version "${VERSION}" \
              --terralist-url "${TERRALIST_URL}" \
              --api-key "${TERRALIST_API_KEY}" \
              --dist-dir "dist" \
              --host "git.sebclem.fr" \
              --owner "sebclem" \
              --repo "terraform-provider-caddy"
  ```

- [ ] **Step 2: Remove `.github/workflows/release.yml`**

Run: `git rm .github/workflows/release.yml`

- [ ] **Step 3: Verify tests and YAML syntax**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.gitea/workflows/release.yml'))"
go test ./...
python3 -m unittest discover -s scripts
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add .gitea/workflows/release.yml
git commit -m "ci: replace GitHub Actions release workflow with Gitea Actions and Terralist publishing"
```

---

### Task 4: Update Documentation and Release Guides

**Files:**
- Modify: `docs/releasing.md`
- Modify: `README.md:13-14`

**Interfaces:**
- Produces: Updated release guide describing Gitea Actions and Terralist usage.

- [ ] **Step 1: Rewrite `docs/releasing.md`**

Update `docs/releasing.md` with:
- Overview of native Gitea Actions workflow in `.gitea/workflows/release.yml`.
- Details of Gitea repository secrets configuration:
  - `GPG_PRIVATE_KEY`: ASCII-armored private key matching the Terralist authority public key.
  - `GPG_PASSPHRASE`: Passphrase for the signing key.
  - `TERRALIST_API_KEY`: API key for `terralist.sebclem.fr` with write permissions to authority `sebclem`.
- Steps to publish:
  - Creating and pushing git tags: `git tag v0.2.0 && git push origin v0.2.0`.
  - Verifying the release on `git.sebclem.fr/sebclem/terraform-provider-caddy/releases`.
  - Verifying publication on `terralist.sebclem.fr/sebclem/caddy`.
- Usage instructions in Terraform/OpenTofu:
  ```hcl
  terraform {
    required_providers {
      caddy = {
        source  = "terralist.sebclem.fr/sebclem/caddy"
        version = "~> 0.2.0"
      }
    }
  }
  ```

- [ ] **Step 2: Update `README.md`**

Update the installation note in `README.md` from `local.sebclem.fr/sebclem/caddy` to refer to the official registry address `terralist.sebclem.fr/sebclem/caddy` published through Terralist.

- [ ] **Step 3: Verify docs format and links**

Run: `git diff docs/releasing.md README.md`
Expected: Clear diff reflecting Terralist and Gitea Actions.

- [ ] **Step 4: Commit**

```bash
git add docs/releasing.md README.md
git commit -m "docs: document Gitea Actions release workflow and Terralist registry usage"
```
