package secretManagers

import (
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Definitions are the secret manager connectors served on the framework. The SDK v2
// declarations stay as the schema source and as the legacy resources the migration tests
// apply state with.
func Definitions() []connectors.Definition {
	return []connectors.Definition{
		{Suffix: "awskms", Type: nextgen.ConnectorTypes.AwsKms, Resource: ResourceConnectorAwsKms, DataSource: DatasourceConnectorAwsKms, Build: buildConnectorAwsKms, Read: readConnectorAwsKms},
		{Suffix: "aws_secret_manager", Type: nextgen.ConnectorTypes.AwsSecretManager, Resource: ResourceConnectorAwsSM, DataSource: DatasourceConnectorAwsSM, Build: buildConnectorAwsSM, Read: readConnectorAwsSM},
		{Suffix: "azure_key_vault", Type: nextgen.ConnectorTypes.AzureKeyVault, Resource: ResourceConnectorAzureKeyVault, DataSource: DataSourceConnectorAzureKeyVault, Build: buildConnectorAzureKeyVault, Read: readConnectorAzureKeyVault,
			Validate: func(d *schema.ResourceData) error { return checkAzureKeyVaultAuth(d) }},
		{Suffix: "custom_secret_manager", Type: nextgen.ConnectorTypes.CustomSecretManager, Resource: ResourceConnectorCSM, DataSource: DatasourceConnectorCustomSM, Build: buildConnectorCustomSM, Read: readConnectorCustomSM},
		{Suffix: "gcp_kms", Type: nextgen.ConnectorTypes.GcpKms, Resource: ResourceConnectorGcpKms, DataSource: DatasourceConnectorGcpKms, Build: buildConnectorGcpKms, Read: readConnectorGcpKms},
		{Suffix: "gcp_secret_manager", Type: nextgen.ConnectorTypes.GcpSecretManager, Resource: ResourceConnectorGCPSecretManager, DataSource: DatasourceConnectorGcpSM, Build: buildConnectorGcpSM, Read: readConnectorGcpSM},
		{Suffix: "vault", Type: nextgen.ConnectorTypes.Vault, Resource: ResourceConnectorVault, DataSource: DataSourceConnectorVault, Build: buildConnectorVault, Read: readConnectorVault},
	}
}
