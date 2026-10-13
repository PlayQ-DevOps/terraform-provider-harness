package monitored_service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/harness/harness-go-sdk/harness/nextgen"
)

// The block builders turn the health_sources, change_sources and dependencies blocks into
// their SDK models. Each block carries a type and a JSON spec whose shape depends on the
// type; a type the provider does not know, or a spec that does not decode, is an error for
// the apply, never a panic that takes the plugin down.

// safeUnmarshal decodes JSON into v. The SDK UnmarshalJSON methods panic on a
// discriminator they do not know (a metric threshold or health source type), so the panic
// is turned into an error here. An empty document decodes as an empty object.
func safeUnmarshal(data []byte, v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	if strings.TrimSpace(string(data)) == "" {
		data = []byte("{}")
	}
	return json.Unmarshal(data, v)
}

// decoded decodes a raw JSON spec into a new T.
func decoded[T any](raw string) (*T, error) {
	v := new(T)
	if err := safeUnmarshal([]byte(raw), v); err != nil {
		return nil, err
	}
	return v, nil
}

func asBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// supportedTypes lists the keys of a builder table for an error message.
func supportedTypes[V any](m map[string]V) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// jsonToMap decodes a JSON object; an empty spec is an empty object.
func jsonToMap(jsonStr string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if err := safeUnmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// A healthSourceBuilder fills the typed spec of out from the raw spec JSON and its decoded
// form.
type healthSourceBuilder func(out *nextgen.HealthSource, raw string, spec map[string]interface{}) error

// typedSpec adapts a builder over the decoded spec; set stores its result in the
// HealthSource.
func typedSpec[T any](build func(map[string]interface{}) (T, error), set func(*nextgen.HealthSource, *T)) healthSourceBuilder {
	return func(out *nextgen.HealthSource, _ string, spec map[string]interface{}) error {
		v, err := build(spec)
		if err != nil {
			return err
		}
		set(out, &v)
		return nil
	}
}

// rawSpec adapts a plain JSON decode of the spec into the SDK model.
func rawSpec[T any](set func(*nextgen.HealthSource, *T)) healthSourceBuilder {
	return func(out *nextgen.HealthSource, raw string, _ map[string]interface{}) error {
		v, err := decoded[T](raw)
		if err != nil {
			return err
		}
		set(out, v)
		return nil
	}
}

var healthSourceBuilders = map[string]healthSourceBuilder{
	"AppDynamics":           typedSpec(getAppDynamicsHealthSource, func(o *nextgen.HealthSource, v *nextgen.AppDynamicsHealthSource) { o.AppDynamics = v }),
	"NewRelic":              typedSpec(getNewRelicHealthSource, func(o *nextgen.HealthSource, v *nextgen.NewRelicHealthSource) { o.NewRelic = v }),
	"StackdriverLog":        rawSpec(func(o *nextgen.HealthSource, v *nextgen.StackdriverLogHealthSource) { o.StackdriverLog = v }),
	"Splunk":                rawSpec(func(o *nextgen.HealthSource, v *nextgen.SplunkHealthSource) { o.Splunk = v }),
	"Prometheus":            typedSpec(getPrometheusHealthSource, func(o *nextgen.HealthSource, v *nextgen.PrometheusHealthSource) { o.Prometheus = v }),
	"Stackdriver":           typedSpec(getStackDriverHealthSource, func(o *nextgen.HealthSource, v *nextgen.StackdriverMetricHealthSource) { o.Stackdriver = v }),
	"DatadogMetrics":        typedSpec(getDataDogHealthSource, func(o *nextgen.HealthSource, v *nextgen.DatadogMetricHealthSource) { o.DatadogMetrics = v }),
	"DatadogLog":            rawSpec(func(o *nextgen.HealthSource, v *nextgen.DatadogLogHealthSource) { o.DatadogLog = v }),
	"Dynatrace":             typedSpec(getDynatraceHealthSource, func(o *nextgen.HealthSource, v *nextgen.DynatraceHealthSource) { o.Dynatrace = v }),
	"ErrorTracking":         rawSpec(func(o *nextgen.HealthSource, v *nextgen.ErrorTrackingHealthSource) { o.ErrorTracking = v }),
	"CustomHealthMetric":    typedSpec(getCustomHealthSource, func(o *nextgen.HealthSource, v *nextgen.CustomHealthSourceMetric) { o.CustomHealthMetric = v }),
	"CustomHealthLog":       rawSpec(func(o *nextgen.HealthSource, v *nextgen.CustomHealthSourceLog) { o.CustomHealthLog = v }),
	"SplunkMetric":          typedSpec(getSplunkHealthSource, func(o *nextgen.HealthSource, v *nextgen.SplunkMetricHealthSource) { o.SplunkMetric = v }),
	"ElasticSearch":         rawSpec(func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.ElasticSearch = v }),
	"CloudWatchMetrics":     typedSpec(getCloudWatchHealthSource, func(o *nextgen.HealthSource, v *nextgen.CloudWatchMetricsHealthSource) { o.CloudWatchMetrics = v }),
	"AwsPrometheus":         typedSpec(getAwsPrometheusHealthSource, func(o *nextgen.HealthSource, v *nextgen.AwsPrometheusHealthSource) { o.AwsPrometheus = v }),
	"SumologicMetrics":      typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.SumologicMetrics = v }),
	"SumologicLogs":         typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.SumologicLogs = v }),
	"SplunkSignalFXMetrics": typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.SplunkSignalFXMetrics = v }),
	"GrafanaLokiLogs":       typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.GrafanaLokiLogs = v }),
	"AzureLogs":             typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.AzureLogs = v }),
	"AzureMetrics":          typedSpec(getNextGenHealthSource, func(o *nextgen.HealthSource, v *nextgen.NextGenHealthSource) { o.AzureMetrics = v }),
}

