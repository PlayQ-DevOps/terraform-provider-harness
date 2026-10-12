package fakeharness

import (
	"net/http"
	"strings"
)

// Match selects requests for a scenario rule. Zero fields match anything.
type Match struct {
	Method     string
	PathPrefix string // on the stripped path, e.g. "/ng/api/connectors"
	Kind       string // a mounted resource kind; matches its base path and sub-paths
}

func (s *Server) matches(m Match, r *http.Request) bool {
	if m.Method != "" && m.Method != r.Method {
		return false
	}
	if m.PathPrefix != "" && !strings.HasPrefix(r.URL.Path, m.PathPrefix) {
		return false
	}
	if m.Kind != "" {
		re := s.kinds[m.Kind]
		if re == nil || !re.MatchString(r.URL.Path) {
			return false
		}
	}
	return true
}

type failRule struct {
	match  Match
	status int
	body   string
	header http.Header
	active bool
}

type dropRule struct {
	match Match
	times int
}

// FailNext makes the next request matching m fail once with status and body. Rules queue in
// call order. 429 and 503 responses carry Retry-After: 0 unless header overrides it, so
// the provider's retrying client retries immediately and tests stay fast.
func (s *Server) FailNext(m Match, status int, body string, header ...http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule := &failRule{match: m, status: status, body: body, active: true}
	if len(header) > 0 {
		rule.header = header[0]
	}
	s.failNext = append(s.failNext, rule)
}

// FailAlways makes every request matching m fail with status and body until cancel is called.
func (s *Server) FailAlways(m Match, status int, body string) (cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule := &failRule{match: m, status: status, body: body, active: true}
	s.failAlways = append(s.failAlways, rule)
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		rule.active = false
	}
}

func (s *Server) takeFail(r *http.Request) *failRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, rule := range s.failNext {
		if s.matches(rule.match, r) {
			s.failNext = append(s.failNext[:i], s.failNext[i+1:]...)
			return rule
		}
	}
	for _, rule := range s.failAlways {
		if rule.active && s.matches(rule.match, r) {
			return rule
		}
	}
	return nil
}

// DropConnection closes the connection without a response for the next `times` requests
// matching m, so the client sees a transport error.
func (s *Server) DropConnection(m Match, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drops = append(s.drops, &dropRule{match: m, times: times})
}

func (s *Server) takeDrop(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, rule := range s.drops {
		if rule.times > 0 && s.matches(rule.match, r) {
			rule.times--
			if rule.times == 0 {
				s.drops = append(s.drops[:i], s.drops[i+1:]...)
			}
			return true
		}
	}
	return false
}

// NotFoundStyle selects the body the fake returns for a missing object. The real API is
// inconsistent across services and proxies, and the provider must treat all of these as
// "gone".
type NotFoundStyle int

const (
	// NotFoundEntity: 404 {"status":"ERROR","code":"ENTITY_NOT_FOUND","message":...} (nextgen).
	NotFoundEntity NotFoundStyle = iota
	// NotFoundResource: 404 {"status":"ERROR","code":"RESOURCE_NOT_FOUND_EXCEPTION",...}.
	NotFoundResource
	// NotFoundInvalidRequest: 404 {"status":"ERROR","code":"INVALID_REQUEST",...}.
	NotFoundInvalidRequest
	// NotFoundHTML: 404 with an HTML error page (a gateway or proxy answering).
	NotFoundHTML
	// NotFoundEmpty: 404 with no body at all.
	NotFoundEmpty
	// NotFoundGatewayRuntime: 404 {"error":"...","code":5,"message":...} (GitOps gateway, gRPC NotFound).
	NotFoundGatewayRuntime
)

// NotFoundStyles lists every style, for table-driven tests.
var NotFoundStyles = []NotFoundStyle{NotFoundEntity, NotFoundResource, NotFoundInvalidRequest, NotFoundHTML, NotFoundEmpty, NotFoundGatewayRuntime}

func (st NotFoundStyle) String() string {
	switch st {
	case NotFoundEntity:
		return "entity_not_found"
	case NotFoundResource:
		return "resource_not_found"
	case NotFoundInvalidRequest:
		return "invalid_request"
	case NotFoundHTML:
		return "html"
	case NotFoundEmpty:
		return "empty"
	case NotFoundGatewayRuntime:
		return "gateway_runtime"
	default:
		return "unknown"
	}
}

// SetNotFoundStyle selects how missing objects are reported.
func (s *Server) SetNotFoundStyle(st NotFoundStyle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notFound = st
}

func (s *Server) notFoundStyle() NotFoundStyle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notFound
}

// EventuallyConsistent makes the first `reads` GETs of every object of kind created from
// now on return not-found, reproducing a backend that indexes writes asynchronously.
func (s *Server) EventuallyConsistent(kind string, reads int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventually[kind] = reads
}

// GoneAfter lets the object answer `reads` more successful GETs, then deletes it.
func (s *Server) GoneAfter(kind string, key Key, reads int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.goneAfter[pendingKey{kind, key}] = reads
}

// Vanish deletes the object out of band: no request is recorded.
func (s *Server) Vanish(kind string, key Key) bool {
	return s.store.Delete(kind, key)
}

// readGate applies EventuallyConsistent and GoneAfter to a GET of the object and reports
// whether the object should be reported missing.
func (s *Server) readGate(kind string, key Key) (missing bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pk := pendingKey{kind, key}
	if n, ok := s.pending[pk]; ok {
		if n > 0 {
			s.pending[pk] = n - 1
			return true
		}
		delete(s.pending, pk)
	}
	if n, ok := s.goneAfter[pk]; ok {
		if n > 0 {
			s.goneAfter[pk] = n - 1
			return false
		}
		delete(s.goneAfter, pk)
		s.store.Delete(kind, key)
		return true
	}
	return false
}

func (s *Server) afterCreate(kind string, key Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.eventually[kind]; n > 0 {
		s.pending[pendingKey{kind, key}] = n
	}
}
