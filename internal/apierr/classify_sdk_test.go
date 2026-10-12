package apierr

// These tests send real SDK clients at an httptest server and classify whatever error
// each generated client returns, so the classifier is proven against the actual error
// types and bodies rather than synthetic stand-ins.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harness/harness-go-sdk/harness/chaos"
	"github.com/harness/harness-go-sdk/harness/code"
	"github.com/harness/harness-go-sdk/harness/dbops"
	"github.com/harness/harness-go-sdk/harness/har"
	"github.com/harness/harness-go-sdk/harness/idp"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/harness-go-sdk/harness/policymgmt"
	"github.com/harness/harness-go-sdk/harness/svcdiscovery"
	openapi "github.com/harness/harness-openapi-go-client/nextgen"
	"github.com/hashicorp/go-retryablehttp"
)

func serve(t *testing.T, status int, contentType, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// noRetry returns the client shape the provider uses (retryablehttp) without retries and
// with the passthrough error handler, so the SDK receives 5xx responses and their bodies
// instead of retryablehttp's nil-response "giving up" error.
func noRetry() *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.RetryMax = 0
	c.Logger = nil
	c.ErrorHandler = retryablehttp.PassthroughErrorHandler
	return c
}

type sdkCase struct {
	name        string
	status      int
	contentType string
	body        string
	call        func(srv *httptest.Server) (error, *http.Response)
	kind        Kind
	code        string
	message     string
	nonJSON     bool
}

func runSDKCases(t *testing.T, cases []sdkCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := serve(t, tc.status, tc.contentType, tc.body)
			var err error
			var rs *http.Response
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("SDK call panicked: %v", r)
					}
				}()
				err, rs = tc.call(srv)
			}()
			if err == nil {
				t.Fatalf("expected an error from the SDK for HTTP %d", tc.status)
			}
			var got Classified
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Classify panicked on %T: %v", err, r)
					}
				}()
				got = Classify(err, rs)
			}()
			if got.Kind != tc.kind {
				t.Errorf("kind = %s, want %s (%+v)", got.Kind, tc.kind, got)
			}
			if got.Status != tc.status {
				t.Errorf("status = %d, want %d", got.Status, tc.status)
			}
			if tc.code != "" && got.Code != tc.code {
				t.Errorf("code = %q, want %q", got.Code, tc.code)
			}
			if tc.message != "" && !strings.Contains(got.Message, tc.message) {
				t.Errorf("message = %q, want it to contain %q", got.Message, tc.message)
			}
			if got.NonJSON != tc.nonJSON {
				t.Errorf("nonJSON = %v, want %v", got.NonJSON, tc.nonJSON)
			}
		})
	}
}

func TestClassifySDK_Nextgen(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := nextgen.NewAPIClient(&nextgen.Configuration{BasePath: srv.URL, AccountId: "acct", ApiKey: "key", HTTPClient: noRetry()})
		_, rs, err := c.ConnectorsApi.GetConnector(context.Background(), "acct", "conn", &nextgen.ConnectorsApiGetConnectorOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404 entity not found", status: 404, contentType: "application/json", body: `{"status":"ERROR","code":"ENTITY_NOT_FOUND","message":"Connector conn not found"}`, call: call, kind: NotFound, code: "ENTITY_NOT_FOUND", message: "Connector conn not found"},
		{name: "404 unrelated code", status: 404, contentType: "application/json", body: `{"status":"ERROR","code":"INVALID_REQUEST","message":"nope"}`, call: call, kind: NotFound, code: "INVALID_REQUEST"},
		{name: "404 html from a proxy", status: 404, contentType: "text/html", body: `<html><body>404 page not found</body></html>`, call: call, kind: NotFound, nonJSON: true},
		{name: "404 empty body", status: 404, body: ``, call: call, kind: NotFound},
		{name: "400 duplicate", status: 400, contentType: "application/json", body: `{"status":"ERROR","code":"DUPLICATE_FIELD","message":"Connector with identifier conn already exists"}`, call: call, kind: Conflict, code: "DUPLICATE_FIELD"},
		{name: "400 entity not found", status: 400, contentType: "application/json", body: `{"status":"ERROR","code":"ENTITY_NOT_FOUND","message":"missing"}`, call: call, kind: NotFound},
		{name: "401", status: 401, contentType: "application/json", body: `{"status":"ERROR","code":"INVALID_TOKEN","message":"Invalid API key"}`, call: call, kind: Unauthorized, message: "Invalid API key"},
		{name: "403", status: 403, contentType: "application/json", body: `{"status":"ERROR","code":"NG_ACCESS_DENIED","message":"Missing permission"}`, call: call, kind: Forbidden},
		{name: "500", status: 500, contentType: "application/json", body: `{"status":"ERROR","code":"UNKNOWN_ERROR","message":"boom"}`, call: call, kind: Transient, message: "boom"},
		{name: "502 html", status: 502, contentType: "text/html", body: `<html>Bad Gateway</html>`, call: call, kind: Transient, nonJSON: true},
		{name: "gitops gateway body on 500", status: 500, contentType: "application/json", body: `{"error":"application not found","code":5,"message":"application not found","details":[]}`, call: call, kind: NotFound, code: "NotFound"},
	})
}

func TestClassifySDK_NextgenTransport(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := nextgen.NewAPIClient(&nextgen.Configuration{BasePath: srv.URL, AccountId: "acct", HTTPClient: noRetry()})
	_, rs, err := c.ConnectorsApi.GetConnector(context.Background(), "acct", "conn", &nextgen.ConnectorsApiGetConnectorOpts{})
	if err == nil {
		t.Fatal("expected a transport error")
	}
	got := Classify(err, rs)
	if got.Kind != Transient || !got.Transport {
		t.Errorf("transport error classified as %+v", got)
	}
}

