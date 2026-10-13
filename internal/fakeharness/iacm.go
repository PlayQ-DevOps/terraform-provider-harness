package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindIacmDefaultPipeline is the store kind of IaCM default pipelines (IacmDefaultPipelines).
const KindIacmDefaultPipeline = "iacm_default_pipeline"

// IacmDefaultPipelines is /iacm/api/orgs/{org}/projects/{project}/settings/default-pipelines:
// a project's defaults are listed as a JSON array, upserted with a PUT and deleted with a
// DELETE whose body names the provisioner and operation. The store key ID is
// "<provisioner>-<operation>".
func IacmDefaultPipelines() Resource {
	base := "/iacm/api/orgs/{org}/projects/{project}/settings/default-pipelines"
	return Resource{Kind: KindIacmDefaultPipeline, Dialect: DialectOpenAPIV1, BasePath: base, RoutesOnly: true, RequireParents: true,
		Scope: ScopeSpec{PathOrg: "org", PathProject: "project"},
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"GET " + base:    iacmDefaultPipelineList,
			"PUT " + base:    iacmDefaultPipelineUpsert,
			"DELETE " + base: iacmDefaultPipelineDelete,
		}}
}

type iacmDefaultPipelineBody struct {
	Provisioner string `json:"provisioner"`
	Operation   string `json:"operation"`
	Pipeline    string `json:"pipeline"`
}

func iacmDecode(s *Server, res Resource, w http.ResponseWriter, r *http.Request) (iacmDefaultPipelineBody, Key, bool) {
	var b iacmDefaultPipelineBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return b, Key{}, false
	}
	if b.Provisioner == "" || b.Operation == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "provisioner and operation are required"})
		return b, Key{}, false
	}
	key := res.scopeOf(r, nil)
	key.ID = b.Provisioner + "-" + b.Operation
	return b, key, true
}

func iacmDefaultPipelineList(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	out := []any{}
	for _, obj := range s.store.List(res.Kind, res.scopeOf(r, nil)) {
		out = append(out, obj)
	}
	writeJSON(w, http.StatusOK, out)
}

func iacmDefaultPipelineUpsert(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	b, key, ok := iacmDecode(s, res, w, r)
	if !ok {
		return
	}
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	obj := map[string]any{
		"account": s.account, "org": key.Org, "project": key.Project,
		"provisioner": b.Provisioner, "operation": b.Operation, "pipeline": b.Pipeline, "updated": 1700000000000,
	}
	created := !s.store.Has(res.Kind, key)
	s.store.Put(res.Kind, key, obj)
	if created {
		s.afterCreate(res.Kind, key)
	}
	writeJSON(w, http.StatusOK, obj)
}

func iacmDefaultPipelineDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	_, key, ok := iacmDecode(s, res, w, r)
	if !ok {
		return
	}
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
