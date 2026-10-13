package fakeharness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Dialect selects the request/response conventions of a Harness service family.
type Dialect int

const (
	// DialectNextgenDTO: the harness-go-sdk nextgen style. Writes wrap the object in
	// {"<bodyKey>": {...}}, responses are {"status":"SUCCESS","data":{"<bodyKey>":{...}}},
	// scope travels in query parameters (and in the body on writes), success is 200.
	DialectNextgenDTO Dialect = iota
	// DialectOpenAPIV1: the harness-openapi-go-client style. Plain bodies, scope in the path
	// (/v1/orgs/{org}/projects/{project}/...), 201 on create, 200 on read/update, 204 on delete,
	// errors are {"message": "..."}.
	DialectOpenAPIV1
)

// ScopeSpec names where a resource's org/project scope appears.
type ScopeSpec struct {
	QueryOrg     string // query parameter carrying the org on reads/deletes, e.g. "orgIdentifier"
	QueryProject string
	BodyOrg      string // object field carrying the org on writes, e.g. "orgIdentifier"
	BodyProject  string
	PathOrg      string // ServeMux wildcard name for the org in the base path, e.g. "org"
	PathProject  string
}

// APIError is a structured error a Validate hook can return.
type APIError struct {
	Status  int
	Code    string
	Message string
}

// Resource declares a CRUD API the fake serves generically.
type Resource struct {
	Kind     string
	Dialect  Dialect
	BasePath string // e.g. "/ng/api/connectors" or "/v1/orgs/{org}/projects/{project}/pipelines"
	BodyKey  string // wrapper key on writes/reads ("connector"); "" for a plain object
	// PlainRequest: writes send the bare object while responses still wrap it in BodyKey
	// (environmentsV2 takes an EnvironmentRequest and answers {"environment": ...}).
	PlainRequest bool
	IDField      string // object field holding the identifier, default "identifier"
	Scope        ScopeSpec
	// UpdateHasID: PUT <base>/{id} (projects, orgs, pipelines) rather than PUT <base> with the
	// identifier in the body (connectors).
	UpdateHasID bool
	// RequireParents: a create fails with 400 when the org/project it names does not exist in
	// the store, so scope-change tests are realistic.
	RequireParents bool
	// RoutesOnly: the API does not follow the generic CRUD shape, so only Routes are served
	// (IDPEntities). Kind, BasePath and RequireParents still apply.
	RoutesOnly bool
	// KindPattern is a regexp matching every path of the kind, for an API whose base path
	// varies with the scope (NotificationChannels); it replaces the pattern derived from
	// BasePath for Match{Kind} and MutatingMethods.
	KindPattern string
	// Decorate adds server-side fields to a stored object on create/update.
	Decorate func(obj map[string]any)
	// Validate rejects a create/update.
	Validate func(obj map[string]any) *APIError
	// Routes adds service-specific endpoints beside the generic CRUD ones, keyed by ServeMux
	// pattern relative to the stripped /gateway prefix; "{fakeid}" is the object identifier.
	Routes map[string]func(s *Server, res Resource, w http.ResponseWriter, r *http.Request)
}

// Organizations is /ng/api/organizations.
func Organizations() Resource {
	return Resource{Kind: KindOrganization, Dialect: DialectNextgenDTO, BasePath: "/ng/api/organizations", BodyKey: "organization", IDField: "identifier", UpdateHasID: true}
}

// Projects is /ng/api/projects.
func Projects() Resource {
	return Resource{Kind: KindProject, Dialect: DialectNextgenDTO, BasePath: "/ng/api/projects", BodyKey: "project", IDField: "identifier",
		Scope: ScopeSpec{QueryOrg: "orgIdentifier", BodyOrg: "orgIdentifier"}, UpdateHasID: true, RequireParents: true}
}

// Connectors is /ng/api/connectors.
func Connectors() Resource {
	return Resource{Kind: KindConnector, Dialect: DialectNextgenDTO, BasePath: "/ng/api/connectors", BodyKey: "connector", IDField: "identifier",
		Scope:          ScopeSpec{QueryOrg: "orgIdentifier", QueryProject: "projectIdentifier", BodyOrg: "orgIdentifier", BodyProject: "projectIdentifier"},
		RequireParents: true}
}

