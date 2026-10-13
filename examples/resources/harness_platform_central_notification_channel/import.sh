# Import an account level notification channel
terraform import harness_platform_central_notification_channel.example <channel_id>

# Import an org level notification channel
terraform import harness_platform_central_notification_channel.example <org_id>/<channel_id>

# Import a project level notification channel
terraform import harness_platform_central_notification_channel.example <org_id>/<project_id>/<channel_id>
