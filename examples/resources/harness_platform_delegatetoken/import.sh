# Import an account level delegate token by name
terraform import harness_platform_delegatetoken.example <token_name>

# Import an org level delegate token by name
terraform import harness_platform_delegatetoken.example <org_id>/<token_name>

# Import a project level delegate token by name
terraform import harness_platform_delegatetoken.example <org_id>/<project_id>/<token_name>
