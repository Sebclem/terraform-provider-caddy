terraform {
  required_providers {
    caddy = {
      source  = "local.sebclem.fr/sebclem/caddy"
      version = "0.1.0"
    }
  }
}

variable "caddy_endpoint" {
  type    = string
  default = "http://127.0.0.1:2019"
}

variable "listen" {
  type    = string
  default = "127.0.0.1:8080"
}

variable "upstream" {
  type    = string
  default = "127.0.0.1:9000"
}

provider "caddy" {
  endpoint = var.caddy_endpoint
}

resource "caddy_http_server" "main" {
  name   = "main"
  listen = [var.listen]
}

resource "caddy_http_route" "app" {
  server_id = caddy_http_server.main.id
  terminal  = true
  match     = [{ hosts = ["app.example.test"] }]
}

resource "caddy_handler_reverse_proxy" "app" {
  route_id = caddy_http_route.app.id
  upstream {
    dial = var.upstream
  }
}
