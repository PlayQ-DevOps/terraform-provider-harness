package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindChaosInfrastructure is the store kind of chaos infrastructures (ChaosInfrastructures).
const KindChaosInfrastructure = "chaos_infrastructure"

// ChaosInfrastructures is the chaos manager's /rest/v2/infrastructure, mounted under the
// /chaos/manager/api prefix the provider adds to its endpoint. An infrastructure is
// registered with a POST whose body carries its identity and environment, fetched by
// identity with the environment as a query parameter (a wrong environment is not found),
// updated with a PUT to the collection and deleted at /{environmentIdentifier}/{identity}.
// The store key ID is the identity.
func ChaosInfrastructures() Resource {
	base := "/chaos/manager/api/rest/v2/infrastructure"
	return Resource{Kind: KindChaosInfrastructure, Dialect: DialectOpenAPIV1, BasePath: base, RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"POST " + base:                       chaosInfraRegister,
			"GET " + base + "/{fakeid}":          chaosInfraGet,
			"PUT " + base:                        chaosInfraUpdate,
			"DELETE " + base + "/{env}/{fakeid}": chaosInfraDelete,
		}}
}

func chaosScope(r *http.Request) Key {
	q := r.URL.Query()
	return Key{Org: q.Get("organizationIdentifier"), Project: q.Get("projectIdentifier")}
}

func chaosDecode(s *Server, res Resource, w http.ResponseWriter, r *http.Request) (map[string]any, Key, bool) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return nil, Key{}, false
	}
	identity := str(body["identity"])
	if identity == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "identity is required"})
		return nil, Key{}, false
	}
	key := chaosScope(r)
	key.ID = identity
	return body, key, true
}

func chaosInfraRegister(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	body, key, ok := chaosDecode(s, res, w, r)
	if !ok {
		return
	}
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	if s.store.Has(res.Kind, key) {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "DUPLICATE_FIELD", Message: "infrastructure " + key.ID + " already exists"})
		return
	}
	if _, ok := body["identifier"]; !ok {
		body["identifier"] = map[string]any{"accountIdentifier": s.account, "orgIdentifier": key.Org, "projectIdentifier": key.Project}
	}
	if str(body["infraID"]) == "" {
		body["infraID"] = key.ID
	}
	body["status"] = "ACTIVE"
	body["createdAt"] = "1700000000"
	s.store.Put(res.Kind, key, body)
	s.afterCreate(res.Kind, key)
	writeJSON(w, http.StatusOK, map[string]any{"identity": key.ID, "name": str(body["name"]), "token": "fake-infra-token"})
}

func chaosInfraGet(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := chaosScope(r)
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
	if env := r.URL.Query().Get("environmentIdentifier"); env != "" && env != str(obj["environmentID"]) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func chaosInfraUpdate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	body, key, ok := chaosDecode(s, res, w, r)
	if !ok {
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		s.writeNotFound(res, w, key.ID)
		return
	}
	for k, v := range body {
		if k != "identity" && k != "identifier" {
			obj[k] = v
		}
	}
	s.store.Put(res.Kind, key, obj)
	writeJSON(w, http.StatusOK, obj)
}

func chaosInfraDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := chaosScope(r)
	key.ID = r.PathValue("fakeid")
	obj, ok := s.store.Get(res.Kind, key)
	if !ok || str(obj["environmentID"]) != r.PathValue("env") {
		s.writeNotFound(res, w, key.ID)
		return
	}
	s.store.Delete(res.Kind, key)
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}
