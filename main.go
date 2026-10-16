package main

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"log"
)

var version = "0.1.0"
var address = "local.sebclem.fr/sebclem/caddy"

func main() {
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{Address: address}); err != nil {
		log.Fatal(err)
	}
}
