# Import an account level chaos infrastructure using the environment it is registered in
terraform import harness_chaos_infrastructure.example <environment_id>/<infra_id>

# Import an org level chaos infrastructure
terraform import harness_chaos_infrastructure.example <org_id>/<environment_id>/<infra_id>

# Import a project level chaos infrastructure
terraform import harness_chaos_infrastructure.example <org_id>/<project_id>/<environment_id>/<infra_id>
