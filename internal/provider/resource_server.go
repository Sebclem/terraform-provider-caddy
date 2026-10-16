package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewHTTPServerResource() resource.Resource { return &caddyResource{kind: "server"} }
