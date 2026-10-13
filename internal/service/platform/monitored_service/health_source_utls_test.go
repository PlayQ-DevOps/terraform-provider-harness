package monitored_service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Regression test for CDS-131295: gcpProjectId set in the Stackdriver health source
// spec must be carried through into the nextgen.StackdriverMetricHealthSource struct
// that gets sent to the backend, instead of being silently dropped.
func TestGetStackDriverHealthSource_GcpProjectId(t *testing.T) {
	hs := map[string]interface{}{
		"connectorRef":      "account.TestGCP",
		"gcpProjectId":      "prj-ch-sharedservices-test",
		"metricDefinitions": []interface{}{},
	}
	result, err := getStackDriverHealthSource(hs)
	if err != nil {
		t.Fatal(err)
	}
	if result.GcpProjectId != "prj-ch-sharedservices-test" {
		t.Errorf("expected GcpProjectId to be %q, got %q", "prj-ch-sharedservices-test", result.GcpProjectId)
	}
}

func TestGetStackDriverHealthSource_GcpProjectIdOmitted(t *testing.T) {
	hs := map[string]interface{}{
		"connectorRef":      "account.TestGCP",
		"metricDefinitions": []interface{}{},
	}
	result, err := getStackDriverHealthSource(hs)
	if err != nil {
		t.Fatal(err)
	}
	if result.GcpProjectId != "" {
		t.Errorf("expected GcpProjectId to be empty when omitted, got %q", result.GcpProjectId)
	}
}

// appDynamicsSpec is a health source spec with one metric threshold whose type is the
// format argument.
const appDynamicsSpec = `{"connectorRef":"account.appd","applicationName":"app","tierName":"tier","metricPacks":[{"identifier":"Performance","metricThresholds":[{"type":"%s","metricName":"m","spec":{}}]}]}`

func healthSourceBlock(typ, spec string) map[string]interface{} {
	return map[string]interface{}{"name": "hs", "identifier": "hs", "type": typ, "version": "", "spec": spec}
}

func TestGetHealthSourceByType(t *testing.T) {
	// Misshapen input is an error, never a panic.
	for name, hs := range map[string]map[string]interface{}{
		"unsupported type":       healthSourceBlock("Nope", `{}`),
		"spec not JSON":          healthSourceBlock("AppDynamics", `{`),
		"unknown threshold type": healthSourceBlock("AppDynamics", fmt.Sprintf(appDynamicsSpec, "Bogus")),
		"raw spec wrong shape":   healthSourceBlock("StackdriverLog", `{"connectorRef":{"nested":true}}`),
	} {
		if _, err := getHealthSourceByType(hs); err == nil {
			t.Errorf("%s: expected an error", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}

	out, err := getHealthSourceByType(healthSourceBlock("AppDynamics", fmt.Sprintf(appDynamicsSpec, "FailImmediately")))
	if err != nil {
		t.Fatal(err)
	}
	if out.Type_ != nextgen.HealthSourceTypes.AppDynamics || out.AppDynamics == nil || out.AppDynamics.ApplicationName != "app" {
		t.Fatalf("typed spec not built: %+v", out)
	}
	if th := out.AppDynamics.MetricPacks[0].MetricThresholds[0]; th.FailImmediately == nil || th.MetricName != "m" {
		t.Errorf("threshold not decoded: %+v", th)
	}

	out, err = getHealthSourceByType(healthSourceBlock("StackdriverLog", `{"connectorRef":"account.gcp"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.StackdriverLog == nil || out.StackdriverLog.ConnectorRef != "account.gcp" {
		t.Errorf("raw spec not decoded: %+v", out)
	}
}

// The SDK's MetricThreshold.UnmarshalJSON panics on an unknown type; the decoder reports
// it as an error.
func TestSafeUnmarshalRecoversSDKPanics(t *testing.T) {
	var th nextgen.MetricThreshold
	if err := safeUnmarshal([]byte(`{"type":"Bogus","spec":{}}`), &th); err == nil || !strings.Contains(err.Error(), "Bogus") {
		t.Errorf("expected the SDK panic as an error, got %v", err)
	}
}

func TestChangeSourcesAndDependencies(t *testing.T) {
	cs, err := getChangeSourceByType(map[string]interface{}{"name": "cd", "identifier": "cd", "type": "HarnessCDNextGen", "enabled": true, "category": "Deployment", "spec": ""})
	if err != nil {
		t.Fatal(err)
	}
	if cs.HarnessCDNextGen == nil || !cs.Enabled {
		t.Errorf("an empty spec decodes as an empty object: %+v", cs)
	}
	if _, err := getChangeSourceByType(map[string]interface{}{"identifier": "x", "type": "Nope"}); err == nil {
		t.Error("an unsupported change source type is an error")
	}
	dep, err := getServiceDependencyByType(map[string]interface{}{"monitored_service_identifier": "ms", "type": "KUBERNETES", "dependency_metadata": `{"namespace":"ns"}`})
	if err != nil {
		t.Fatal(err)
	}
	if dep.KUBERNETES == nil {
		t.Errorf("dependency metadata not decoded: %+v", dep)
	}
	if _, err := getServiceDependencyByType(map[string]interface{}{"monitored_service_identifier": "ms", "type": "Nope"}); err == nil {
		t.Error("an unsupported dependency type is an error")
	}
}

func TestBuildMonitoredServiceRequest(t *testing.T) {
	request := func(spec string) map[string]interface{} {
		return map[string]interface{}{
			"org_id": "o", "project_id": "p", "identifier": "ms",
			"request": []interface{}{map[string]interface{}{
				"name": "ms", "type": "Application", "service_ref": "svc", "environment_ref": "env",
				"tags":           []interface{}{"team:sre", "nocolon"},
				"health_sources": []interface{}{healthSourceBlock("AppDynamics", spec)},
			}},
		}
	}
	d := schema.TestResourceDataRaw(t, ResourceMonitoredService().Schema, request(fmt.Sprintf(appDynamicsSpec, "Bogus")))
	if _, err := buildMonitoredServiceRequest(d); err == nil || !strings.Contains(err.Error(), "Bogus") {
		t.Fatalf("expected the bad threshold type as an error, got %v", err)
	}
	d = schema.TestResourceDataRaw(t, ResourceMonitoredService().Schema, request(fmt.Sprintf(appDynamicsSpec, "FailImmediately")))
	req, err := buildMonitoredServiceRequest(d)
	if err != nil {
		t.Fatal(err)
	}
	if req.Identifier != "ms" || req.Tags["team"] != "sre" || req.Tags["nocolon"] != "" {
		t.Errorf("request not built: %+v", req)
	}
	if len(req.Sources.HealthSources) != 1 || req.Sources.HealthSources[0].AppDynamics == nil {
		t.Errorf("health source not built: %+v", req.Sources)
	}
}