// SecretsText is /ng/api/v2/secrets.
func SecretsText() Resource {
	return Resource{Kind: KindSecret, Dialect: DialectNextgenDTO, BasePath: "/ng/api/v2/secrets", BodyKey: "secret", IDField: "identifier",
		Scope:       ScopeSpec{QueryOrg: "orgIdentifier", QueryProject: "projectIdentifier", BodyOrg: "orgIdentifier", BodyProject: "projectIdentifier"},
		UpdateHasID: true, RequireParents: true}
}

// PipelinesV1 is /v1/orgs/{org}/projects/{project}/pipelines.
func PipelinesV1() Resource {
	return Resource{Kind: KindPipeline, Dialect: DialectOpenAPIV1, BasePath: "/v1/orgs/{org}/projects/{project}/pipelines", IDField: "identifier",
		Scope: ScopeSpec{PathOrg: "org", PathProject: "project"}, UpdateHasID: true, RequireParents: true,
		Decorate: func(obj map[string]any) {
			if _, ok := obj["created"]; !ok {
				obj["created"] = 1700000000000
			}
			// The real API keeps no git details for inline pipelines.
			if gd, _ := obj["git_details"].(map[string]any); gd != nil && str(gd["store_type"]) == "INLINE" {
				delete(obj, "git_details")
			}
		},
		Routes: map[string]func(s *Server, res Resource, w http.ResponseWriter, r *http.Request){
			"PUT /v1/orgs/{org}/projects/{project}/pipelines/{fakeid}/git-metadata": func(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
				key := res.scopeOf(r, nil)
				key.ID = r.PathValue("fakeid")
				if !s.store.Has(res.Kind, key) {
					s.writeNotFound(res, w, key.ID)
					return
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				s.store.Mutate(res.Kind, key, func(obj map[string]any) {
					gd, _ := obj["git_details"].(map[string]any)
					if gd == nil {
						gd = map[string]any{}
					}
					for k, v := range body {
						gd[k] = v
					}
					obj["git_details"] = gd
				})
				writeJSON(w, http.StatusOK, map[string]any{"identifier": key.ID})
			},
		}}
}

// EnvironmentsV2 is /ng/api/environmentsV2: a plain EnvironmentRequest on writes (the
// identifier in the body, also on update), {"environment": ...} in responses.
func EnvironmentsV2() Resource {
	return Resource{Kind: KindEnvironment, Dialect: DialectNextgenDTO, BasePath: "/ng/api/environmentsV2", BodyKey: "environment", PlainRequest: true, IDField: "identifier",
		Scope:          ScopeSpec{QueryOrg: "orgIdentifier", QueryProject: "projectIdentifier", BodyOrg: "orgIdentifier", BodyProject: "projectIdentifier"},
		RequireParents: true}
}

// Platform returns the resources New mounts by default.
func Platform() []Resource {
	return []Resource{Organizations(), Projects(), Connectors(), SecretsText(), PipelinesV1()}
}

var wildcardRe = regexp.MustCompile(`\{[^/}]+\}`)

// Mount registers a resource's routes.
func (s *Server) Mount(res Resource) {
	if res.IDField == "" {
		res.IDField = "identifier"
	}
	base := res.BasePath
	s.mu.Lock()
	if res.KindPattern != "" {
		s.kinds[res.Kind] = regexp.MustCompile(res.KindPattern)
	} else {
		s.kinds[res.Kind] = basePathRegexp(base)
	}
	s.mu.Unlock()

	if !res.RoutesOnly {
		s.mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) { s.handleCreate(res, w, r) })
		s.mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) { s.handleList(res, w, r) })
		s.mux.HandleFunc("GET "+base+"/{fakeid}", func(w http.ResponseWriter, r *http.Request) { s.handleGet(res, w, r) })
		if res.UpdateHasID {
			s.mux.HandleFunc("PUT "+base+"/{fakeid}", func(w http.ResponseWriter, r *http.Request) { s.handleUpdate(res, w, r) })
		} else {
			s.mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) { s.handleUpdate(res, w, r) })
		}
		s.mux.HandleFunc("DELETE "+base+"/{fakeid}", func(w http.ResponseWriter, r *http.Request) { s.handleDelete(res, w, r) })
	}
	for pattern, h := range res.Routes {
		h := h
		s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) { h(s, res, w, r) })
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// scopeOf derives the object's Key (without ID) from the path, the query and the body.
func (res Resource) scopeOf(r *http.Request, obj map[string]any) Key {
	var k Key
	if res.Scope.PathOrg != "" {
		k.Org = r.PathValue(res.Scope.PathOrg)
	}
	if res.Scope.PathProject != "" {
		k.Project = r.PathValue(res.Scope.PathProject)
	}
	q := r.URL.Query()
	if res.Scope.QueryOrg != "" {
		if v := q.Get(res.Scope.QueryOrg); v != "" {
			k.Org = v
		} else if obj != nil {
			k.Org = str(obj[res.Scope.BodyOrg])
		}
	}
	if res.Scope.QueryProject != "" {
		if v := q.Get(res.Scope.QueryProject); v != "" {
			k.Project = v
		} else if obj != nil {
			k.Project = str(obj[res.Scope.BodyProject])
		}
	}
	return k
}

