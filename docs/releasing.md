# Provider releases

The Gitea repository remains the source repository. The release workflow is intended to run from the future public GitHub mirror, `sebclem/opentofu-provider-caddy`. It builds platform archives, the OpenTofu protocol manifest, SHA256 checksums and a detached GPG signature, then publishes those files as a GitHub Release. The workflow deliberately skips runs from any other GitHub repository.

## One-time setup

1. Create and publish the GitHub mirror `sebclem/opentofu-provider-caddy`, including `.github/workflows/release.yml`, `.goreleaser.yaml` and `terraform-registry-manifest.json`. Ensure version tags are mirrored to GitHub; the Gitea workflow runner does not execute GitHub Actions.
2. Generate a long-lived GPG signing key and submit its public key to the OpenTofu Registry using the provider registry's GPG-key submission process. The registry requires the signing key to be registered before it will accept signed versions. Keep the private key and passphrase secure.
3. Add GitHub Actions repository secrets `GPG_PRIVATE_KEY` (ASCII-armored private key) and `GPG_PASSPHRASE` (its passphrase). The built-in `GITHUB_TOKEN` needs repository contents write permission, which the workflow grants for release uploads.
4. Submit the provider to the OpenTofu Registry and verify that the provider's public GPG key and GitHub release repository are associated with namespace `sebclem` and provider type `caddy`.
5. Push a new semantic version tag such as `v0.2.0` to the GitHub mirror. The action builds and publishes a signed GitHub release. Once the provider is registered, the OpenTofu Registry ingests releases from its GitHub release webhook. Check the registry listing and run `tofu init` with `source = "sebclem/caddy"` after the version appears.

Never replace the assets of an already published version. Publish fixes under a new version tag.

## Repository naming note

The planned mirror name `opentofu-provider-caddy` is configured as the release destination. The current OpenTofu Registry submission automation's procedure says provider repositories are expected to use `NAMESPACE/terraform-provider-NAME`; therefore, the planned repository name may need to be renamed to `terraform-provider-caddy` before registry submission. The release files themselves use the required `terraform-provider-caddy` binary and archive names either way. See the [registry submission procedure](https://github.com/opentofu/registry/blob/main/PROCEDURES.md#provider-submissions).

After publication, use the registry source address `registry.opentofu.org/sebclem/caddy` (or the shorthand `sebclem/caddy`) with the release version constraint. The release workflow and GoReleaser do not create the OpenTofu Registry entry or submit the GPG key. Registry ingestion is available after those one-time steps are complete. See the [OpenTofu provider signing requirements](https://opentofu.org/docs/cli/plugins/signing/) and [provider registry protocol](https://opentofu.org/docs/internals/provider-registry-protocol/).
