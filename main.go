package main

import (
	"context"
	"log"

	"github.com/hachisieunhan/terraform-provider-googledrivesuite/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/hachisieunhan/googledrivesuite",
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
