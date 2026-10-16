# caddy_tls_certificate

Required `certificate` and `key` paths, optional `tags` list. These refer to existing PEM files on the Caddy host; the provider does not read them from the runner. Each resource owns one entry in `apps/tls/certificates/load_files`. Supports optional `object_id`, generated UUID, in-place path updates and import only by an existing loader entry's `@id`.

Use `depends_on = [caddy_tls_certificate.files]` on HTTPS servers. Caddy selects loaded certificates by SNI. The active inventory's two certificate groups have distinct hostnames, so the default connection policy suffices; no separate connection-policy resource is needed for these sites.

Changing paths reprovisions the loader. Updating file contents at unchanged paths creates no Terraform/OpenTofu diff: the operator must renew files and reload Caddy to refresh them. File provisioning, permissions and renewal are external. Missing/invalid files produce Caddy diagnostics; inspect Caddy logs for the detailed cause. No ACME issuance is enabled.
