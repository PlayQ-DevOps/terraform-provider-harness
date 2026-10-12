package apierr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// fakeSwaggerErr mimics the generated SDK error types: an error string (usually the raw
// HTTP status text) plus the raw body.
type fakeSwaggerErr struct {
	msg  string
	body string
}

func (e fakeSwaggerErr) Error() string { return e.msg }
func (e fakeSwaggerErr) Body() []byte  { return []byte(e.body) }

type fakeNetErr struct{}

func (fakeNetErr) Error() string   { return "i/o timeout" }
func (fakeNetErr) Timeout() bool   { return true }
func (fakeNetErr) Temporary() bool { return true }

func resp(status int) *http.Response {
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status))}
}

func swagger(status int, body string) (error, *http.Response) {
	return fakeSwaggerErr{msg: fmt.Sprintf("%d %s", status, http.StatusText(status)), body: body}, resp(status)
}

func TestClassify(t *testing.T) {
	var _ net.Error = fakeNetErr{}

	cases := []struct {
		name      string
		err       error
		resp      *http.Response
		kind      Kind
		status    int
		code      string
		message   string // substring
		transport bool
		nonJSON   bool
	}{
		{name: "nil error", err: nil, kind: None},
		{name: "plain error without response", err: errors.New("boom"), kind: Unknown, message: "boom"},
		{name: "url error is transport", err: &url.Error{Op: "Get", URL: "https://app.harness.io/gateway/x", Err: errors.New("dial tcp: connection refused")}, kind: Transient, transport: true, message: "could not reach the Harness API"},
		{name: "net timeout is transport", err: fakeNetErr{}, kind: Transient, transport: true},
		{name: "context cancelled", err: fmt.Errorf("Get x: %w", context.Canceled), kind: Unknown, message: "cancelled"},
		{name: "retry budget exhausted", err: errors.New("GET https://app.harness.io/gateway/x giving up after 11 attempt(s)"), kind: Transient, message: "retry budget"},

		{name: "404 entity not found envelope", kind: NotFound, status: 404, code: "ENTITY_NOT_FOUND", message: "Connector x not found"},
		{name: "404 resource not found exception", kind: NotFound, status: 404, code: "RESOURCE_NOT_FOUND_EXCEPTION"},
		{name: "404 with unrelated code stays not found", kind: NotFound, status: 404, code: "INVALID_REQUEST", message: "bad"},
		{name: "404 html body", kind: NotFound, status: 404, nonJSON: true, message: "non-JSON response body"},
		{name: "404 empty body", kind: NotFound, status: 404, message: "Not Found"},
		{name: "404 message only in responseMessages", kind: NotFound, status: 404, code: "INVALID_REQUEST", message: "No connector exists with id x"},
		{name: "410 gone", kind: NotFound, status: 410},
		{name: "400 with entity not found code", kind: NotFound, status: 400, code: "ENTITY_NOT_FOUND"},
		{name: "400 duplicate field", kind: Conflict, status: 400, code: "DUPLICATE_FIELD", message: "already exists"},
		{name: "400 validation errors list", kind: BadRequest, status: 400, code: "INVALID_REQUEST", message: "name must not be empty"},
		{name: "409 conflict", kind: Conflict, status: 409, message: "exists"},
		{name: "422 unprocessable", kind: BadRequest, status: 422, message: "invalid"},
		{name: "401 unauthorized", kind: Unauthorized, status: 401},
		{name: "403 forbidden", kind: Forbidden, status: 403},
		{name: "403 with not-found code is still forbidden", kind: Forbidden, status: 403, code: "ENTITY_NOT_FOUND"},
		{name: "429 rate limited", kind: Transient, status: 429},
		{name: "500 server error", kind: Transient, status: 500, message: "boom"},
		{name: "502 html body", kind: Transient, status: 502, nonJSON: true},
		{name: "501 not implemented is not transient", kind: Unknown, status: 501},

		{name: "gitops gateway not found", kind: NotFound, status: 500, code: "NotFound", message: "application not found"},
		{name: "gitops gateway permission denied", kind: Forbidden, status: 500, code: "PermissionDenied"},
		{name: "gitops gateway already exists", kind: Conflict, status: 500, code: "AlreadyExists"},
		{name: "argo rpc not found inside message", kind: NotFound, status: 500, message: "rpc error: code = NotFound"},
		{name: "goa not found", kind: NotFound, status: 404, code: "not_found", message: "policy not found"},
		{name: "goa temporary failure", kind: Transient, status: 503, code: "unavailable"},
		{name: "chaos description appended", kind: BadRequest, status: 400, code: "E400", message: "summary: details"},

		{name: "undefined response type on 200", err: errors.New("undefined response type"), resp: resp(200), kind: Unknown, status: 200, nonJSON: true},
		{name: "split string error", err: errors.New("environments list: 404 404 Not Found: {}"), kind: NotFound, status: 404},
		{name: "chaos graphql status in text", err: errors.New("unexpected status code: 404, body: {}"), kind: NotFound, status: 404},
		{name: "chaos graphql message cleaned", err: errors.New("graphql error: hub not found (path: [getHub])"), kind: Unknown, message: "hub not found"},
		{name: "firstgen unauthorized", err: errors.New("unauthorized"), kind: Unauthorized, message: "unauthorized"},
	}

	bodies := map[string]string{
		"404 entity not found envelope":              `{"status":"ERROR","code":"ENTITY_NOT_FOUND","message":"Connector x not found","correlationId":"c"}`,
		"404 resource not found exception":           `{"status":"ERROR","code":"RESOURCE_NOT_FOUND_EXCEPTION","message":"gone"}`,
		"404 with unrelated code stays not found":    `{"code":"INVALID_REQUEST","message":"bad"}`,
		"404 html body":                              `<html><head><title>404</title></head><body>Not Found</body></html>`,
		"404 empty body":                             ``,
		"404 message only in responseMessages":       `{"status":"ERROR","code":"INVALID_REQUEST","responseMessages":[{"code":"INVALID_REQUEST","level":"ERROR","message":"No connector exists with id x"}]}`,
		"410 gone":                                   `{"message":"gone"}`,
		"400 with entity not found code":             `{"status":"ERROR","code":"ENTITY_NOT_FOUND","message":"missing"}`,
		"400 duplicate field":                        `{"status":"ERROR","code":"DUPLICATE_FIELD","message":"Connector with identifier x already exists"}`,
		"400 validation errors list":                 `{"status":"ERROR","code":"INVALID_REQUEST","errors":[{"fieldId":"name","error":"must not be empty"}]}`,
		"409 conflict":                               `{"message":"exists"}`,
		"422 unprocessable":                          `{"message":"invalid"}`,
		"401 unauthorized":                           `{"status":"ERROR","code":"INVALID_TOKEN","message":"Invalid API key"}`,
		"403 forbidden":                              `{"status":"ERROR","code":"NG_ACCESS_DENIED","message":"Missing permission"}`,
		"403 with not-found code is still forbidden": `{"code":"ENTITY_NOT_FOUND","message":"x"}`,
		"429 rate limited":                           ``,
		"500 server error":                           `{"status":"ERROR","code":"UNKNOWN_ERROR","message":"boom"}`,
		"502 html body":                              `<html>Bad Gateway</html>`,
		"501 not implemented is not transient":       ``,
		"gitops gateway not found":                   `{"error":"application not found","code":5,"message":"application not found","details":[]}`,
		"gitops gateway permission denied":           `{"error":"denied","code":7,"message":"denied"}`,
		"gitops gateway already exists":              `{"error":"exists","code":6,"message":"exists"}`,
		"argo rpc not found inside message":          `{"message":"rpc error: code = NotFound desc = applications.argoproj.io \"x\" not found"}`,
		"goa not found":                              `{"name":"not_found","id":"abc","message":"policy not found","temporary":false,"timeout":false,"fault":false}`,
		"goa temporary failure":                      `{"name":"unavailable","id":"abc","message":"try later","temporary":true,"timeout":false,"fault":false}`,
		"chaos description appended":                 `{"code":"E400","description":"details","message":"summary"}`,
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err, rs := tc.err, tc.resp
			if body, ok := bodies[tc.name]; ok {
				err, rs = swagger(tc.status, body)
			}
			var got Classified
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Classify panicked: %v", r)
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
			if got.Transport != tc.transport {
				t.Errorf("transport = %v, want %v", got.Transport, tc.transport)
			}
			if got.NonJSON != tc.nonJSON {
				t.Errorf("nonJSON = %v, want %v", got.NonJSON, tc.nonJSON)
			}
			if tc.kind != None && got.Message == "" {
				t.Errorf("message must never be empty for a classified error")
			}
			if tc.kind != None && !errors.Is(got, err) {
				t.Errorf("Classified must unwrap to the original error")
			}
		})
	}
}

