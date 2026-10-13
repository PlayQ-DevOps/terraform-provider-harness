package provider

import (
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
	"github.com/harness/terraform-provider-harness/internal/service/platform/organization"
	"github.com/harness/terraform-provider-harness/internal/service/platform/project"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// LegacySDKProvider is the SDK v2 provider with the SDK v2 implementations of the types
// that have been migrated to the framework added back (organization, project and every
// connector). acctest.RunMigration applies state with it and then hands that state to the
// muxed provider, proving the migration leaves an empty plan. It is test scaffolding and is
// never served to users; the legacy CRUD code it needs stays for that reason only.
func LegacySDKProvider(version string) *schema.Provider {
	p := Provider(version)()
	p.ResourcesMap["harness_platform_organization"] = organization.LegacyResourceOrganization()
	p.ResourcesMap["harness_platform_project"] = project.LegacyResourceProject()
	p.DataSourcesMap["harness_platform_organization"] = organization.LegacyDataSourceOrganization()
	p.DataSourcesMap["harness_platform_project"] = project.LegacyDataSourceProject()
	p.DataSourcesMap["harness_platform_project_list"] = project.LegacyDataSourceProjectList()
	for name, r := range connectors.LegacyResources(ConnectorDefinitions()) {
		p.ResourcesMap[name] = r
	}
	for name, ds := range connectors.LegacyDataSources(ConnectorDefinitions()) {
		p.DataSourcesMap[name] = ds
	}
	return p
}
