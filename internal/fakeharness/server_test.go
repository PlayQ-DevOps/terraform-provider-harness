package fakeharness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func do(t *testing.T, s *Server, method, path, body string, header ...string) (int, map[string]any, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, s.Endpoint()+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-api-key", s.APIKey())
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m, raw, resp.Header
}

func data(m map[string]any, key string) map[string]any {
	d, _ := m["data"].(map[string]any)
	if key == "" {
		return d
	}
	obj, _ := d[key].(map[string]any)
	return obj
}

func TestNextgenDialectRoundTrip(t *testing.T) {
	s := New(t)
	// Parents are required for project-level connectors.
	status, m, _, _ := do(t, s, "POST", "/ng/api/connectors?accountIdentifier=acct", `{"connector":{"identifier":"c1","name":"C1","orgIdentifier":"o","projectIdentifier":"p","type":"Github","spec":{"url":"x"}}}`)
	if status != 400 || m["code"] != "INVALID_REQUEST" {
		t.Fatalf("create without parents: %d %v", status, m)
	}
	if st, _, _, _ := do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`); st != 200 {
		t.Fatalf("create org: %d", st)
	}
	if st, m, _, _ := do(t, s, "POST", "/ng/api/projects?accountIdentifier=acct&orgIdentifier=o", `{"project":{"identifier":"p","name":"P","orgIdentifier":"o"}}`); st != 200 || data(m, "project")["identifier"] != "p" {
		t.Fatalf("create project: %d %v", st, m)
	}
	status, m, _, _ = do(t, s, "POST", "/ng/api/connectors?accountIdentifier=acct", `{"connector":{"identifier":"c1","name":"C1","orgIdentifier":"o","projectIdentifier":"p","type":"Github","spec":{"url":"x"}}}`)
	if status != 200 || m["status"] != "SUCCESS" || data(m, "connector")["name"] != "C1" {
		t.Fatalf("create connector: %d %v", status, m)
	}
	// Duplicate.
	if st, m, _, _ := do(t, s, "POST", "/ng/api/connectors?accountIdentifier=acct", `{"connector":{"identifier":"c1","name":"C1","orgIdentifier":"o","projectIdentifier":"p"}}`); st != 400 || m["code"] != "DUPLICATE_FIELD" {
		t.Fatalf("duplicate: %d %v", st, m)
	}
	// Read with scope in the query; the polymorphic spec round-trips untouched.
	st, m, _, _ := do(t, s, "GET", "/ng/api/connectors/c1?accountIdentifier=acct&orgIdentifier=o&projectIdentifier=p", "")
	if st != 200 || data(m, "connector")["type"] != "Github" || data(m, "connector")["spec"].(map[string]any)["url"] != "x" {
		t.Fatalf("get: %d %v", st, m)
	}
	// Wrong scope is not found (this is what a scope change looks like to the API).
	if st, _, _, _ := do(t, s, "GET", "/ng/api/connectors/c1?accountIdentifier=acct&orgIdentifier=o", ""); st != 404 {
		t.Fatalf("get at wrong scope: %d", st)
	}
	// Update (connectors PUT the base path with the identifier in the body).
	if st, m, _, _ := do(t, s, "PUT", "/ng/api/connectors?accountIdentifier=acct", `{"connector":{"identifier":"c1","name":"C1 renamed","orgIdentifier":"o","projectIdentifier":"p","type":"Github"}}`); st != 200 || data(m, "connector")["name"] != "C1 renamed" {
		t.Fatalf("update: %d %v", st, m)
	}
	// Update at a scope where it does not exist: 404 (the #1347 scenario).
	if st, _, _, _ := do(t, s, "PUT", "/ng/api/connectors?accountIdentifier=acct", `{"connector":{"identifier":"c1","name":"x","orgIdentifier":"o","projectIdentifier":"other"}}`); st != 404 {
		t.Fatalf("update at wrong scope: %d", st)
	}
	// List.
	if st, m, _, _ := do(t, s, "GET", "/ng/api/connectors?accountIdentifier=acct&orgIdentifier=o&projectIdentifier=p", ""); st != 200 || data(m, "")["totalItems"].(float64) != 1 {
		t.Fatalf("list: %d %v", st, m)
	}
	// Delete, then gone.
	if st, m, _, _ := do(t, s, "DELETE", "/ng/api/connectors/c1?accountIdentifier=acct&orgIdentifier=o&projectIdentifier=p", ""); st != 200 || m["data"] != true {
		t.Fatalf("delete: %d %v", st, m)
	}
	if st, _, _, _ := do(t, s, "DELETE", "/ng/api/connectors/c1?accountIdentifier=acct&orgIdentifier=o&projectIdentifier=p", ""); st != 404 {
		t.Fatalf("delete again: %d", st)
	}
	if s.Store().Has(KindConnector, Key{Org: "o", Project: "p", ID: "c1"}) {
		t.Fatal("store still has the connector")
	}
}

func TestOpenAPIDialectRoundTrip(t *testing.T) {
	s := New(t)
	do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`)
	do(t, s, "POST", "/ng/api/projects?accountIdentifier=acct&orgIdentifier=o", `{"project":{"identifier":"p","name":"P"}}`)
	st, m, _, _ := do(t, s, "POST", "/v1/orgs/o/projects/p/pipelines", `{"identifier":"pipe","name":"Pipe","pipeline_yaml":"pipeline:\n  name: Pipe\n"}`)
	if st != 201 || m["identifier"] != "pipe" || m["org"] != "o" || m["project"] != "p" {
		t.Fatalf("create: %d %v", st, m)
	}
	if st, m, _, _ := do(t, s, "GET", "/v1/orgs/o/projects/p/pipelines/pipe", ""); st != 200 || m["pipeline_yaml"] != "pipeline:\n  name: Pipe\n" {
		t.Fatalf("get: %d %v", st, m)
	}
	if st, m, _, _ := do(t, s, "GET", "/v1/orgs/o/projects/p/pipelines/nope", ""); st != 404 || !strings.Contains(m["message"].(string), "not found") {
		t.Fatalf("get missing: %d %v", st, m)
	}
	if st, m, _, _ := do(t, s, "PUT", "/v1/orgs/o/projects/p/pipelines/pipe", `{"identifier":"pipe","name":"Pipe 2","pipeline_yaml":"x"}`); st != 200 || m["name"] != "Pipe 2" {
		t.Fatalf("update: %d %v", st, m)
	}
	if st, _, _, _ := do(t, s, "DELETE", "/v1/orgs/o/projects/p/pipelines/pipe", ""); st != 204 {
		t.Fatalf("delete: %d", st)
	}
	if st, _, _, _ := do(t, s, "POST", "/v1/orgs/o/projects/missing/pipelines", `{"identifier":"x","name":"x"}`); st != 400 {
		t.Fatalf("create under missing project: %d", st)
	}
}

