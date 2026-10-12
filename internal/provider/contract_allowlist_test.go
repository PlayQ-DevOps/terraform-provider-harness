package provider_test

// contractAllowlist records deliberate, reasoned exceptions to the provider contract
// (see contract_test.go), keyed by check name and then by resource type or
// "type.attribute.path". Every entry needs a reason. An entry that no longer violates
// its check fails the test until it is removed, so this list stays honest.
//
// Pre-existing violations that are debt rather than decisions live in the generated
// contract_baseline_test.go instead.
var contractAllowlist = map[string]map[string]string{
	checkImporterMissing: {
		"harness_time_sleep":                "no remote object exists; there is nothing to import",
		"harness_platform_trigger_pipeline": "one-shot action resource that runs a pipeline execution; nothing to import",
		"harness_chaos_hub_sync":            "one-shot action resource that triggers a hub sync; nothing to import",
	},
}
