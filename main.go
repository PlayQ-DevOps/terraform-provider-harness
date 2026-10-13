package main

import (
	"context"
	"flag"
	"log"

	"github.com/harness/terraform-provider-harness/internal/provider"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
)

// Run "go generate" to format example terraform files and generate the docs for the registry/website

// If you do not have terraform installed, you can remove the formatting command, but its suggested to
// ensure the documentation is formatted properly.
//go:generate terraform fmt -recursive ./examples/

// Run the docs generation tool, check its repository for more information on how it works and how docs
// can be customized.
//go:generate go tool tfplugindocs

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = "dev"

	// goreleaser can also pass the specific commit if you want
	// commit  string = ""
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	ctx := context.Background()

	// The SDK v2 provider and the plugin-framework provider are served as one provider.
	muxServer, err := provider.NewMuxServer(ctx, version)
	if err != nil {
		log.Fatal(err.Error())
	}

	var serveOpts []tf5server.ServeOpt
	if debugMode {
		serveOpts = append(serveOpts, tf5server.WithManagedDebug())
	}

	if err := tf5server.Serve("registry.terraform.io/harness/harness", muxServer, serveOpts...); err != nil {
		log.Fatal(err.Error())
	}
}
