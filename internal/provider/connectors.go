package provider

import (
	"github.com/harness/terraform-provider-harness/internal/service/cd_nextgen/connector/artifactRepositories"
	"github.com/harness/terraform-provider-harness/internal/service/cd_nextgen/connector/cloudProviders"
	"github.com/harness/terraform-provider-harness/internal/service/cd_nextgen/connector/codeRepositories"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connector"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connector/secretManagers"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
)

// ConnectorDefinitions lists every connector served on the framework. A connector package
// moves here as a whole (its Definitions function) and leaves the SDK v2 maps in
// provider.go at the same time.
func ConnectorDefinitions() []connectors.Definition {
	var defs []connectors.Definition
	defs = append(defs, codeRepositories.Definitions()...)
	defs = append(defs, cloudProviders.Definitions()...)
	defs = append(defs, artifactRepositories.Definitions()...)
	defs = append(defs, secretManagers.Definitions()...)
	defs = append(defs, connector.Definitions()...)
	return defs
}
