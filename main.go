package main

import (
	"context"
	"log"

	"github.com/combor/terraform-provider-ona/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	if err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/combor/ona",
	}); err != nil {
		log.Fatal(err)
	}
}
