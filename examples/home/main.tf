terraform {
  required_providers {
    caddy = {
      source  = "local.sebclem.fr/sebclem/caddy"
      version = "0.1.0"
    }
  }
}
provider "caddy" { endpoint = var.caddy_endpoint }
variable "caddy_endpoint" { type = string }
variable "listen" {
  type    = list(string)
  default = [":443"]
}
variable "certificate_files" {
  type = map(object({ certificate = string, key = string }))
  default = {
    sebclem     = { certificate = "/etc/ssl/acme/-.sebclem.fr/fullchain.pem", key = "/etc/ssl/acme/-.sebclem.fr/key.pem" }
    claptrapbot = { certificate = "/etc/ssl/acme/-.claptrapbot.com/fullchain.pem", key = "/etc/ssl/acme/-.claptrapbot.com/key.pem" }
  }
}
variable "error_root" {
  type    = string
  default = "/srv"
}
variable "log_directory" {
  type    = string
  default = "/var/log/caddy"
}
variable "basic_auth_accounts" {
  type      = list(object({ username = string, password_hash = string }))
  sensitive = true
}
resource "caddy_tls_certificate" "files" {
  for_each    = var.certificate_files
  certificate = each.value.certificate
  key         = each.value.key
}
resource "caddy_http_metrics" "main" { per_host = true }
resource "caddy_http_server" "main" {
  name            = "home"
  listen          = var.listen
  trusted_proxies = ["10.0.0.0/8"]
  https           = true
  depends_on      = [caddy_tls_certificate.files]
}
resource "caddy_log" "site" {
  for_each = local.sites
  name     = each.key
  writer = {
    filename        = "${var.log_directory}/${each.key}.log"
    mode            = "0640"
    roll_size_mb    = 48
    roll_local_time = true
  }
  include = ["http.log.access.${each.key}"]
}
resource "caddy_log" "default" {
  name    = "default"
  exclude = [for hostname in sort(keys(local.sites)) : "http.log.access.${hostname}"]
}
resource "caddy_log_binding" "site" {
  for_each  = local.sites
  server_id = caddy_http_server.main.id
  hostname  = each.key
  log_id    = caddy_log.site[each.key].id
}
resource "caddy_http_route" "site" {
  for_each  = local.sites
  server_id = caddy_http_server.main.id
  match     = [{ hosts = [each.key] }]
  terminal  = true
}
resource "caddy_handler_subroute" "site" {
  for_each = local.sites
  route_id = caddy_http_route.site[each.key].id
}
resource "caddy_error_pages" "site" {
  for_each    = local.sites
  subroute_id = caddy_handler_subroute.site[each.key].id
  root        = var.error_root
  page_uri    = "/{http.error.status_code}.html"
}
resource "caddy_http_route" "auth" {
  for_each    = { for k, v in local.sites : k => v if v.protected }
  subroute_id = caddy_handler_subroute.site[each.key].id
}
resource "caddy_handler_forward_auth" "site" {
  for_each     = { for k, v in local.sites : k => v if v.protected }
  route_id     = caddy_http_route.auth[each.key].id
  upstream     = "auth.home:9091"
  uri          = "/api/authz/forward-auth"
  copy_headers = ["Remote-User", "Remote-Groups", "Remote-Email", "Remote-Name"]
}
resource "caddy_http_route" "denied" {
  for_each    = { for k, v in local.sites : k => v if v.whitelist_local || v.webdav }
  subroute_id = caddy_handler_subroute.site[each.key].id
  after_id    = try(caddy_http_route.auth[each.key].id, null)
  match = [{
    methods = each.value.webdav ? ["POST", "PUT", "DELETE"] : null
    not     = [{ client_ip_ranges = local.local_ips }]
  }]
}
resource "caddy_handler_error" "denied" {
  for_each    = { for k, v in local.sites : k => v if v.whitelist_local || v.webdav }
  route_id    = caddy_http_route.denied[each.key].id
  status_code = 403
  message     = "Unauthorized"
}
// The adapted metrics subroute checks access and then continues to the
// application proxy for allowed requests. Keep this route nonterminal.
resource "caddy_http_route" "metrics" {
  for_each    = { for k, v in local.sites : k => v if v.jellyfin }
  subroute_id = caddy_handler_subroute.site[each.key].id
  match       = [{ paths = ["/metrics"] }]
}
resource "caddy_handler_subroute" "metrics" {
  for_each = { for k, v in local.sites : k => v if v.jellyfin }
  route_id = caddy_http_route.metrics[each.key].id
}
resource "caddy_http_route" "metrics_denied" {
  for_each    = { for k, v in local.sites : k => v if v.jellyfin }
  subroute_id = caddy_handler_subroute.metrics[each.key].id
  match       = [{ not = [{ client_ip_ranges = ["10.0.0.0/8"] }] }]
}
resource "caddy_handler_error" "metrics_denied" {
  for_each    = { for k, v in local.sites : k => v if v.jellyfin }
  route_id    = caddy_http_route.metrics_denied[each.key].id
  status_code = 403
  message     = "Unauthorized"
}
resource "caddy_http_route" "app" {
  for_each    = local.sites
  subroute_id = caddy_handler_subroute.site[each.key].id
  after_id = try(
    caddy_http_route.metrics[each.key].id,
    caddy_http_route.denied[each.key].id,
    caddy_http_route.auth[each.key].id,
    null
  )
}
resource "caddy_handler_basic_auth" "site" {
  for_each = { for k, v in local.sites : k => v if v.basic_auth }
  route_id = caddy_http_route.app[each.key].id
  accounts = var.basic_auth_accounts
}
resource "caddy_handler_reverse_proxy" "site" {
  for_each = local.sites
  route_id = caddy_http_route.app[each.key].id
  after_id = try(caddy_handler_basic_auth.site[each.key].id, null)
  upstream { dial = each.value.upstream }
  transport        = each.value.tls ? { tls = true, insecure_skip_verify = true } : null
  request_headers  = each.value.override_host ? { set = { Host = ["{http.request.hostport}"] } } : null
  request_buffers  = each.value.buffered ? 36700160 : null
  response_buffers = each.value.buffered ? 36700160 : null
  flush_interval   = each.value.buffered ? "-1ns" : null
  health = each.value.health_uri != "" ? {
    uri              = each.value.health_uri
    follow_redirects = true
    fails            = 3
    timeout          = "20s"
  } : null
}
