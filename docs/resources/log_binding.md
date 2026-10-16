# caddy_log_binding

Required `server_id`, `hostname` and `log_id`. Each resource owns one `logs.logger_names` hostname entry; updates preserve sibling entries. The target server and named log must have IDs. Server/hostname changes replace the binding; changing the log updates it in place.

`id` is `binding-` plus SHA256 of `server_id + NUL + hostname`. This is a provider state identity, never a fabricated Caddy `@id`. No `object_id`, ordering or importer exists. Exactly one state resource may own a binding; creation rejects an existing entry, including an identical one. Deleting a server removes its bindings with its subtree.

Computed `target_satisfied` reports whether the live binding resolves to an identified log. A missing named log marks drift while retaining dependency identity, allowing a managed log to be recreated and its binding repaired in the same apply.
