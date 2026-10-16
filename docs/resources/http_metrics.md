# caddy_http_metrics

Required `per_host` boolean. Owns the singleton `apps/http/metrics` node while preserving unrelated HTTP app fields. State ID is `http-metrics`; there is no Caddy `@id`, no `object_id` and no importer. Exactly one state resource may own this setting; creation rejects an existing node. Updates preserve other metrics settings. Deletion removes the metrics subtree. Caddy exposes observations through the externally managed admin metrics endpoint.