func TestClassifySDK_OpenAPI(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := openapi.NewAPIClient(&openapi.Configuration{BasePath: srv.URL, AccountId: "acct", HTTPClient: noRetry()})
		_, rs, err := c.PipelinesApi.GetPipeline(context.Background(), "org", "proj", "pipe", &openapi.PipelinesApiGetPipelineOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"message":"Pipeline with identifier pipe not found"}`, call: call, kind: NotFound, message: "Pipeline with identifier pipe not found"},
		{name: "400", status: 400, contentType: "application/json", body: `{"message":"Invalid yaml"}`, call: call, kind: BadRequest, message: "Invalid yaml"},
		{name: "500 html", status: 500, contentType: "text/html", body: `<html>oops</html>`, call: call, kind: Transient, nonJSON: true},
	})
}

func TestClassifySDK_HAR(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := har.NewAPIClient(&har.Configuration{BasePath: srv.URL, HTTPClient: noRetry()})
		_, rs, err := c.RegistriesApi.GetRegistry(context.Background(), "acct/reg")
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"code":"NOT_FOUND","message":"registry not found"}`, call: call, kind: NotFound, code: "NOT_FOUND", message: "registry not found"},
		{name: "400", status: 400, contentType: "application/json", body: `{"code":"BAD_REQUEST","message":"invalid"}`, call: call, kind: BadRequest, message: "invalid"},
	})
}

func TestClassifySDK_IDP(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := idp.NewAPIClient(&idp.Configuration{BasePath: srv.URL, HTTPClient: noRetry()})
		_, rs, err := c.EntitiesApi.GetEntity(context.Background(), "account", "component", "svc", &idp.EntitiesApiGetEntityOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"message":"entity not found"}`, call: call, kind: NotFound, message: "entity not found"},
		{name: "500 saying not found is transient without a predicate", status: 500, contentType: "application/json", body: `{"message":"entity not found"}`, call: call, kind: Transient, message: "entity not found"},
	})
}

func TestClassifySDK_Code(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := code.NewAPIClient(&code.Configuration{BasePath: srv.URL, HTTPClient: noRetry()})
		_, rs, err := c.WebhookApi.GetWebhook(context.Background(), "acct", "repo", "hook", &code.WebhookApiGetWebhookOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"message":"Not Found"}`, call: call, kind: NotFound},
		{name: "409", status: 409, contentType: "application/json", body: `{"message":"webhook already exists"}`, call: call, kind: Conflict},
		{name: "422", status: 422, contentType: "application/json", body: `{"message":"invalid"}`, call: call, kind: BadRequest},
	})
}

func TestClassifySDK_Chaos(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := chaos.NewAPIClient(&chaos.Configuration{BasePath: srv.URL, HTTPClient: srv.Client()})
		_, rs, err := c.DefaultApi.GetChaosV2Experiment(context.Background(), "acct", "org", "proj", "exp")
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"code":"E404","description":"experiment exp not found","message":"not found"}`, call: call, kind: NotFound, message: "experiment exp not found"},
		{name: "500 graceful pattern needs a predicate", status: 500, contentType: "application/json", body: `{"code":"E500","description":"no matching infra found","message":"error"}`, call: call, kind: Transient, message: "no matching infra"},
	})
}

func TestClassifySDK_DBOps(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := dbops.NewAPIClient(&dbops.Configuration{BasePath: srv.URL, HTTPClient: srv.Client()})
		_, rs, err := c.DatabaseSchemaApi.V1GetProjDbSchema(context.Background(), "org", "proj", "schema", &dbops.DatabaseSchemaApiV1GetProjDbSchemaOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"message":"schema not found"}`, call: call, kind: NotFound, message: "schema not found"},
		{name: "403", status: 403, contentType: "application/json", body: `{"message":"denied"}`, call: call, kind: Forbidden},
	})
}

func TestClassifySDK_PolicyMgmt(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := policymgmt.NewAPIClient(&policymgmt.Configuration{BasePath: srv.URL, HTTPClient: noRetry()})
		_, rs, err := c.PoliciesApi.PoliciesFind(context.Background(), "pol", &policymgmt.PoliciesApiPoliciesFindOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404 goa", status: 404, contentType: "application/json", body: `{"name":"not_found","id":"x","message":"policy not found","temporary":false,"timeout":false,"fault":false}`, call: call, kind: NotFound, code: "not_found", message: "policy not found"},
		{name: "503 goa temporary", status: 503, contentType: "application/json", body: `{"name":"unavailable","id":"x","message":"try again","temporary":true,"timeout":false,"fault":false}`, call: call, kind: Transient},
	})
}

func TestClassifySDK_ServiceDiscovery(t *testing.T) {
	call := func(srv *httptest.Server) (error, *http.Response) {
		c := svcdiscovery.NewAPIClient(&svcdiscovery.Configuration{BasePath: srv.URL, HTTPClient: srv.Client()})
		_, rs, err := c.AgentApi.GetAgent(context.Background(), "agent", "acct", "env", &svcdiscovery.AgentApiGetAgentOpts{})
		return err, rs
	}
	runSDKCases(t, []sdkCase{
		{name: "404", status: 404, contentType: "application/json", body: `{"description":"agent not found","message":"not found"}`, call: call, kind: NotFound, message: "agent not found"},
		{name: "500", status: 500, contentType: "application/json", body: `{"description":"db down","message":"internal"}`, call: call, kind: Transient},
	})
}
