# Cluster without Optional tags
resource "harness_platform_gitops_cluster" "example" {
  identifier = "identifier"
  account_id = "account_id"
  project_id = "project_id"
  org_id     = "org_id"
  agent_id   = "agent_id"

  request {
    upsert = false
    cluster {
      server = "https://kubernetes.default.svc"
      name   = "name"
      config {
        tls_client_config {
          insecure = true
        }
        cluster_connection_type = "IN_CLUSTER"
      }

    }
  }
  lifecycle {
    ignore_changes = [
      request.0.upsert, request.0.cluster.0.config.0.bearer_token,
    ]
  }
}

# Cluster with Optional tags. Cluster at org scope with an account level agent.
resource "harness_platform_gitops_cluster" "example" {
  identifier = "identifier"
  account_id = "account_id"
  org_id     = "org_id"
  agent_id   = "account.agent_id"

  request {
    upsert = false
    tags = [
      "foo:bar",
    ]
    cluster {
      server = "https://kubernetes.default.svc"
      name   = "name"
      config {
        tls_client_config {
          insecure = true
        }
        cluster_connection_type = "IN_CLUSTER"
      }

    }
  }
  lifecycle {
    ignore_changes = [
      request.0.upsert, request.0.cluster.0.config.0.bearer_token,
    ]
  }
}


# Cluster with secret_expressions (agent resolves secrets at connect time)
resource "harness_platform_gitops_cluster" "example" {
  identifier = "identifier"
  account_id = "account_id"
  agent_id   = "account.agent_id"

  request {
    upsert = true
    secret_expressions = {
      bearerToken = "account.my_k8s_token_secret"
    }
    cluster {
      server = "https://my-cluster.example.com"
      name   = "name"
      config {
        cluster_connection_type = "SERVICE_ACCOUNT"
        tls_client_config {
          insecure = true
        }
      }
    }
  }
  lifecycle {
    ignore_changes = [
      request.0.upsert, request.0.cluster.0.config.0.bearer_token,
    ]
  }
}

# Cluster with self signed certificate
resource "harness_platform_gitops_cluster" "example" {
  identifier = "identifier"
  account_id = "account_id"
  project_id = "project_id"
  org_id     = "org_id"
  agent_id   = "agent_id"

  request {
    upsert = false
    cluster {
      server = "https://1.2.3.4"
      name   = "name"
      config {
        tls_client_config {
          ca_data = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tClhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWApYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFgKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ=="
        }
        bearer_token            = "ey......X"
        cluster_connection_type = "SERVICE_ACCOUNT"
      }

    }
  }
  lifecycle {
    ignore_changes = [
      request.0.upsert, request.0.cluster.0.config.0.bearer_token,
    ]
  }
}
