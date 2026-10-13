package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindEnvironmentClusters is the store kind of an environment's linked GitOps clusters
// (EnvironmentClusters): one object per environment holding the linked cluster list.
const KindEnvironmentClusters = "environment_clusters"

// EnvironmentClusters is /ng/api/gitops/clusters: clusters are linked to an environment in
// batches (POST .../batch), unlinked in batches (POST .../batchunlink) and listed per
// environment in a nextgen page. The store key ID is the environment identifier; an
// environment with no linked cluster left is removed from the store, so the list answers
// empty, as the real API does.
func EnvironmentClusters() Resource {
	return Resource{Kind: KindEnvironmentClusters, BasePath: "/ng/api/gitops/clusters", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"GET /ng/api/gitops/clusters":              environmentClustersList,
			"POST /ng/api/gitops/clusters/batch":       environmentClustersLink,
			"POST /ng/api/gitops/clusters/batchunlink": environmentClustersUnlink,
		}}
}

type clusterBatch struct {
	OrgIdentifier     string           `json:"orgIdentifier"`
	ProjectIdentifier string           `json:"projectIdentifier"`
	EnvRef            string           `json:"envRef"`
	Clusters          []map[string]any `json:"clusters"`
}

func decodeClusterBatch(s *Server, res Resource, w http.ResponseWriter, r *http.Request) (clusterBatch, Key, bool) {
	var b clusterBatch
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return b, Key{}, false
	}
	if b.EnvRef == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "envRef is required"})
		return b, Key{}, false
	}
	return b, Key{Org: b.OrgIdentifier, Project: b.ProjectIdentifier, ID: b.EnvRef}, true
}

func (s *Server) clusterRecord(key Key, c map[string]any) map[string]any {
	return map[string]any{
		"clusterRef": str(c["identifier"]), "name": str(c["name"]), "agentIdentifier": str(c["agentIdentifier"]), "scope": str(c["scope"]),
		"envRef": key.ID, "orgIdentifier": key.Org, "projectIdentifier": key.Project, "accountIdentifier": s.account, "linkedAt": 1700000000000,
	}
}

func environmentClustersLink(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	b, key, ok := decodeClusterBatch(s, res, w, r)
	if !ok {
		return
	}
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		obj = map[string]any{"envRef": key.ID, "clusters": []any{}}
	}
	existing, _ := obj["clusters"].([]any)
	linked := 0
	for _, c := range b.Clusters {
		ref := str(c["identifier"])
		if ref == "" {
			continue
		}
		replaced := false
		for i, e := range existing {
			if m, _ := e.(map[string]any); m != nil && str(m["clusterRef"]) == ref {
				existing[i] = s.clusterRecord(key, c)
				replaced = true
			}
		}
		if !replaced {
			existing = append(existing, s.clusterRecord(key, c))
		}
		linked++
	}
	obj["clusters"] = existing
	s.store.Put(res.Kind, key, obj)
	if !exists {
		s.afterCreate(res.Kind, key)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": map[string]any{"linked": linked}, "correlationId": "fake"})
}

func environmentClustersUnlink(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	b, key, ok := decodeClusterBatch(s, res, w, r)
	if !ok {
		return
	}
	unlinked := 0
	if obj, exists := s.store.Get(res.Kind, key); exists {
		existing, _ := obj["clusters"].([]any)
		var kept []any
		for _, e := range existing {
			m, _ := e.(map[string]any)
			remove := false
			for _, c := range b.Clusters {
				if m != nil && str(m["clusterRef"]) == str(c["identifier"]) {
					remove = true
				}
			}
			if remove {
				unlinked++
			} else {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			s.store.Delete(res.Kind, key)
		} else {
			obj["clusters"] = kept
			s.store.Put(res.Kind, key, obj)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": map[string]any{"unlinked": unlinked}, "correlationId": "fake"})
}

func environmentClustersList(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier"), ID: q.Get("environmentIdentifier")}
	content := []any{}
	if !s.readGate(res.Kind, key) {
		if obj, ok := s.store.Get(res.Kind, key); ok {
			content, _ = obj["clusters"].([]any)
		}
	}
	n := len(content)
	writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS", "data": map[string]any{
		"content": content, "totalItems": n, "totalPages": 1, "pageItemCount": n, "pageSize": n, "pageIndex": 0, "empty": n == 0,
	}, "correlationId": "fake"})
}
