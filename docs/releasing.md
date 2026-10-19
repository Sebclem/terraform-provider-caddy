# Provider releases

Provider releases for `terraform-provider-caddy` are built, signed, and published natively via Gitea Actions on `git.sebclem.fr` and registered with the Terralist registry at `terralist.sebclem.fr`.

Whenever a semantic version tag (e.g. `v0.2.0`) is pushed to the repository, Gitea Actions executes `.gitea/workflows/release.yml`:
1. Checks out the repository and imports the release GPG signing key.
2. Runs GoReleaser to cross-compile platform binaries stamped with `main.address=terralist.sebclem.fr/sebclem/caddy`.
3. Generates the `SHA256SUMS` file and detached GPG signature (`.sig`).
4. Creates a public Gitea Release on `git.sebclem.fr/sebclem/terraform-provider-caddy` with all package assets.
5. Dispatches an upload registration request to `https://terralist.sebclem.fr/v1/api/providers/sebclem/caddy/:version/upload` via `scripts/publish-terralist.py`. Terralist downloads the release assets directly from the public Gitea release and verifies the signature against the `sebclem` authority public key.

## One-time setup

1. **Terralist Authority & Signing Key:**
   - Ensure an authority named `sebclem` exists in your Terralist instance (`terralist.sebclem.fr`).
   - Configure your GPG public key in the authority settings so Terralist can verify uploaded release signatures.
2. **Terralist API Key:**
   - In Terralist Settings (`API Keys`), create an API key with write permissions for the `sebclem` authority.
3. **Gitea Repository Secrets:**
   Configure the following secrets under **Repository Settings > Actions > Secrets** on `git.sebclem.fr`:
   - `GPG_PRIVATE_KEY`: ASCII-armored GPG private key matching the authority public key.
   - `GPG_PASSPHRASE`: Passphrase for the GPG private key.
   - `TERRALIST_API_KEY`: The Terralist API key token.

## Publishing a release

1. Ensure all tests and checks pass:
   ```bash
   make check
   ```
2. Create and push a semantic version tag:
   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```
3. Monitor the Gitea Actions workflow under **Actions** on `git.sebclem.fr/sebclem/terraform-provider-caddy`.
4. Verify the published assets on Gitea Releases (`https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/tag/v0.2.0`).
5. Verify the provider version appears in the Terralist registry (`https://terralist.sebclem.fr`).

> [!WARNING]
> Never replace the assets of an already published release. Publish fixes under a new incremented semantic version tag.

## Consuming the provider

To use the published provider in Terraform or OpenTofu configurations, point the provider source to the Terralist registry:

```hcl
terraform {
  required_providers {
    caddy = {
      source  = "terralist.sebclem.fr/sebclem/caddy"
      version = "~> 0.2.0"
    }
  }
}

provider "caddy" {
  # ... configuration ...
}
```
