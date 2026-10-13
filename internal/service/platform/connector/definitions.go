package connector

import (
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/service/platform/connectors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Definitions are the platform connectors served on the framework. The SDK v2
// declarations stay as the schema source and as the legacy resources the migration tests
// apply state with. harness_platform_gcp_projects is not a connector lookup and stays an
// SDK v2 data source.
func Definitions() []connectors.Definition {
	return []connectors.Definition{
		{Suffix: "anthropic_model", Type: nextgen.ConnectorTypes.AnthropicModel, Resource: ResourceConnectorAnthropicModel, DataSource: DatasourceConnectorAnthropicModel, Build: buildConnectorAnthropicModel, Read: readConnectorAnthropicModel,
			Validate: func(d *schema.ResourceData) error { return checkAnthropicModelAuthConfig(d) }},
		{Suffix: "appdynamics", Type: nextgen.ConnectorTypes.AppDynamics, Resource: ResourceConnectorAppDynamics, DataSource: DatasourceConnectorAppDynamics, Build: buildConnectorAppDynamics, Read: readConnectorAppDynamics},
		{Suffix: "awscc", Type: nextgen.ConnectorTypes.CEAws, Resource: ResourceConnectorAwsCC, DataSource: DatasourceConnectorAwsCC, Build: buildConnectorAwsCC, Read: readConnectorAwsCC},
		{Suffix: "azure_cloud_cost", Type: nextgen.ConnectorTypes.CEAzure, Resource: ResourceConnectorAzureCloudCost, DataSource: DataSourceConnectorAzureCloudCost, Build: buildConnectorAzureCloudCost, Read: readConnectorAzureCloudCost},
		{Suffix: "customhealthsource", Type: nextgen.ConnectorTypes.CustomHealth, Resource: ResourceConnectorCustomHealthSource, DataSource: DatasourceConnectorCustomHealthSource, Build: buildConnectorCustomHealthSource, Read: readConnectorCustomHealthSource},
		{Suffix: "datadog", Type: nextgen.ConnectorTypes.Datadog, Resource: ResourceConnectorDatadog, DataSource: DatasourceConnectorDatadog, Build: buildConnectorDatadog, Read: readConnectorDatadog},
		{Suffix: "dynatrace", Type: nextgen.ConnectorTypes.Dynatrace, Resource: ResourceConnectorDynatrace, DataSource: DatasourceConnectorDynatrace, Build: buildConnectorDynatrace, Read: readConnectorDynatrace},
		{Suffix: "elasticsearch", Type: nextgen.ConnectorTypes.ElasticSearch, Resource: ResourceConnectorElasticSearch, DataSource: DatasourceConnectorElasticSearch, Build: buildConnectorElasticSearch, Read: readConnectorElasticSearch},
		{Suffix: "gcp_cloud_cost", Type: nextgen.ConnectorTypes.GcpCloudCost, Resource: ResourceConnectorGCPCloudCost, DataSource: DataSourceConnectorGCPCloudCost, Build: buildConnectorGcpCloudCost, Read: readConnectorGCPCloudCost},
		{Suffix: "jdbc", Type: nextgen.ConnectorTypes.JDBC, Resource: ResourceConnectorJDBC, DataSource: DatasourceConnectorJDBC, BuildErr: buildConnectorJDBC, Read: readConnectorJDBC},
		{Suffix: "jira", Type: nextgen.ConnectorTypes.Jira, Resource: ResourceConnectorJira, DataSource: DatasourceConnectorJira, Build: buildConnectorJira, Read: readConnectorJira},
		{Suffix: "kubernetes_cloud_cost", Type: nextgen.ConnectorTypes.CEK8sCluster, Resource: ResourceConnectorKubernetesCloudCost, DataSource: DatasourceConnectorKubernetesCloudCost, Build: buildConnectorKubernetesCloudCost, Read: readConnectorKubernetesCloudCost},
		{Suffix: "newrelic", Type: nextgen.ConnectorTypes.NewRelic, Resource: ResourceConnectorNewRelic, Build: buildConnectorNewRelic, Read: readConnectorNewRelic},
		{Suffix: "openai_model", Type: nextgen.ConnectorTypes.OpenAIModel, Resource: ResourceConnectorOpenAIModel, DataSource: DatasourceConnectorOpenAIModel, Build: buildConnectorOpenAIModel, Read: readConnectorOpenAIModel,
			Validate: func(d *schema.ResourceData) error { return checkOpenAIModelAuthConfig(d) }},
		{Suffix: "pagerduty", Type: nextgen.ConnectorTypes.PagerDuty, Resource: ResourceConnectorPagerDuty, DataSource: DatasourceConnectorPagerDuty, Build: buildConnectorPagerDuty, Read: readConnectorPagerDuty},
		{Suffix: "prometheus", Type: nextgen.ConnectorTypes.Prometheus, Resource: ResourceConnectorPrometheus, DataSource: DatasourceConnectorPrometheus, Build: buildConnectorPrometheus, Read: readConnectorPrometheus},
		{Suffix: "service_now", Type: nextgen.ConnectorTypes.ServiceNow, Resource: ResourceConnectorServiceNow, DataSource: DataSourceConnectorSerivceNow, Build: buildConnectorServiceNow, Read: readConnectorServiceNow},
		{Suffix: "splunk", Type: nextgen.ConnectorTypes.Splunk, Resource: ResourceConnectorSplunk, DataSource: DatasourceConnectorSplunk, Build: buildConnectorSplunk, Read: readConnectorSplunk},
		{Suffix: "sumologic", Type: nextgen.ConnectorTypes.SumoLogic, Resource: ResourceConnectorSumologic, DataSource: DatasourceConnectorSumologic, Build: buildConnectorSumologic, Read: readConnectorSumologic},
	}
}