func (res Resource) describe(id string) string {
	return fmt.Sprintf("%s with identifier %s", res.Kind, id)
}

func (s *Server) writeError(res Resource, w http.ResponseWriter, e APIError) {
	switch res.Dialect {
	case DialectOpenAPIV1:
		writeJSON(w, e.Status, map[string]any{"message": e.Message})
	default:
		writeJSON(w, e.Status, map[string]any{"status": "ERROR", "code": e.Code, "message": e.Message, "correlationId": "fake"})
	}
}

func (s *Server) writeNotFound(res Resource, w http.ResponseWriter, id string) {
	msg := res.describe(id) + " not found"
	switch s.notFoundStyle() {
	case NotFoundHTML:
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "<html><head><title>404 Not Found</title></head><body><h1>404 Not Found</h1><p>%s</p></body></html>", msg)
	case NotFoundEmpty:
		w.WriteHeader(http.StatusNotFound)
	case NotFoundGatewayRuntime:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": msg, "code": 5, "message": msg, "details": []any{}})
	case NotFoundResource:
		s.writeError(res, w, APIError{Status: http.StatusNotFound, Code: "RESOURCE_NOT_FOUND_EXCEPTION", Message: msg})
	case NotFoundInvalidRequest:
		s.writeError(res, w, APIError{Status: http.StatusNotFound, Code: "INVALID_REQUEST", Message: msg})
	default:
		s.writeError(res, w, APIError{Status: http.StatusNotFound, Code: "ENTITY_NOT_FOUND", Message: msg})
	}
}

func (s *Server) writeObject(res Resource, w http.ResponseWriter, status int, obj map[string]any) {
	switch res.Dialect {
	case DialectOpenAPIV1:
		writeJSON(w, status, obj)
	default:
		data := obj
		if res.BodyKey != "" {
			data = map[string]any{res.BodyKey: obj}
		}
		writeJSON(w, status, map[string]any{"status": "SUCCESS", "data": data, "metaData": nil, "correlationId": "fake"})
	}
}

func (s *Server) decodeObject(res Resource, w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return nil, false
	}
	obj := body
	if res.BodyKey != "" && !res.PlainRequest {
		nested, _ := body[res.BodyKey].(map[string]any)
		if nested == nil {
			s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "request body must contain " + res.BodyKey})
			return nil, false
		}
		obj = nested
	}
	return obj, true
}

func (s *Server) parentsExist(res Resource, key Key) *APIError {
	if !res.RequireParents {
		return nil
	}
	if key.Org != "" && res.Kind != KindOrganization && !s.store.Has(KindOrganization, Key{ID: key.Org}) {
		return &APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: fmt.Sprintf("organization %s does not exist", key.Org)}
	}
	if key.Project != "" && res.Kind != KindProject && !s.store.Has(KindProject, Key{Org: key.Org, ID: key.Project}) {
		return &APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: fmt.Sprintf("project %s does not exist in organization %s", key.Project, key.Org)}
	}
	return nil
}

