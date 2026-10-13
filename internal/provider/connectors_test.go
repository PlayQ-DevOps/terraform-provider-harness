package provider_test

import (
	"context"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/provider"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

// Every connector definition converts to a framework schema the framework accepts.
func TestConnectorDefinitionsConvert(t *testing.T) {
	ctx := context.Background()
	for _, def := range provider.ConnectorDefinitions() {
		if err := connectors.Check(def); err != nil {
			t.Errorf("%s: %v", def.TypeName(), err)
			continue
		}
		for _, newResource := range connectors.Resources([]connectors.Definition{def}) {
			sr := &fwresource.SchemaResponse{}
			newResource().Schema(ctx, fwresource.SchemaRequest{}, sr)
			if sr.Diagnostics.HasError() {
				t.Errorf("%s: schema: %v", def.TypeName(), sr.Diagnostics)
				continue
			}
			if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Errorf("%s: the framework rejects the converted schema: %v", def.TypeName(), diags)
			}
		}
	}
}