func TestAuth(t *testing.T) {
	s := New(t)
	if st, m, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", "", "x-api-key", "wrong"); st != 401 || m["code"] != "INVALID_TOKEN" {
		t.Fatalf("wrong key: %d %v", st, m)
	}
	open := New(t, WithAPIKey(""))
	if st, _, _, _ := do(t, open, "GET", "/ng/api/organizations/o?accountIdentifier=acct", "", "x-api-key", "anything"); st != 404 {
		t.Fatalf("auth disabled: %d", st)
	}
}

func TestNotFoundStyles(t *testing.T) {
	s := New(t)
	for _, style := range NotFoundStyles {
		s.SetNotFoundStyle(style)
		st, m, raw, h := do(t, s, "GET", "/ng/api/connectors/missing?accountIdentifier=acct", "")
		if st != 404 {
			t.Errorf("%s: status %d", style, st)
		}
		switch style {
		case NotFoundEntity:
			if m["code"] != "ENTITY_NOT_FOUND" {
				t.Errorf("%s: %s", style, raw)
			}
		case NotFoundResource:
			if m["code"] != "RESOURCE_NOT_FOUND_EXCEPTION" {
				t.Errorf("%s: %s", style, raw)
			}
		case NotFoundInvalidRequest:
			if m["code"] != "INVALID_REQUEST" {
				t.Errorf("%s: %s", style, raw)
			}
		case NotFoundHTML:
			if !strings.HasPrefix(h.Get("Content-Type"), "text/html") || !bytes.Contains(raw, []byte("<html>")) {
				t.Errorf("%s: %q %s", style, h.Get("Content-Type"), raw)
			}
		case NotFoundEmpty:
			if len(raw) != 0 {
				t.Errorf("%s: body %q", style, raw)
			}
		case NotFoundGatewayRuntime:
			if m["code"] != float64(5) {
				t.Errorf("%s: %s", style, raw)
			}
		}
	}
	// The OpenAPI dialect reports JSON {"message"} for the JSON styles and honours HTML/empty.
	s.SetNotFoundStyle(NotFoundEntity)
	if st, m, _, _ := do(t, s, "GET", "/v1/orgs/o/projects/p/pipelines/missing", ""); st != 404 || m["message"] == nil {
		t.Errorf("openapi not found: %d %v", st, m)
	}
	s.SetNotFoundStyle(NotFoundEmpty)
	if st, _, raw, _ := do(t, s, "GET", "/v1/orgs/o/projects/p/pipelines/missing", ""); st != 404 || len(raw) != 0 {
		t.Errorf("openapi empty not found: %d %q", st, raw)
	}
}

