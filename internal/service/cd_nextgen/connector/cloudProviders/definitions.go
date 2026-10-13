package cloudProviders

import (
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
)

// Definitions are the cloud provider connectors served on the framework. The SDK v2
// declarations stay as the schema source and as the legacy resources the migration tests
// apply state with.
func Definitions() []connectors.Definition {
	return []connectors.Definition{
		{Suffix: "aws", Type: nextgen.ConnectorTypes.Aws, Resource: ResourceConnectorAws, DataSource: DatasourceConnectorAws, Build: buildConnectorAws, Read: readConnectorAws},
		{Suffix: "azure_cloud_provider", Type: nextgen.ConnectorTypes.Azure, Resource: ResourceConnectorAzureCloudProvider, DataSource: DataSourceConnectorAzureCloudProvider, Build: buildConnectorAzureCloudProvider, Read: readConnectorAzureCloudProvider},
		{Suffix: "gcp", Type: nextgen.ConnectorTypes.Gcp, Resource: ResourceConnectorGcp, DataSource: DatasourceConnectorGcp, Build: buildConnectorGcp, Read: readConnectorGcp},
		{Suffix: "kubernetes", Type: nextgen.ConnectorTypes.K8sCluster, Resource: ResourceConnectorK8s, DataSource: DatasourceConnectorKubernetes, Build: buildConnectorK8s, Read: readConnectorK8s},
		{Suffix: "rancher", Type: nextgen.ConnectorTypes.Rancher, Resource: ResourceConnectorK8sRancher, DataSource: DatasourceConnectorRancher, Build: buildConnectorRancher, Read: readConnectorRancher},
		{Suffix: "pdc", Type: nextgen.ConnectorTypes.Pdc, Resource: ResourceConnectorPdc, DataSource: DatasourceConnectorPdc, Build: buildConnectorPdc, Read: readConnectorPdc},
		{Suffix: "spot", Type: nextgen.ConnectorTypes.Spot, Resource: ResourceConnectorSpot, DataSource: DatasourceConnectorSpot, Build: buildConnectorSpot, Read: readConnectorSpot},
		{Suffix: "tas", Type: nextgen.ConnectorTypes.Tas, Resource: ResourceConnectorTas, DataSource: DataSourceConnectorTas, Build: buildConnectorTas, Read: readConnectorTas},
		{Suffix: "terraform_cloud", Type: nextgen.ConnectorTypes.TerraformCloud, Resource: ResourceConnectorTerraformCloud, DataSource: DatasourceConnectorTerraformCloud, Build: buildConnectorTerraformCloud, Read: readConnectorTerraformCloud},
	}
}