func (s *Server) handleCreate(res Resource, w http.ResponseWriter, r *http.Request) {
	obj, ok := s.decodeObject(res, w, r)
	if !ok {
		return
	}
	id := str(obj[res.IDField])
	if id == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: res.IDField + " is required"})
		return
	}
	key := res.scopeOf(r, obj)
	key.ID = id
	if res.Scope.BodyOrg != "" && key.Org != "" {
		obj[res.Scope.BodyOrg] = key.Org
	}
	if res.Scope.BodyProject != "" && key.Project != "" {
		obj[res.Scope.BodyProject] = key.Project
	}
	if res.Scope.PathOrg != "" {
		obj[res.Scope.PathOrg] = key.Org
	}
	if res.Scope.PathProject != "" {
		obj[res.Scope.PathProject] = key.Project
	}
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	if res.Validate != nil {
		if e := res.Validate(obj); e != nil {
			s.writeError(res, w, *e)
			return
		}
	}
	if s.store.Has(res.Kind, key) {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "DUPLICATE_FIELD", Message: res.describe(id) + " already exists"})
		return
	}
	if res.Decorate != nil {
		res.Decorate(obj)
	}
	s.store.Put(res.Kind, key, obj)
	s.afterCreate(res.Kind, key)
	status := http.StatusOK
	if res.Dialect == DialectOpenAPIV1 {
		status = http.StatusCreated
	}
	s.writeObject(res, w, status, obj)
}

func (s *Server) handleGet(res Resource, w http.ResponseWriter, r *http.Request) {
	key := res.scopeOf(r, nil)
	key.ID = r.PathValue("fakeid")
	if s.readGate(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		s.writeNotFound(res, w, key.ID)
		return
	}
	s.writeObject(res, w, http.StatusOK, obj)
}

func (s *Server) handleList(res Resource, w http.ResponseWriter, r *http.Request) {
	scope := res.scopeOf(r, nil)
	objs := s.store.List(res.Kind, scope)
	switch res.Dialect {
	case DialectOpenAPIV1:
		writeJSON(w, http.StatusOK, objs)
	default:
		content := make([]any, 0, len(objs))
		for _, obj := range objs {
			if res.BodyKey != "" {
				content = append(content, map[string]any{res.BodyKey: obj})
			} else {
				content = append(content, obj)
			}
		}
		n := len(content)
		writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": map[string]any{
			"content": content, "totalItems": n, "totalPages": 1, "pageItemCount": n, "pageSize": n, "pageIndex": 0, "empty": n == 0,
		}, "correlationId": "fake"})
	}
}

func (s *Server) handleUpdate(res Resource, w http.ResponseWriter, r *http.Request) {
	obj, ok := s.decodeObject(res, w, r)
	if !ok {
		return
	}
	key := res.scopeOf(r, obj)
	if res.UpdateHasID {
		key.ID = r.PathValue("fakeid")
		obj[res.IDField] = key.ID
	} else {
		key.ID = str(obj[res.IDField])
	}
	if key.ID == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: res.IDField + " is required"})
		return
	}
	if res.Scope.PathOrg != "" {
		obj[res.Scope.PathOrg] = key.Org
	}
	if res.Scope.PathProject != "" {
		obj[res.Scope.PathProject] = key.Project
	}
	if !s.store.Has(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	if res.Validate != nil {
		if e := res.Validate(obj); e != nil {
			s.writeError(res, w, *e)
			return
		}
	}
	if res.Decorate != nil {
		res.Decorate(obj)
	}
	s.store.Put(res.Kind, key, obj)
	s.writeObject(res, w, http.StatusOK, obj)
}

func (s *Server) handleDelete(res Resource, w http.ResponseWriter, r *http.Request) {
	key := res.scopeOf(r, nil)
	key.ID = r.PathValue("fakeid")
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	switch res.Dialect {
	case DialectOpenAPIV1:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": true, "correlationId": "fake"})
	}
}

// KindFor returns the kind whose base path matches path, or "".
func (s *Server) KindFor(path string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for kind, re := range s.kinds {
		if re.MatchString(strings.TrimPrefix(path, "/gateway")) {
			return kind
		}
	}
	return ""
}

// basePathRegexp matches a resource base path (whose {wildcards} match one path segment)
// and anything beneath it.
func basePathRegexp(base string) *regexp.Regexp {
	parts := wildcardRe.Split(base, -1)
	var b strings.Builder
	b.WriteString("^")
	for i, p := range parts {
		if i > 0 {
			b.WriteString(`[^/]+`)
		}
		b.WriteString(regexp.QuoteMeta(p))
	}
	b.WriteString("(/|$)")
	return regexp.MustCompile(b.String())
}