func TestFailRules(t *testing.T) {
	s := New(t)
	s.FailNext(Match{Method: "GET", Kind: KindOrganization}, 503, `{"status":"ERROR","message":"try later"}`)
	s.FailNext(Match{Method: "GET", Kind: KindOrganization}, 500, `<html>boom</html>`)
	st, _, _, h := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", "")
	if st != 503 || h.Get("Retry-After") != "0" || !strings.HasPrefix(h.Get("Content-Type"), "application/json") {
		t.Fatalf("first fail: %d %v", st, h)
	}
	if st, _, _, h := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 500 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatalf("second fail: %d %v", st, h)
	}
	if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 404 {
		t.Fatalf("rules consumed, expected plain 404: %d", st)
	}
	// Rules only match their selector.
	s.FailNext(Match{Method: "POST"}, 500, "")
	if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 404 {
		t.Fatalf("GET must not consume a POST rule: %d", st)
	}
	if st, _, _, _ := do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`); st != 500 {
		t.Fatalf("POST rule: %d", st)
	}
	cancel := s.FailAlways(Match{Kind: KindOrganization}, 429, "")
	for i := 0; i < 3; i++ {
		if st, _, _, h := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 429 || h.Get("Retry-After") != "0" {
			t.Fatalf("always: %d", st)
		}
	}
	cancel()
	if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 404 {
		t.Fatalf("after cancel: %d", st)
	}
}

func TestEventualConsistencyAndGoneAfter(t *testing.T) {
	s := New(t)
	s.EventuallyConsistent(KindOrganization, 2)
	do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`)
	for i, want := range []int{404, 404, 200, 200} {
		if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != want {
			t.Fatalf("read %d: %d, want %d", i, st, want)
		}
	}
	s.GoneAfter(KindOrganization, Key{ID: "o"}, 1)
	for i, want := range []int{200, 404, 404} {
		if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != want {
			t.Fatalf("gone-after read %d: %d, want %d", i, st, want)
		}
	}
	if s.Store().Has(KindOrganization, Key{ID: "o"}) {
		t.Fatal("GoneAfter must delete the object")
	}
	do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o2","name":"O2"}}`)
	// EventuallyConsistent applies to every create of the kind; a fresh object also 404s twice.
	if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o2?accountIdentifier=acct", ""); st != 404 {
		t.Fatalf("second object should also be eventually consistent: %d", st)
	}
	mark := s.Mark()
	if !s.Vanish(KindOrganization, Key{ID: "o2"}) {
		t.Fatal("Vanish should report the object existed")
	}
	if n := len(s.Since(mark)); n != 0 {
		t.Fatalf("Vanish must not record a request, got %d", n)
	}
}

