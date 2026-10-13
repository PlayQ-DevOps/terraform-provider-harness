package fakeharness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Kinds of the feature flag (CF) objects.
const (
	KindFeatureFlagTarget      = "ff_target"
	KindFeatureFlagTargetGroup = "ff_target_group"
)

// CFKey is the store key of a feature flag target or target group, which live in an
// environment of a project.
func CFKey(org, project, environment, identifier string) Key {
	return Key{Org: org, Project: project, ID: environment + "/" + identifier}
}

// FeatureFlagTargets is /cf/admin/targets: plain bodies, the project and environment inside
// the Target object on create and in query parameters otherwise, 201 on create, 204 on
// delete, errors as {"code":"<status>","message":...}.
func FeatureFlagTargets() Resource {
	return Resource{Kind: KindFeatureFlagTarget, Dialect: DialectOpenAPIV1, BasePath: "/cf/admin/targets", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"POST /cf/admin/targets":            cfTargetCreate,
			"GET /cf/admin/targets/{fakeid}":    cfTargetGet,
			"PUT /cf/admin/targets/{fakeid}":    cfTargetModify,
			"DELETE /cf/admin/targets/{fakeid}": cfTargetDelete,
		}}
}

// FeatureFlagTargetGroups is /cf/admin/segments: created from a request whose included and
// excluded lists are target identifiers, read back as a Segment whose lists hold Target
// objects, and updated with PATCH instructions.
func FeatureFlagTargetGroups() Resource {
	return Resource{Kind: KindFeatureFlagTargetGroup, Dialect: DialectOpenAPIV1, BasePath: "/cf/admin/segments", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"POST /cf/admin/segments":            cfSegmentCreate,
			"GET /cf/admin/segments/{fakeid}":    cfSegmentGet,
			"PATCH /cf/admin/segments/{fakeid}":  cfSegmentPatch,
			"DELETE /cf/admin/segments/{fakeid}": cfSegmentDelete,
		}}
}

func cfError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"code": strconv.Itoa(status), "message": message})
}

func cfKeyFromQuery(r *http.Request) Key {
	q := r.URL.Query()
	return CFKey(q.Get("orgIdentifier"), q.Get("projectIdentifier"), q.Get("environmentIdentifier"), r.PathValue("fakeid"))
}

func cfDecode(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		cfError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return nil, false
	}
	return body, true
}

func cfCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request, what string, decorate func(key Key, body map[string]any) map[string]any) {
	body, ok := cfDecode(w, r)
	if !ok {
		return
	}
	id, env := str(body["identifier"]), str(body["environment"])
	if id == "" || env == "" {
		cfError(w, http.StatusBadRequest, what+" needs an identifier and an environment")
		return
	}
	key := CFKey(r.URL.Query().Get("orgIdentifier"), str(body["project"]), env, id)
	if e := s.parentsExist(res, key); e != nil {
		cfError(w, e.Status, e.Message)
		return
	}
	if s.store.Has(res.Kind, key) {
		cfError(w, http.StatusConflict, fmt.Sprintf("%s %s already exists in environment %s", what, id, env))
		return
	}
	obj := decorate(key, body)
	s.store.Put(res.Kind, key, obj)
	s.afterCreate(res.Kind, key)
	writeJSON(w, http.StatusCreated, obj)
}

func cfGet(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := cfKeyFromQuery(r)
	if s.readGate(res.Kind, key) {
		s.writeNotFound(res, w, r.PathValue("fakeid"))
		return
	}
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		s.writeNotFound(res, w, r.PathValue("fakeid"))
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func cfDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := cfKeyFromQuery(r)
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, r.PathValue("fakeid"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func cfTargetCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfCreate(s, res, w, r, "target", func(key Key, body map[string]any) map[string]any {
		if str(body["account"]) == "" {
			body["account"] = s.account
		}
		if _, ok := body["createdAt"]; !ok {
			body["createdAt"] = 1700000000
		}
		return body
	})
}

func cfTargetGet(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfGet(s, res, w, r)
}

func cfTargetModify(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := cfKeyFromQuery(r)
	body, ok := cfDecode(w, r)
	if !ok {
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		s.writeNotFound(res, w, r.PathValue("fakeid"))
		return
	}
	for k, v := range body {
		if k != "identifier" && k != "environment" && k != "project" && k != "org" {
			obj[k] = v
		}
	}
	s.store.Put(res.Kind, key, obj)
	writeJSON(w, http.StatusOK, obj)
}

func cfTargetDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfDelete(s, res, w, r)
}

