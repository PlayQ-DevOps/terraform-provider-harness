package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindGitopsRepoCert is the store kind of an agent's repository certificate list
// (GitopsRepoCertificates).
const KindGitopsRepoCert = "gitops_repo_cert"

// GitopsRepoCertificates is /gitops/api/v1/agents/{agent}/certificates: an agent holds one
// certificate list, created or upserted with a POST, listed with a GET that answers an empty
// list (not 404) when the agent has none, and trimmed with a DELETE filtered by host name
// pattern. The store key ID is the agent identifier.
func GitopsRepoCertificates() Resource {
	base := "/gitops/api/v1/agents/{agent}/certificates"
	return Resource{Kind: KindGitopsRepoCert, Dialect: DialectOpenAPIV1, BasePath: base, RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"GET " + base:    repoCertList,
			"POST " + base:   repoCertCreate,
			"DELETE " + base: repoCertDelete,
		}}
}

func repoCertKey(r *http.Request) Key {
	q := r.URL.Query()
	return Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier"), ID: r.PathValue("agent")}
}

func repoCertCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Certificates map[string]any `json:"certificates"`
		Upsert       bool           `json:"upsert"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return
	}
	key := repoCertKey(r)
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	items, _ := body.Certificates["items"].([]any)
	if len(items) == 0 {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "certificates.items is required"})
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		obj = map[string]any{"metadata": map[string]any{}, "items": []any{}}
	}
	existing, _ := obj["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		replaced := false
		for i, e := range existing {
			if em, _ := e.(map[string]any); em != nil && str(em["serverName"]) == str(m["serverName"]) && str(em["certType"]) == str(m["certType"]) {
				if body.Upsert {
					existing[i] = m
				}
				replaced = true
			}
		}
		if !replaced {
			existing = append(existing, m)
		}
	}
	obj["items"] = existing
	if md, _ := body.Certificates["metadata"].(map[string]any); md != nil {
		obj["metadata"] = md
	}
	s.store.Put(res.Kind, key, obj)
	if !exists {
		s.afterCreate(res.Kind, key)
	}
	writeJSON(w, http.StatusOK, obj)
}

func repoCertList(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := repoCertKey(r)
	if s.readGate(res.Kind, key) {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func repoCertDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := repoCertKey(r)
	pattern := r.URL.Query().Get("query.hostNamePattern")
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	items, _ := obj["items"].([]any)
	kept := []any{}
	for _, it := range items {
		if m, _ := it.(map[string]any); m != nil && pattern != "" && str(m["serverName"]) != pattern {
			kept = append(kept, it)
		}
	}
	if len(kept) == 0 {
		s.store.Delete(res.Kind, key)
	} else {
		obj["items"] = kept
		s.store.Put(res.Kind, key, obj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"metadata": obj["metadata"], "items": kept})
}
