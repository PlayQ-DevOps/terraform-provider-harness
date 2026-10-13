package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
)

// NewMuxServer returns the provider server that serves both halves of the provider over
// protocol 5: the SDK v2 provider (the existing resources and data sources) and the
// plugin-framework provider (new and migrated resources, ephemeral resources). The two
// halves must agree on the provider schema; a mismatch surfaces as a GetProviderSchema
// diagnostic, which framework_provider_test.go checks.
func NewMuxServer(ctx context.Context, version string) (func() tfprotov5.ProviderServer, error) {
	mux, err := tf5muxserver.NewMuxServer(ctx,
		Provider(version)().GRPCProvider,
		providerserver.NewProtocol5(NewFrameworkProvider(version)()),
	)
	if err != nil {
		return nil, err
	}
	return mux.ProviderServer, nil
}
