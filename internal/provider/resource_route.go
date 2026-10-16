package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewHTTPRouteResource() resource.Resource { return &caddyResource{kind: "route"} }
