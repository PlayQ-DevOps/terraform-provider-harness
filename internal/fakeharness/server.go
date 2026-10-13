// Package fakeharness is an in-process fake of the Harness API for offline provider
// tests. Point the provider's `endpoint` at Server.Endpoint() and the generated SDK
// clients talk to it like the real thing: objects round-trip through an in-memory store,
// and scenario knobs reproduce the failure modes the provider must handle (not-found
// bodies of every shape, transient errors, eventual consistency, out-of-band deletes,
// dropped connections). Every request is recorded so tests can assert on the wire.
//
// The package depends only on the standard library so it can never import-cycle with the
// provider; typed helpers (provider factories, lifecycle matrices) live in internal/acctest.
package fakeharness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Kinds of the resources Platform mounts by default.
const (
	KindOrganization = "organization"
	KindProject      = "project"
	KindConnector    = "connector"
	KindSecret       = "secret"
	KindPipeline     = "pipeline"
	KindEnvironment  = "environment"
)

// Server is a fake Harness API.
type Server struct {
	t   testing.TB
	srv *httptest.Server
	mux *http.ServeMux

	mu         sync.Mutex
	apiKey     string
	account    string
	store      *Store
	kinds      map[string]*regexp.Regexp // kind -> path regexp of its base
	requests   []Request
	seq        int
	failNext   []*failRule
	failAlways []*failRule
	drops      []*dropRule
	notFound   NotFoundStyle
	eventually map[string]int     // kind -> reads that 404 after each create
	pending    map[pendingKey]int // object -> remaining 404 reads
	goneAfter  map[pendingKey]int // object -> remaining successful reads
}

type pendingKey struct {
	kind string
	key  Key
}

// Option configures a Server.
type Option func(*Server)

// WithAPIKey sets the API key the fake requires in the x-api-key header. The default is
// "test-key"; an empty key disables the check.
func WithAPIKey(key string) Option { return func(s *Server) { s.apiKey = key } }

// WithAccount sets the account identifier the fake reports. The default is "acct".
func WithAccount(id string) Option { return func(s *Server) { s.account = id } }

// WithMounts mounts additional resources beyond Platform().
func WithMounts(res ...Resource) Option {
	return func(s *Server) {
		for _, r := range res {
			s.Mount(r)
		}
	}
}

// New starts a fake with the Platform() resources mounted and closes it when the test ends.
func New(t testing.TB, opts ...Option) *Server {
	t.Helper()
	s := &Server{
		t:          t,
		mux:        http.NewServeMux(),
		apiKey:     "test-key",
		account:    "acct",
		store:      NewStore(),
		kinds:      map[string]*regexp.Regexp{},
		eventually: map[string]int{},
		pending:    map[pendingKey]int{},
		goneAfter:  map[pendingKey]int{},
		notFound:   NotFoundEntity,
	}
	for _, r := range Platform() {
		s.Mount(r)
	}
	for _, opt := range opts {
		opt(s)
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.srv.Close)
	return s
}

// Endpoint is the value for the provider's `endpoint` attribute. It ends in /gateway, as
// the real endpoint does, because the provider derives other service endpoints from it.
func (s *Server) Endpoint() string { return s.srv.URL + "/gateway" }

// URL is the bare server URL, for clients configured without the /gateway suffix.
func (s *Server) URL() string { return s.srv.URL }

// Account returns the account identifier.
func (s *Server) Account() string { return s.account }

// APIKey returns the expected API key.
func (s *Server) APIKey() string { return s.apiKey }

// Store returns the object store.
func (s *Server) Store() *Store { return s.store }

// ProviderHCL returns a provider block pointing at the fake.
func (s *Server) ProviderHCL() string {
	return fmt.Sprintf("provider \"harness\" {\n  endpoint         = %q\n  account_id       = %q\n  api_key          = %q\n  platform_api_key = %q\n}\n", s.Endpoint(), s.account, s.apiKey, s.apiKey)
}

// Handle registers a custom route. pattern is a Go 1.22 ServeMux pattern ("GET /ng/api/x/{id}")
// relative to the stripped /gateway prefix.
func (s *Server) Handle(pattern string, h http.HandlerFunc) { s.mux.HandleFunc(pattern, h) }

