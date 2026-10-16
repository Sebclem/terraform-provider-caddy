# caddy_log

Required `name`; optional typed `writer = {filename = "...", mode = "0640", roll_size_mb = 48, roll_local_time = true}`, `include` and `exclude` lists. Named destinations use file output. `name` changes replace the resource; writer/filter edits are in place. Supports `object_id`, UUID default and existing-ID import. Do not create two resources with the same name.

For a site logger named `app`, set `include = ["http.log.access.app"]`. A single explicit log named `default` owns the shared `exclude` list; exclude each named access logger there to prevent default-log duplication. An omitted default writer preserves Caddy's normal default destination. Bind hosts separately with `caddy_log_binding`.

The pinned adapter converts legacy `roll_size 50mb` to `roll_size_mb = 48`. Caddy may retain open writers when configuration changes refer to the same filename; changing rotation/mode options for the same file can require an operator restart. The provider reports the configured values and does not restart services. See [Caddy's file output documentation](https://caddyserver.com/docs/caddyfile/directives/log#file).
