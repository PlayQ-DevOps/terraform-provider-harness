package provider_test

// contractAllowlist records deliberate, reasoned exceptions to the provider contract
// (see contract_test.go), keyed by check name and then by resource type or
// "type.attribute.path". Every entry needs a reason. An entry that no longer violates
// its check fails the test until it is removed, so this list stays honest.
//
// Pre-existing violations that are debt rather than decisions live in the generated
// contract_baseline_test.go instead.
var contractAllowlist = map[string]map[string]string{
	checkMutableIdentity: {
		"harness_platform_environment_clusters_mapping.identifier": "a label with no meaning to the API; the mapping is identified by its environment, which is ForceNew",
	},
	checkImporterMissing: {
		"harness_time_sleep":                "no remote object exists; there is nothing to import",
		"harness_platform_trigger_pipeline": "one-shot action resource that runs a pipeline execution; nothing to import",
		"harness_chaos_hub_sync":            "one-shot action resource that triggers a hub sync; nothing to import",
	},
	checkSensitive: {
		"harness_chaos_probe_template.apm_probe.prometheus_inputs.tls_config.ca_cert_secret":     "a Harness secret identifier, not the certificate",
		"harness_chaos_probe_template.apm_probe.prometheus_inputs.tls_config.client_cert_secret": "a Harness secret identifier, not the certificate",
		"harness_chaos_probe_template.apm_probe.prometheus_inputs.tls_config.client_key_secret":  "a Harness secret identifier, not the key",
		"harness_platform_connector_aws.manual.access_key":                                       "an AWS access key ID, which identifies the key; the secret half is only ever a reference (secret_key_ref)",
		"harness_platform_connector_azure_key_vault.secret_key":                                  "a reference to the Harness text secret holding the Azure key, per its description",
		"harness_platform_connector_tas.credentials.tas_manual_details.reference_token":          "a secret reference: it maps to the API field refreshTokenRef",
		"harness_platform_connector_vault.auth_token":                                            "a reference to the Harness secret holding the Vault token (account.<secret>), per the examples",
	},
}
