package main

import (
	"context"
	"git.sebclem.fr/sebclem/terraform-provider-caddy/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"log"
)

var version = "0.1.0"

func main() {
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{Address: "local.sebclem.fr/sebclem/caddy"}); err != nil {
		log.Fatal(err)
	}
}
