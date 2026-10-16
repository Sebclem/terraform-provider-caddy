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

## Active configuration matrix

The active delivery adds tests for nested route/handler composition, typed matchers and trusted/untrusted forwarded clients (including IPv6 client addresses), forward-auth preserving original method/body and replacing copied headers, bcrypt access, real active health probes, selective health/TLS updates and preserved transport siblings. TLS tests use synthetic files, verify loaded certificates and redirects, import and path updates, and externally bootstrap unprivileged test ports. No real certificate files or private backends are used.

Log/error tests verify destination/mode, read-only top-ID import, stable error-page IDs, 403/404/500 files and missing-file behavior against the actual adapted legacy handler. Metrics tests verify real per-host observations and per_host-only writes. The home rebuild rehearsal exercises the checked-in typed example for all 49 active sites and 55 request probes, checks SNI groups and access-log record counts, exports/restores an operator backup, and requires a no-change second plan/apply with zero API writes. Synthetic replacements apply only in the test fixture.

Fixtures in `testdata/legacy/` record the actual loader's resolved public settings and pinned Caddy adaptation. `TestLegacyFixture` repeats adaptation without requiring Python/Ansible or touching production; optional fixture regeneration uses `scripts/capture-legacy.py` and the installed loader source. The provider has no YAML/Ansible runtime dependency.

Post-review verification on 2026-10-16 passed the full `go test ./... -count=1` matrix with Caddy 2.11.7 and each CLI: OpenTofu 1.12.6 (acceptance package 124.467s) and Terraform 1.15.9 (126.997s). This includes 29 top-level acceptance/fixture tests plus subcases, the 49-site/55-probe rebuild and lifecycle regressions. Extended same-path certificate-renewal/missing-file checks also passed separately on both CLIs after addition. `go test -race ./internal/caddy ./internal/provider ./internal/testutil`, `go vet ./...`, build, Go/HCL formatting and local documentation-link/fence checks passed. No production cutover was executed.
