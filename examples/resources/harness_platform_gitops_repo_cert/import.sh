# Import the repository certificates of an account level agent
terraform import harness_platform_gitops_repo_cert.example <agent_id>

# Import the repository certificates of an org level agent
terraform import harness_platform_gitops_repo_cert.example <org_id>/<agent_id>

# Import the repository certificates of a project level agent
terraform import harness_platform_gitops_repo_cert.example <org_id>/<project_id>/<agent_id>
