# Testing

The initial compatibility matrix uses Go 1.27.2, Caddy 2.11.7, OpenTofu 1.12.6 and Terraform 1.15.9 on Linux amd64. Dependency pins are in go.mod/go.sum and tool pins in tools/versions.env. Compatibility claims apply to the executed matrix, not to every release of these products.

`make check` runs unit tests, vet and build. Set `CADDY_BIN` to include real Caddy process tests. With no tool environment variables, process tests explicitly skip; this is insufficient for release validation.

Run the release acceptance suite separately with each CLI:

```sh
CADDY_BIN=/absolute/path/to/caddy IAC_BIN=/absolute/path/to/tofu make test-acceptance
CADDY_BIN=/absolute/path/to/caddy IAC_BIN=/absolute/path/to/terraform make test-acceptance
```

The target builds the provider, creates a temporary filesystem mirror and CLI configuration, and uses loopback Caddy instances and synthetic upstreams. Tests cover init/validate, ordinary and refresh-only apply, plan JSON actions, stable UUIDs, explicit-ID replacement, read-only imports, optional-field ownership/removal, route/handler movement and drift, subtree deletion, rejected writes, endpoint errors and dropped responses after committed creates. Review regressions additionally cover explicit-ID parent replacements, missing anchors, cancellation after commitment and duplicate desired upstream addresses. A second apply must make no API mutations.

The HTTP/API suite additionally covers HTTP/verified HTTPS, custom trust, mutual TLS, context deadlines, ETag collisions, ID resolution, preservation of large JSON numbers and a live ID/reorder race. Caddy checksums use SHA512; OpenTofu uses SHA256. See tools/README.md before obtaining tools.

Use `go test -race ./...` with the same environment to check concurrent test/client behavior. Fixtures start processes only in tests; provider lifecycle code never launches a command or manages a service. Test runs must never receive a production endpoint.
