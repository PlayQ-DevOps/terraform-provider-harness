package provider

import (
	"github.com/harness/terraform-provider-harness/internal/service/cd_nextgen/connector/cloudProviders"
	"github.com/harness/terraform-provider-harness/internal/service/cd_nextgen/connector/codeRepositories"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
)

// ConnectorDefinitions lists every connector served on the framework. A connector package
// moves here as a whole (its Definitions function) and leaves the SDK v2 maps in
// provider.go at the same time.
func ConnectorDefinitions() []connectors.Definition {
	var defs []connectors.Definition
	defs = append(defs, codeRepositories.Definitions()...)
	defs = append(defs, cloudProviders.Definitions()...)
	return defs
}
