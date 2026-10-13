package fakeharness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// KindIDPEntity is the store kind of IDP catalog entities (IDPEntities).
const KindIDPEntity = "idp_entity"

// IDPEntities is /v1/entities, the IDP catalog API. It does not follow the generic CRUD
// shape, so it is served by its own routes: a create carries the entity YAML (whose
// top-level kind, identifier, orgIdentifier and projectIdentifier name the object) beside
// optional git_details; objects are addressed as /{scope}/{kind}/{identifier} with the org
// and project repeated as query parameters; responses are the plain entity object; a delete
// answers 204.
func IDPEntities() Resource {
	return Resource{Kind: KindIDPEntity, Dialect: DialectOpenAPIV1, BasePath: "/v1/entities", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"POST /v1/entities":                                     idpCreateEntity,
			"GET /v1/entities/{scope}/{kind}/{fakeid}":              idpGetEntity,
			"PUT /v1/entities/{scope}/{kind}/{fakeid}":              idpUpdateEntity,
			"DELETE /v1/entities/{scope}/{kind}/{fakeid}":           idpDeleteEntity,
			"PUT /v1/entities/git-metadata/{scope}/{kind}/{fakeid}": idpUpdateGitMetadata,
		}}
}

type idpWriteBody struct {
	Yaml       string         `json:"yaml"`
	GitDetails map[string]any `json:"git_details"`
}

func idpError(w http.ResponseWriter, e APIError) {
	writeJSON(w, e.Status, map[string]any{"code": e.Code, "message": e.Message})
}

func idpDecode(w http.ResponseWriter, r *http.Request) (idpWriteBody, bool) {
	var body idpWriteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		idpError(w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return body, false
	}
	return body, true
}

// idpKey derives the store key of an addressed entity from the query parameters, falling
// back to the scope path segment ("account", "account.<org>" or "account.<org>.<project>").
func idpKey(r *http.Request) Key {
	q := r.URL.Query()
	k := Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier"), ID: r.PathValue("fakeid")}
	if k.Org == "" {
		parts := strings.Split(r.PathValue("scope"), ".")
		if len(parts) > 1 {
			k.Org = parts[1]
		}
		if len(parts) > 2 {
			k.Project = parts[2]
		}
	}
	return k
}

func idpScope(k Key) string {
	scope := "account"
	if k.Org != "" {
		scope += "." + k.Org
	}
	if k.Project != "" {
		scope += "." + k.Project
	}
	return scope
}

func idpCreateEntity(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	body, ok := idpDecode(w, r)
	if !ok {
		return
	}
	top := yamlTopLevelScalars(body.Yaml)
	id, kind := top["identifier"], strings.ToLower(top["kind"])
	if id == "" || kind == "" {
		idpError(w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "entity YAML must carry kind and identifier"})
		return
	}
	q := r.URL.Query()
	key := Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier"), ID: id}
	if key.Org == "" {
		key.Org = top["orgIdentifier"]
	}
	if key.Project == "" {
		key.Project = top["projectIdentifier"]
	}
	if e := s.parentsExist(res, key); e != nil {
		idpError(w, *e)
		return
	}
	if s.store.Has(res.Kind, key) {
		idpError(w, APIError{Status: http.StatusBadRequest, Code: "DUPLICATE_FIELD", Message: fmt.Sprintf("entity %s/%s already exists in scope %s", kind, id, idpScope(key))})
		return
	}
	obj := map[string]any{
		"identifier": id, "kind": kind, "name": top["name"], "yaml": body.Yaml,
		"scope": idpScope(key), "entity_ref": idpScope(key) + "/" + kind + ":" + id,
		"referenceType": "INLINE", "created": 1700000000000,
	}
	if key.Org != "" {
		obj["orgIdentifier"] = key.Org
	}
	if key.Project != "" {
		obj["projectIdentifier"] = key.Project
	}
	if len(body.GitDetails) > 0 {
		obj["git_details"] = body.GitDetails
		obj["referenceType"] = "REMOTE"
	}
	s.store.Put(res.Kind, key, obj)
	s.afterCreate(res.Kind, key)
	writeJSON(w, http.StatusCreated, obj)
}

func idpGetEntity(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := idpKey(r)
	if s.readGate(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	obj, ok := s.store.Get(res.Kind, key)
	if !ok || str(obj["kind"]) != strings.ToLower(r.PathValue("kind")) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func idpUpdateEntity(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := idpKey(r)
	body, ok := idpDecode(w, r)
	if !ok {
		return
	}
	if !s.store.Has(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	top := yamlTopLevelScalars(body.Yaml)
	s.store.Mutate(res.Kind, key, func(obj map[string]any) {
		obj["yaml"] = body.Yaml
		if name := top["name"]; name != "" {
			obj["name"] = name
		}
		if len(body.GitDetails) > 0 {
			gd, _ := obj["git_details"].(map[string]any)
			if gd == nil {
				gd = map[string]any{}
			}
			for k, v := range body.GitDetails {
				gd[k] = v
			}
			obj["git_details"] = gd
		}
	})
	obj, _ := s.store.Get(res.Kind, key)
	writeJSON(w, http.StatusOK, obj)
}

func idpDeleteEntity(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := idpKey(r)
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func idpUpdateGitMetadata(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := idpKey(r)
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		idpError(w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return
	}
	if !s.store.Has(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
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
	writeJSON(w, http.StatusOK, map[string]any{})
}

// yamlTopLevelScalars returns the unindented "key: value" scalars of a YAML document: enough
// to find an entity's kind, identifier, name and scope without a YAML dependency.
func yamlTopLevelScalars(doc string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(doc, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		out[strings.TrimSpace(k)] = strings.Trim(v, `"'`)
	}
	return out
}
