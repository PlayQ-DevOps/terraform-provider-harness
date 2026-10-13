package artifactRepositories

import (
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
)

// Definitions are the artifact repository connectors served on the framework. The SDK v2
// declarations stay as the schema source and as the legacy resources the migration tests
// apply state with.
func Definitions() []connectors.Definition {
	return []connectors.Definition{
		{Suffix: "artifactory", Type: nextgen.ConnectorTypes.Artifactory, Resource: ResourceConnectorArtifactory, DataSource: DatasourceConnectorArtifactory, Build: buildConnectorArtifactory, Read: readConnectorArtifactory},
		{Suffix: "azure_artifacts", Type: nextgen.ConnectorTypes.AzureArtifacts, Resource: ResourceConnectorAzureArtifacts, Build: buildConnectorAzureArtifacts, Read: readConnectorAzureArtifacts},
		{Suffix: "docker", Type: nextgen.ConnectorTypes.DockerRegistry, Resource: ResourceConnectorDocker, DataSource: DatasourceConnectorDocker, Build: buildConnectorDocker, Read: readConnectorDocker},
		{Suffix: "helm", Type: nextgen.ConnectorTypes.HttpHelmRepo, Resource: ResourceConnectorHelm, DataSource: DatasourceConnectorHelm, Build: buildConnectorHelm, Read: readConnectorHelm},
		{Suffix: "jenkins", Type: nextgen.ConnectorTypes.Jenkins, Resource: ResourceConnectorJenkins, DataSource: DataSourceConnectorJenkins, Build: buildConnectorJenkins, Read: readConnectorJenkins},
		{Suffix: "nexus", Type: nextgen.ConnectorTypes.Nexus, Resource: ResourceConnectorNexus, DataSource: DatasourceConnectorNexus, Build: buildConnectorNexus, Read: readConnectorNexus},
		{Suffix: "oci_helm", Type: nextgen.ConnectorTypes.OciHelmRepo, Resource: ResourceConnectorOciHelm, DataSource: DatasourceConnectorOciHelm, Build: buildConnectorOciHelm, Read: readConnectorOciHelm},
	}
}
