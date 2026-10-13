# Import the cluster mapping of an account level environment
terraform import harness_platform_environment_clusters_mapping.example <env_id>

# Import the cluster mapping of an org level environment
terraform import harness_platform_environment_clusters_mapping.example <org_id>/<env_id>

# Import the cluster mapping of a project level environment
terraform import harness_platform_environment_clusters_mapping.example <org_id>/<project_id>/<env_id>
