package monitored_service

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/harness/harness-go-sdk/harness/nextgen"
)

// The health source builders turn the decoded `spec` JSON of a health source block into the
// SDK model. Misshapen input (a missing key, a wrong type, a definition the model cannot
// decode) is an error for the plan or apply, never a panic that kills the plugin.

// asString returns v as a string, or "" when it is absent or not a string.
func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// asList returns v as a list, or nil when it is absent or not a list.
func asList(v interface{}) []interface{} {
	l, _ := v.([]interface{})
	return l
}

// decodeInto re-encodes a decoded JSON value into the SDK model out.
func decodeInto(v interface{}, out interface{}, what string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("invalid %s %v: %w", what, v, err)
	}
	if err := safeUnmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid %s %v: %w", what, v, err)
	}
	return nil
}

// decodeList decodes every element of a list into T.
func decodeList[T any](v interface{}, what string) ([]T, error) {
	items := asList(v)
	out := make([]T, len(items))
	for i, item := range items {
		if err := decodeInto(item, &out[i], what); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// metricPacksOf decodes the metric packs of a health source spec, if it has any.
func metricPacksOf(hs map[string]interface{}) ([]nextgen.TimeSeriesMetricPackDto, error) {
	if hs["metricPacks"] == nil {
		return nil, nil
	}
	return getMetricPacks(hs, "metricPacks")
}

func getAppDynamicsHealthSource(hs map[string]interface{}) (nextgen.AppDynamicsHealthSource, error) {
	healthSource := nextgen.AppDynamicsHealthSource{
		ConnectorRef:    asString(hs["connectorRef"]),
		ApplicationName: asString(hs["applicationName"]),
		Feature:         asString(hs["feature"]),
		TierName:        asString(hs["tierName"]),
	}
	if hs["metricDefinitions"] != nil {
		defs, err := decodeList[nextgen.AppDMetricDefinitions](hs["metricDefinitions"], "AppDynamics metric definition")
		if err != nil {
			return healthSource, err
		}
		healthSource.MetricDefinitions = defs
	}
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getPrometheusHealthSource(hs map[string]interface{}) (nextgen.PrometheusHealthSource, error) {
	healthSource := nextgen.PrometheusHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.PrometheusMetricDefinition](hs["metricDefinitions"], "Prometheus metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getNewRelicHealthSource(hs map[string]interface{}) (nextgen.NewRelicHealthSource, error) {
	healthSource := nextgen.NewRelicHealthSource{
		ConnectorRef:    asString(hs["connectorRef"]),
		ApplicationName: asString(hs["applicationName"]),
		ApplicationId:   asString(hs["applicationId"]),
		Feature:         asString(hs["feature"]),
	}
	if hs["newRelicMetricDefinitions"] != nil {
		defs, err := decodeList[nextgen.NewRelicMetricDefinition](hs["newRelicMetricDefinitions"], "New Relic metric definition")
		if err != nil {
			return healthSource, err
		}
		healthSource.NewRelicMetricDefinitions = defs
	}
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getStackDriverHealthSource(hs map[string]interface{}) (nextgen.StackdriverMetricHealthSource, error) {
	healthSource := nextgen.StackdriverMetricHealthSource{
		ConnectorRef: asString(hs["connectorRef"]),
		GcpProjectId: asString(hs["gcpProjectId"]),
	}
	defs, err := decodeList[nextgen.StackdriverDefinition](hs["metricDefinitions"], "Stackdriver metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getDataDogHealthSource(hs map[string]interface{}) (nextgen.DatadogMetricHealthSource, error) {
	healthSource := nextgen.DatadogMetricHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.DatadogMetricHealthDefinition](hs["metricDefinitions"], "Datadog metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getDynatraceHealthSource(hs map[string]interface{}) (nextgen.DynatraceHealthSource, error) {
	healthSource := nextgen.DynatraceHealthSource{
		ConnectorRef: asString(hs["connectorRef"]),
		ServiceId:    asString(hs["serviceId"]),
		ServiceName:  asString(hs["serviceName"]),
		Feature:      asString(hs["feature"]),
	}
	if ids := asList(hs["serviceMethodIds"]); ids != nil {
		s := make([]string, len(ids))
		for i, v := range ids {
			s[i] = fmt.Sprint(v)
		}
		healthSource.ServiceMethodIds = s
	}
	if hs["metricDefinitions"] != nil {
		defs, err := decodeList[nextgen.DynatraceMetricDefinition](hs["metricDefinitions"], "Dynatrace metric definition")
		if err != nil {
			return healthSource, err
		}
		healthSource.MetricDefinitions = defs
	}
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getCustomHealthSource(hs map[string]interface{}) (nextgen.CustomHealthSourceMetric, error) {
	healthSource := nextgen.CustomHealthSourceMetric{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.CustomHealthMetricDefinition](hs["metricDefinitions"], "custom health metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getSplunkHealthSource(hs map[string]interface{}) (nextgen.SplunkMetricHealthSource, error) {
	healthSource := nextgen.SplunkMetricHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.SplunkMetricDefinition](hs["metricDefinitions"], "Splunk metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getCloudWatchHealthSource(hs map[string]interface{}) (nextgen.CloudWatchMetricsHealthSource, error) {
	healthSource := nextgen.CloudWatchMetricsHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.CloudWatchMetricDefinition](hs["metricDefinitions"], "CloudWatch metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getAwsPrometheusHealthSource(hs map[string]interface{}) (nextgen.AwsPrometheusHealthSource, error) {
	healthSource := nextgen.AwsPrometheusHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	defs, err := decodeList[nextgen.PrometheusMetricDefinition](hs["metricDefinitions"], "AWS Prometheus metric definition")
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricDefinitions = defs
	packs, err := metricPacksOf(hs)
	if err != nil {
		return healthSource, err
	}
	healthSource.MetricPacks = packs
	return healthSource, nil
}

func getNextGenHealthSource(hs map[string]interface{}) (nextgen.NextGenHealthSource, error) {
	healthSource := nextgen.NextGenHealthSource{ConnectorRef: asString(hs["connectorRef"])}
	healthSourceParamDto := nextgen.HealthSourceParamsDto{}
	if err := decodeInto(hs["healthSourceParams"], &healthSourceParamDto, "health source params"); err != nil {
		return healthSource, err
	}
	queryDefinitions := asList(hs["queryDefinitions"])
	queryDefinitionDtos := make([]nextgen.QueryDefinition, len(queryDefinitions))
	for i, queryDefinition := range queryDefinitions {
		data, _ := queryDefinition.(map[string]interface{})
		if data == nil {
			return healthSource, fmt.Errorf("invalid query definition %v: expected an object", queryDefinition)
		}

		queryParams := nextgen.QueryParamsDto{}
		if err := decodeInto(data["queryParams"], &queryParams, "query params"); err != nil {
			return healthSource, err
		}
		riskProfile := nextgen.RiskProfile{}
		if err := decodeInto(data["riskProfile"], &riskProfile, "risk profile"); err != nil {
			return healthSource, err
		}

		liveMonitoringEnabled, _ := strconv.ParseBool(asString(data["liveMonitoringEnabled"]))
		continuousVerificationEnabled, _ := strconv.ParseBool(asString(data["continuousVerificationEnabled"]))
		sliEnabled, _ := strconv.ParseBool(asString(data["sliEnabled"]))
		thresholds, err := getMetricThreshold(data)
		if err != nil {
			return healthSource, err
		}
		queryDefinitionDtos[i] = nextgen.QueryDefinition{
			Identifier:                    asString(data["identifier"]),
			Name:                          asString(data["name"]),
			GroupName:                     asString(data["groupName"]),
			LiveMonitoringEnabled:         liveMonitoringEnabled,
			ContinuousVerificationEnabled: continuousVerificationEnabled,
			SliEnabled:                    sliEnabled,
			Query:                         asString(data["query"]),
			MetricThresholds:              thresholds,
			QueryParams:                   &queryParams,
			RiskProfile:                   &riskProfile,
		}
	}
	healthSource.QueryDefinitions = queryDefinitionDtos
	healthSource.HealthSourceParams = &healthSourceParamDto

	return healthSource, nil
}

func getMetricPacks(hs map[string]interface{}, path string) ([]nextgen.TimeSeriesMetricPackDto, error) {
	metricPacks := asList(hs[path])
	metricPackDto := make([]nextgen.TimeSeriesMetricPackDto, len(metricPacks))
	for i, metricPack := range metricPacks {
		pack, _ := metricPack.(map[string]interface{})
		if pack == nil {
			return nil, fmt.Errorf("invalid metric pack %v: expected an object", metricPack)
		}
		thresholds, err := getMetricThreshold(pack)
		if err != nil {
			return nil, err
		}
		metricPackDto[i] = nextgen.TimeSeriesMetricPackDto{
			Identifier:       asString(pack["identifier"]),
			MetricThresholds: thresholds,
		}
	}
	return metricPackDto, nil
}

func getMetricThreshold(hs map[string]interface{}) ([]nextgen.MetricThreshold, error) {
	metricThresholds := asList(hs["metricThresholds"])
	metricThresholdDto := make([]nextgen.MetricThreshold, len(metricThresholds))
	for j, metricThreshold := range metricThresholds {
		threshold, _ := metricThreshold.(map[string]interface{})
		if threshold == nil {
			return nil, fmt.Errorf("invalid metric threshold %v: expected an object", metricThreshold)
		}
		dto, err := getMetricThresholdByType(threshold)
		if err != nil {
			return nil, err
		}
		metricThresholdDto[j] = dto
	}
	return metricThresholdDto, nil
}

func getMetricThresholdByType(hs map[string]interface{}) (nextgen.MetricThreshold, error) {
	metricThresholdType := asString(hs["type"])
	criteria := nextgen.MetricThresholdCriteria{}
	if err := decodeInto(hs["criteria"], &criteria, "metric threshold criteria"); err != nil {
		return nextgen.MetricThreshold{}, err
	}
	threshold := nextgen.MetricThreshold{
		MetricName:       asString(hs["metricName"]),
		MetricType:       asString(hs["metricType"]),
		MetricIdentifier: asString(hs["identifier"]),
		GroupName:        asString(hs["groupName"]),
		Type_:            nextgen.MetricThresholdType(metricThresholdType),
		Criteria:         &criteria,
	}
	switch metricThresholdType {
	case "FailImmediately":
		data := nextgen.FailMetricThresholdSpec{}
		if err := decodeInto(hs["spec"], &data, "metric threshold spec"); err != nil {
			return nextgen.MetricThreshold{}, err
		}
		threshold.FailImmediately = &data
	case "IgnoreThreshold":
		data := nextgen.IgnoreMetricThresholdSpec{}
		if err := decodeInto(hs["spec"], &data, "metric threshold spec"); err != nil {
			return nextgen.MetricThreshold{}, err
		}
		threshold.IgnoreThreshold = &data
	default:
		return nextgen.MetricThreshold{}, fmt.Errorf("invalid metric threshold type %q (expected FailImmediately or IgnoreThreshold)", metricThresholdType)
	}
	return threshold, nil
}