func (s *Server) cfTargetRef(key Key, id string) map[string]any {
	return map[string]any{"identifier": id, "name": id, "account": s.account, "org": key.Org, "project": key.Project, "environment": segmentEnv(key)}
}

// segmentEnv recovers the environment from a CFKey ID ("<environment>/<identifier>").
func segmentEnv(key Key) string {
	for i := 0; i < len(key.ID); i++ {
		if key.ID[i] == '/' {
			return key.ID[:i]
		}
	}
	return ""
}

func cfSegmentCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfCreate(s, res, w, r, "target group", func(key Key, body map[string]any) map[string]any {
		obj := map[string]any{
			"identifier": str(body["identifier"]), "name": str(body["name"]), "environment": str(body["environment"]),
			"included": []any{}, "excluded": []any{}, "rules": []any{}, "createdAt": 1700000000, "modifiedAt": 1700000000, "version": 1,
		}
		for _, list := range []string{"included", "excluded"} {
			ids, _ := body[list].([]any)
			refs := []any{}
			for _, id := range ids {
				refs = append(refs, s.cfTargetRef(key, str(id)))
			}
			obj[list] = refs
		}
		if rules, _ := body["rules"].([]any); rules != nil {
			out := []any{}
			for i, rule := range rules {
				if m, _ := rule.(map[string]any); m != nil {
					if str(m["id"]) == "" {
						m["id"] = fmt.Sprintf("rule-%d", i+1)
					}
					out = append(out, m)
				}
			}
			obj["rules"] = out
		}
		return obj
	})
}

func cfSegmentGet(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfGet(s, res, w, r)
}

// cfSegmentPatch applies the instruction kinds the provider emits: updateName,
// addToIncludeList, removeFromIncludeList, addToExcludeList, removeFromExcludeList,
// addClause and removeClause.
func cfSegmentPatch(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := cfKeyFromQuery(r)
	body, ok := cfDecode(w, r)
	if !ok {
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		s.writeNotFound(res, w, r.PathValue("fakeid"))
		return
	}
	instructions, _ := body["instructions"].([]any)
	for _, in := range instructions {
		m, _ := in.(map[string]any)
		if m == nil {
			continue
		}
		params, _ := m["parameters"].(map[string]any)
		targets, _ := params["targets"].([]any)
		switch str(m["kind"]) {
		case "updateName":
			obj["name"] = str(params["name"])
		case "addToIncludeList", "addToExcludeList":
			list := "included"
			if str(m["kind"]) == "addToExcludeList" {
				list = "excluded"
			}
			refs, _ := obj[list].([]any)
			for _, id := range targets {
				refs = append(refs, s.cfTargetRef(key, str(id)))
			}
			obj[list] = refs
		case "removeFromIncludeList", "removeFromExcludeList":
			list := "included"
			if str(m["kind"]) == "removeFromExcludeList" {
				list = "excluded"
			}
			refs, _ := obj[list].([]any)
			kept := []any{}
			for _, ref := range refs {
				rm, _ := ref.(map[string]any)
				drop := false
				for _, id := range targets {
					if rm != nil && str(rm["identifier"]) == str(id) {
						drop = true
					}
				}
				if !drop {
					kept = append(kept, ref)
				}
			}
			obj[list] = kept
		case "addClause":
			rules, _ := obj["rules"].([]any)
			clause := map[string]any{"id": fmt.Sprintf("rule-%d", len(rules)+1), "attribute": params["attribute"], "op": params["op"], "negate": params["negate"], "values": params["values"]}
			obj["rules"] = append(rules, clause)
		case "removeClause":
			rules, _ := obj["rules"].([]any)
			kept := []any{}
			for _, rule := range rules {
				if rm, _ := rule.(map[string]any); rm == nil || str(rm["id"]) != str(params["ruleID"]) {
					kept = append(kept, rule)
				}
			}
			obj["rules"] = kept
		}
	}
	if v, ok := obj["version"].(float64); ok {
		obj["version"] = v + 1
	}
	s.store.Put(res.Kind, key, obj)
	writeJSON(w, http.StatusOK, obj)
}

func cfSegmentDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	cfDelete(s, res, w, r)
}
