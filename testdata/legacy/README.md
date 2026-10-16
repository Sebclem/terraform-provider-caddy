# Active legacy fixture

Captured from the public YAML inputs, inventory variables and Caddyfile template of `ansible-caddy-playbook` on 2026-10-16 using Caddy 2.11.7. No vault file is read. All four password hashes are synthetic (`fixture-password`). Hostname spelling is preserved, including dotted site filenames. The 49 active sites exclude absent aliases.

`scripts/capture-legacy.py ANSIBLE_REPO LOADER_SOURCE` executes the installed action plugin's actual `_load_files_in_dir` method in an isolated namespace with a read-only YAML loader, then renders the real template. PyYAML and Jinja2 are fixture-generation dependencies only. The loader SHA256 and fully resolved sites are recorded in `manifest.yaml`. Additional hostnames inherit access flags and upstream address, but not health checks, custom site fragments, proxy options or parent certificate-directory overrides. Explicit alias certificate paths are retained.

The checked-in adaptation preserves array order and all settings. Tests normalize only the source Caddyfile's file-server hide path, which depends on checkout location. `50mb` becomes `roll_size_mb: 48` in the pinned adapter; `35MiB` becomes 36700160 bytes. These are adapter results, not estimated conversions. These fixtures are never loaded into production.
