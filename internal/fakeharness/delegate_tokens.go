package fakeharness

import (
	"net/http"
	"strconv"
)

// KindDelegateToken is the store kind of delegate tokens (DelegateTokens).
const KindDelegateToken = "delegate_token"

// DelegateTokens is /ng/api/delegate-token-ng. Tokens are addressed by name through query
// parameters (tokenName on writes, name on reads), responses use the RestResponse envelope
// {"resource": ...}, and a revoke is a PUT. The real API keeps a revoked token listed for 30
// days; the fake purges it at once so a destroyed token leaves the store.
func DelegateTokens() Resource {
	return Resource{Kind: KindDelegateToken, BasePath: "/ng/api/delegate-token-ng", RoutesOnly: true, RequireParents: true,
		Routes: map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){
			"GET /ng/api/delegate-token-ng":  delegateTokenList,
			"POST /ng/api/delegate-token-ng": delegateTokenCreate,
			"PUT /ng/api/delegate-token-ng":  delegateTokenRevoke,
		}}
}

func restResponse(w http.ResponseWriter, status int, resource any) {
	writeJSON(w, status, map[string]any{"metaData": map[string]any{}, "resource": resource, "responseMessages": []any{}})
}

func restError(w http.ResponseWriter, e APIError) {
	writeJSON(w, e.Status, map[string]any{"metaData": map[string]any{}, "resource": nil,
		"responseMessages": []any{map[string]any{"code": e.Code, "level": "ERROR", "message": e.Message}}})
}

func delegateTokenKey(r *http.Request, nameParam string) Key {
	q := r.URL.Query()
	return Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier"), ID: q.Get(nameParam)}
}

func delegateTokenList(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := Key{Org: q.Get("orgIdentifier"), Project: q.Get("projectIdentifier")}
	name, status := q.Get("name"), q.Get("status")
	out := []any{}
	for _, obj := range s.store.List(res.Kind, scope) {
		if (name == "" || str(obj["name"]) == name) && (status == "" || str(obj["status"]) == status) {
			out = append(out, obj)
		}
	}
	restResponse(w, http.StatusOK, out)
}

func delegateTokenCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := delegateTokenKey(r, "tokenName")
	if key.ID == "" {
		restError(w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "tokenName is required"})
		return
	}
	if e := s.parentsExist(res, key); e != nil {
		restError(w, *e)
		return
	}
	if s.store.Has(res.Kind, key) {
		restError(w, APIError{Status: http.StatusBadRequest, Code: "DUPLICATE_FIELD", Message: "delegate token " + key.ID + " already exists"})
		return
	}
	obj := map[string]any{
		"uuid": "tok-" + key.ID, "accountId": s.account, "name": key.ID, "status": "ACTIVE",
		"value": "ZmFrZS1kZWxlZ2F0ZS10b2tlbg==", "createdAt": 1700000000000,
		"createdByNgUser": map[string]any{"type": "USER", "name": "fake-user"},
	}
	if ra, err := strconv.ParseInt(r.URL.Query().Get("revokeAfter"), 10, 64); err == nil {
		obj["revokeAfter"] = ra
	}
	s.store.Put(res.Kind, key, obj)
	s.afterCreate(res.Kind, key)
	restResponse(w, http.StatusOK, obj)
}

func delegateTokenRevoke(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := delegateTokenKey(r, "tokenName")
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		s.writeNotFound(res, w, key.ID)
		return
	}
	obj["status"] = "REVOKED"
	s.store.Delete(res.Kind, key)
	restResponse(w, http.StatusOK, obj)
}
