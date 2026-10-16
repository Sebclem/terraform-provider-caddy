# Test tools

Versions are pinned in versions.env. Use official Go downloads, Caddy GitHub release archives, and OpenTofu GitHub releases. Verify Go against go.dev's published SHA256, Caddy against its release checksums (SHA512), and OpenTofu against its SHA256SUMS before extraction. Keep local binaries in .tools/ or /tmp and do not overwrite system tools.

Set CADDY_BIN to the absolute Caddy executable path. Process fixtures run only on loopback and use temporary configuration/data directories. They never connect to the configured production endpoint. Set IAC_BIN to the OpenTofu or Terraform executable for each acceptance run.
