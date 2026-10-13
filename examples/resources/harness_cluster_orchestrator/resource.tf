data "aws_region" "current" {}

resource "harness_cluster_orchestrator" "test" {
  name             = "name"
  cluster_endpoint = "http://test.test.com"
  k8s_connector_id = "test"
  region           = data.aws_region.current.name
}
