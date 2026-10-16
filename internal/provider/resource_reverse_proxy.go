package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewReverseProxyResource() resource.Resource { return &caddyResource{kind: "proxy"} }