func getHealthSourceByType(hs map[string]interface{}) (nextgen.HealthSource, error) {
	healthSourceType := asString(hs["type"])
	out := nextgen.HealthSource{
		Name:       asString(hs["name"]),
		Identifier: asString(hs["identifier"]),
		Version:    asString(hs["version"]),
		Type_:      nextgen.HealthSourceType(healthSourceType),
	}
	build, ok := healthSourceBuilders[healthSourceType]
	if !ok {
		return out, fmt.Errorf("health source %q: unsupported type %q (one of %s)", out.Identifier, healthSourceType, supportedTypes(healthSourceBuilders))
	}
	raw := asString(hs["spec"])
	spec, err := jsonToMap(raw)
	if err != nil {
		return out, fmt.Errorf("health source %q: spec is not valid JSON: %w", out.Identifier, err)
	}
	if err := build(&out, raw, spec); err != nil {
		return out, fmt.Errorf("health source %q: %w", out.Identifier, err)
	}
	return out, nil
}

type changeSourceBuilder func(out *nextgen.ChangeSourceDto, raw string) error

func changeSpec[T any](set func(*nextgen.ChangeSourceDto, *T)) changeSourceBuilder {
	return func(out *nextgen.ChangeSourceDto, raw string) error {
		v, err := decoded[T](raw)
		if err != nil {
			return err
		}
		set(out, v)
		return nil
	}
}

var changeSourceBuilders = map[string]changeSourceBuilder{
	"HarnessCDNextGen": changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.HarnessCdChangeSourceSpec) { o.HarnessCDNextGen = v }),
	"PagerDuty":        changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.PagerDutyChangeSourceSpec) { o.PagerDuty = v }),
	"K8sCluster":       changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.KubernetesChangeSourceSpec) { o.K8sCluster = v }),
	"HarnessCD":        changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.HarnessCdCurrentGenChangeSourceSpec) { o.HarnessCD = v }),
	"CustomDeploy":     changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.CustomDeployChangeSourceSpec) { o.CustomDeploy = v }),
	"CustomIncident":   changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.CustomIncidentChangeSourceSpec) { o.CustomIncident = v }),
	"CustomFF":         changeSpec(func(o *nextgen.ChangeSourceDto, v *nextgen.CustomFFChangeSourceSpec) { o.CustomFF = v }),
}

func getChangeSourceByType(cs map[string]interface{}) (nextgen.ChangeSourceDto, error) {
	changeSourceType := asString(cs["type"])
	out := nextgen.ChangeSourceDto{
		Name:       asString(cs["name"]),
		Identifier: asString(cs["identifier"]),
		Type_:      nextgen.ChangeSourceType(changeSourceType),
		Enabled:    asBool(cs["enabled"]),
		Category:   asString(cs["category"]),
	}
	build, ok := changeSourceBuilders[changeSourceType]
	if !ok {
		return out, fmt.Errorf("change source %q: unsupported type %q (one of %s)", out.Identifier, changeSourceType, supportedTypes(changeSourceBuilders))
	}
	if err := build(&out, asString(cs["spec"])); err != nil {
		return out, fmt.Errorf("change source %q: invalid spec: %w", out.Identifier, err)
	}
	return out, nil
}

func getServiceDependencyByType(sd map[string]interface{}) (nextgen.ServiceDependencyDto, error) {
	dependencyType := asString(sd["type"])
	out := nextgen.ServiceDependencyDto{
		MonitoredServiceIdentifier: asString(sd["monitored_service_identifier"]),
		Type_:                      nextgen.DependencyMetadataType(dependencyType),
	}
	switch dependencyType {
	case "KUBERNETES":
		v, err := decoded[nextgen.KubernetesDependencyMetadata](asString(sd["dependency_metadata"]))
		if err != nil {
			return out, fmt.Errorf("dependency %q: invalid dependency_metadata: %w", out.MonitoredServiceIdentifier, err)
		}
		out.KUBERNETES = v
	default:
		return out, fmt.Errorf("dependency %q: unsupported type %q (one of KUBERNETES)", out.MonitoredServiceIdentifier, dependencyType)
	}
	return out, nil
}
