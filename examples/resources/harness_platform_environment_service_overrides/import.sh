# Import an account level service override using the environment and the service it applies to
terraform import harness_platform_environment_service_overrides.example <env_id>/<service_id>

# Import an org level service override
terraform import harness_platform_environment_service_overrides.example <org_id>/<env_id>/<service_id>

# Import a project level service override
terraform import harness_platform_environment_service_overrides.example <org_id>/<project_id>/<env_id>/<service_id>