func TestDropConnection(t *testing.T) {
	s := New(t)
	s.DropConnection(Match{Kind: KindOrganization}, 1)
	req, _ := http.NewRequest("GET", s.Endpoint()+"/ng/api/organizations/o?accountIdentifier=acct", nil)
	req.Header.Set("x-api-key", s.APIKey())
	_, err := http.DefaultClient.Do(req)
	if err == nil || !(errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection")) {
		t.Fatalf("expected a transport error, got %v", err)
	}
	if st, _, _, _ := do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", ""); st != 404 {
		t.Fatalf("after the drop the server answers again: %d", st)
	}
}

func TestRecorder(t *testing.T) {
	s := New(t)
	mark := s.Mark()
	do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`)
	do(t, s, "GET", "/ng/api/organizations/o?accountIdentifier=acct", "")
	do(t, s, "PUT", "/ng/api/organizations/o?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O2"}}`)
	do(t, s, "DELETE", "/ng/api/organizations/o?accountIdentifier=acct", "")
	do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"o","name":"O"}}`)
	reqs := s.Since(mark)
	if len(reqs) != 5 || reqs[0].Method != "POST" || reqs[0].Status != 200 || reqs[0].JSON()["organization"] == nil || reqs[0].Query.Get("accountIdentifier") != "acct" {
		t.Fatalf("recorded: %+v", reqs)
	}
	if n := s.Count(mark, "GET", "/ng/api/organizations/"); n != 1 {
		t.Fatalf("Count = %d", n)
	}
	s.AssertSequence(t, mark, KindOrganization, "POST", "PUT", "DELETE", "POST")
	s.AssertNoRequest(t, mark, "PATCH", "/")
	if got := fmt.Sprint(s.MutatingMethods(mark, KindConnector)); got != "[]" {
		t.Fatalf("connector writes: %s", got)
	}
	s.Reset()
	if len(s.Requests()) != 0 || s.Store().Len(KindOrganization) != 0 {
		t.Fatal("Reset must clear requests and the store")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("o%d", i)
			do(t, s, "POST", "/ng/api/organizations?accountIdentifier=acct", `{"organization":{"identifier":"`+id+`","name":"O"}}`)
			do(t, s, "GET", "/ng/api/organizations/"+id+"?accountIdentifier=acct", "")
			s.Store().Mutate(KindOrganization, Key{ID: id}, func(obj map[string]any) { obj["name"] = "drifted" })
			do(t, s, "DELETE", "/ng/api/organizations/"+id+"?accountIdentifier=acct", "")
		}(i)
	}
	wg.Wait()
	if n := s.Store().Len(KindOrganization); n != 0 {
		t.Fatalf("%d organizations left", n)
	}
	if n := len(s.Requests()); n != 60 {
		t.Fatalf("recorded %d requests, want 60", n)
	}
}

func TestKindMatching(t *testing.T) {
	s := New(t)
	if k := s.KindFor("/v1/orgs/o/projects/p/pipelines/x"); k != KindPipeline {
		t.Fatalf("pipeline kind: %q", k)
	}
	if k := s.KindFor("/gateway/ng/api/connectors"); k != KindConnector {
		t.Fatalf("connector kind: %q", k)
	}
	if k := s.KindFor("/ng/api/connectorsx"); k != "" {
		t.Fatalf("prefix must not match a different path: %q", k)
	}
}