func TestClassifiedError(t *testing.T) {
	cases := []struct {
		c    Classified
		want string
	}{
		{Classified{Kind: None}, ""},
		{Classified{Kind: Unknown, Message: "boom"}, "boom"},
		{Classified{Kind: NotFound, Status: 404, Message: "gone"}, "gone (HTTP 404)"},
		{Classified{Kind: NotFound, Status: 404, Code: "ENTITY_NOT_FOUND", Message: "gone"}, "gone (HTTP 404, code ENTITY_NOT_FOUND)"},
		{Classified{Kind: Unknown, Code: "x", Message: "m"}, "m (code x)"},
	}
	for _, tc := range cases {
		if got := tc.c.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestDescribe(t *testing.T) {
	if d := Describe(Classify(nil, nil)); d != "" {
		t.Errorf("Describe(None) = %q, want empty", d)
	}
	err, rs := swagger(401, `{"message":"Invalid API key"}`)
	if d := Describe(Classify(err, rs)); !strings.Contains(d, "platform_api_key") || !strings.Contains(d, "Invalid API key") {
		t.Errorf("401 description lacks hint or message: %q", d)
	}
	err, rs = swagger(403, `{"message":"nope"}`)
	if d := Describe(Classify(err, rs)); !strings.Contains(d, "required permission") {
		t.Errorf("403 description lacks hint: %q", d)
	}
	err, rs = swagger(502, `<html>Bad Gateway</html>`)
	if d := Describe(Classify(err, rs)); !strings.Contains(d, "non-JSON response") || !strings.Contains(d, "HARNESS_ENDPOINT") {
		t.Errorf("non-JSON description lacks endpoint hint: %q", d)
	}
	transport := &url.Error{Op: "Get", URL: "https://x", Err: errors.New("no such host")}
	if d := Describe(Classify(transport, nil)); !strings.Contains(d, "could not be reached") {
		t.Errorf("transport description lacks hint: %q", d)
	}
}

func TestGitRewrite(t *testing.T) {
	err, rs := swagger(400, `{"message":"No connector found with identifier account.github in scope"}`)
	if msg, ok := GitRewrite(Classify(err, rs), "account.github", "repo"); !ok || !strings.Contains(msg, "Invalid connector reference: account.github") {
		t.Errorf("connector rewrite = %q, %v", msg, ok)
	}
	err, rs = swagger(400, `{"message":"Please check the requested file path in repo myrepo"}`)
	if msg, ok := GitRewrite(Classify(err, rs), "account.github", "myrepo"); !ok || !strings.Contains(msg, "Invalid repository name: myrepo") {
		t.Errorf("repo rewrite = %q, %v", msg, ok)
	}
	err, rs = swagger(404, `{"message":"No connector found with identifier account.github"}`)
	if _, ok := GitRewrite(Classify(err, rs), "account.github", "repo"); ok {
		t.Errorf("rewrite must only apply to HTTP 400")
	}
}

func TestPredicates(t *testing.T) {
	err, rs := swagger(500, `{"code":"CHAOS_ERR","message":"no matching infra found for the experiment"}`)
	c := Classify(err, rs)
	if !StatusIs(404, 500)(c) || StatusIs(404)(c) {
		t.Error("StatusIs")
	}
	if !CodeIs("chaos_err")(c) || CodeIs("other")(c) {
		t.Error("CodeIs should match case-insensitively")
	}
	if !MessageHasPhrase("No Matching Infra")(c) || MessageHasPhrase("hub not found")(c) {
		t.Error("MessageHasPhrase")
	}
	if !And(StatusIs(500), MessageHasPhrase("no matching infra"))(c) || And(StatusIs(404), MessageHasPhrase("no matching infra"))(c) || And()(c) {
		t.Error("And")
	}
	if !Or(StatusIs(404), CodeIs("CHAOS_ERR"))(c) || Or()(c) {
		t.Error("Or")
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MessageHasPhrase must reject phrases shorter than MinPhraseLength")
			}
		}()
		MessageHasPhrase("not fnd")
	}()
	if !IsNotFound(fakeSwaggerErr{msg: "404 Not Found"}, resp(404)) || IsNotFound(fakeSwaggerErr{msg: "500 Internal Server Error"}, resp(500)) {
		t.Error("IsNotFound")
	}
	if !IsTransport(&url.Error{Op: "Get", URL: "x", Err: errors.New("refused")}, nil) || IsTransport(errors.New("x"), resp(500)) {
		t.Error("IsTransport")
	}
}
