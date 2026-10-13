package contract

// refreshConditionalBaseline records, per Read function, how many d.Set calls are still
// skipped when the API value is empty (see TestReadsRefreshFromTheAPI). Each one hides a
// value that was removed in Harness. Entries may only shrink: fix the sets and lower the
// count. Regenerate with:
//
//	CONTRACT_PRINT_BASELINE=1 go test ./internal/contract -run TestReadsRefreshFromTheAPI
var refreshConditionalBaseline = map[string]int{
	// infra_ref mirrors infra_id only when the API names an infrastructure; the two attributes conflict.
	"internal/service/chaos/experiment/resource_experiment.go:resourceExperimentRead": 2,
	// resource_identifier is server-assigned and absent from some responses.
	"internal/service/platform/cdb/dashboards/resource_dashboards.go:readDashboard": 1,
}
