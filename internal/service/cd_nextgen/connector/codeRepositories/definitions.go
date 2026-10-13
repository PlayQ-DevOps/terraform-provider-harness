package codeRepositories

import (
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Definitions are the code repository connectors served on the framework. The SDK v2
// declarations (ResourceConnectorX, DatasourceConnectorX) stay as the schema source and
// as the legacy resources the migration tests apply state with.
func Definitions() []connectors.Definition {
	return []connectors.Definition{
		{Suffix: "github", Type: nextgen.ConnectorTypes.Github, Resource: ResourceConnectorGithub, DataSource: DatasourceConnectorGithub, Build: buildConnectorGithub, Read: readConnectorGithub},
		{Suffix: "gitlab", Type: nextgen.ConnectorTypes.Gitlab, Resource: ResourceConnectorGitlab, DataSource: DatasourceConnectorGitlab, Build: buildConnectorGitlab, Read: readConnectorGitlab},
		{Suffix: "bitbucket", Type: nextgen.ConnectorTypes.Bitbucket, Resource: ResourceConnectorBitbucket, DataSource: DatasourceConnectorBitbucket, Build: buildConnectorBitbucket, Read: readConnectorBitbucket, Validate: func(d *schema.ResourceData) error { return checkBitbucketApiAuthentication(d) }},
		{Suffix: "azure_repo", Type: nextgen.ConnectorTypes.AzureRepo, Resource: ResourceConnectorAzureRepo, DataSource: DatasourceConnectorAzureRepo, Build: buildConnectorAzureRepo, Read: readConnectorAzureRepo},
		{Suffix: "git", Type: nextgen.ConnectorTypes.Git, Resource: ResourceConnectorGit, DataSource: DatasourceConnectorGit, Build: buildConnectorGit, Read: readConnectorGit},
	}
}
