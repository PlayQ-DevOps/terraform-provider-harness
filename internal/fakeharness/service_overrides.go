package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindServiceOverride is the store kind of environment service overrides (ServiceOverrides).
const KindServiceOverride = "service_override"

// ServiceOverrides is /ng/api/environmentsV2/serviceOverrides, the v1 override API: one
// override per service and environment, written by an upsert whose body carries the scope,
// listed by environment (optionally filtered by service) in a nextgen page, and deleted by
// query parameters. The store key is ServiceOverrideKey.
func ServiceOverrides() Resource {
	return Resource{Kind: KindServiceOverride, BasePath: "/ng/api/environmentsV2/serviceOverrides", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"GET /ng/api/environmentsV2/serviceOverrides":    serviceOverrideList,
			"POST /ng/api/environmentsV2/serviceOverrides":   serviceOverrideUpsert,
			"DELETE /ng/api/environmentsV2/serviceOverrides": serviceOverrideDelete,
		}}
}

// ServiceOverrideKey is the store key of the override of service in env.
func ServiceOverrideKey(org, project, env, service string) Key {
	return Key{Org: org, Project: project, ID: env + "/" + service}
}

func serviceOverrideUpsert(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrgIdentifier         string `json:"orgIdentifier"`
		ProjectIdentifier     string `json:"projectIdentifier"`
		EnvironmentIdentifier string `json:"environmentIdentifier"`
		ServiceIdentifier     string `json:"serviceIdentifier"`
		Yaml                  string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return
	}
	if body.EnvironmentIdentifier == "" || body.ServiceIdentifier == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "environmentIdentifier and serviceIdentifier are required"})
		return
	}
	key := ServiceOverrideKey(body.OrgIdentifier, body.ProjectIdentifier, body.EnvironmentIdentifier, body.ServiceIdentifier)
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	obj := map[string]any{
		"accountId": s.account, "orgIdentifier": body.OrgIdentifier, "projectIdentifier": body.ProjectIdentifier,
		"environmentRef": body.EnvironmentIdentifier, "serviceRef": body.ServiceIdentifier, "yaml": body.Yaml,
	}
	created := !s.store.Has(res.Kind, key)
	s.store.Put(res.Kind, key, obj)
	if created {
		s.afterCreate(res.Kind, key)
	}
	s.writeObject(res, w, http.StatusOK, obj)
}

func serviceOverrideList(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier")}
	env, service := q.Get("environmentIdentifier"), q.Get("serviceIdentifier")
	content := []any{}
	for _, obj := range s.store.List(res.Kind, scope) {
		if str(obj["environmentRef"]) != env || (service != "" && str(obj["serviceRef"]) != service) {
			continue
		}
		content = append(content, obj)
	}
	n := len(content)
	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": map[string]any{
		"content": content, "totalItems": n, "totalPages": 1, "pageItemCount": n, "pageSize": n, "pageIndex": 0, "empty": n == 0,
	}, "correlationId": "fake"})
}

func serviceOverrideDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := ServiceOverrideKey(q.Get("orgIdentifier"), q.Get("projectIdentifier"), q.Get("environmentIdentifier"), q.Get("serviceIdentifier"))
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": true, "correlationId": "fake"})
}