// Reset clears the store, the recorded requests and every scenario knob.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.reset()
	s.requests = nil
	s.seq = 0
	s.failNext = nil
	s.failAlways = nil
	s.drops = nil
	s.notFound = NotFoundEntity
	s.eventually = map[string]int{}
	s.pending = map[pendingKey]int{}
	s.goneAfter = map[pendingKey]int{}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r = r.Clone(r.Context())
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/gateway")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mu.Lock()
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.requests = append(s.requests, Request{
			Seq: seq, Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(),
			Header: r.Header.Clone(), Body: body, Status: rec.status,
		})
		s.mu.Unlock()
	}()

	if s.apiKey != "" && r.Header.Get("x-api-key") != s.apiKey {
		writeJSON(rec, http.StatusUnauthorized, map[string]any{"status": "ERROR", "code": "INVALID_TOKEN", "message": "Invalid API key"})
		return
	}
	if s.takeDrop(r) {
		rec.status = 0
		panic(http.ErrAbortHandler)
	}
	if rule := s.takeFail(r); rule != nil {
		for k, vs := range rule.header {
			for _, v := range vs {
				rec.Header().Add(k, v)
			}
		}
		if (rule.status == http.StatusTooManyRequests || rule.status == http.StatusServiceUnavailable) && rec.Header().Get("Retry-After") == "" {
			rec.Header().Set("Retry-After", "0")
		}
		if rule.body != "" && rec.Header().Get("Content-Type") == "" {
			if strings.HasPrefix(strings.TrimSpace(rule.body), "{") {
				rec.Header().Set("Content-Type", "application/json")
			} else {
				rec.Header().Set("Content-Type", "text/html")
			}
		}
		rec.WriteHeader(rule.status)
		_, _ = io.WriteString(rec, rule.body)
		return
	}
	s.mux.ServeHTTP(rec, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Request is a recorded request.
type Request struct {
	Seq    int
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
	Status int
}

// JSON decodes the recorded body into a map, or returns nil.
func (r Request) JSON() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		return nil
	}
	return m
}

// Requests returns every recorded request in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// Mark returns a position to pass to Since/Count/Assert* so they only consider later requests.
func (s *Server) Mark() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// Since returns the requests recorded after mark.
func (s *Server) Since(mark int) []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mark > len(s.requests) {
		mark = len(s.requests)
	}
	out := make([]Request, len(s.requests)-mark)
	copy(out, s.requests[mark:])
	return out
}

// Count returns how many requests since mark match method (empty = any) and pathPrefix.
func (s *Server) Count(mark int, method, pathPrefix string) int {
	n := 0
	for _, r := range s.Since(mark) {
		if (method == "" || r.Method == method) && strings.HasPrefix(r.Path, pathPrefix) {
			n++
		}
	}
	return n
}

// AssertNoRequest fails the test if a request since mark matches method and pathPrefix.
func (s *Server) AssertNoRequest(t testing.TB, mark int, method, pathPrefix string) {
	t.Helper()
	for _, r := range s.Since(mark) {
		if (method == "" || r.Method == method) && strings.HasPrefix(r.Path, pathPrefix) {
			t.Errorf("unexpected %s %s (status %d)", r.Method, r.Path, r.Status)
		}
	}
}

// MutatingMethods returns the methods of the write requests (POST/PUT/PATCH/DELETE) made to
// kind since mark, in order.
func (s *Server) MutatingMethods(mark int, kind string) []string {
	s.mu.Lock()
	re := s.kinds[kind]
	s.mu.Unlock()
	var out []string
	for _, r := range s.Since(mark) {
		if re == nil || !re.MatchString(r.Path) {
			continue
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			out = append(out, r.Method)
		}
	}
	return out
}

// AssertSequence fails the test unless the write requests made to kind since mark are
// exactly methods, in order (reads are ignored).
func (s *Server) AssertSequence(t testing.TB, mark int, kind string, methods ...string) {
	t.Helper()
	got := s.MutatingMethods(mark, kind)
	if strings.Join(got, ",") != strings.Join(methods, ",") {
		t.Errorf("write requests to %s since mark: got %v, want %v", kind, got, methods)
	}
}
