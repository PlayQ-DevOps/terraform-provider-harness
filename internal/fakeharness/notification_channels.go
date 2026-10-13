package fakeharness

import (
	"encoding/json"
	"net/http"
)

// KindNotificationChannel is the store kind of central notification channels
// (NotificationChannels).
const KindNotificationChannel = "notification_channel"

// NotificationChannels is the central notification API, which has one base path per scope:
// /v1/notification-channels, /v1/orgs/{org}/notification-channels and
// /v1/orgs/{org}/projects/{project}/notification-channels. Bodies and responses are the
// channel DTO; a delete answers 204.
func NotificationChannels() Resource {
	res := Resource{Kind: KindNotificationChannel, Dialect: DialectOpenAPIV1, BasePath: "/v1/notification-channels", RoutesOnly: true, RequireParents: true,
		KindPattern: `^/v1(/orgs/[^/]+(/projects/[^/]+)?)?/notification-channels(/|$)`,
		Routes:      map[string]func(*Server, Resource, http.ResponseWriter, *http.Request){}}
	for _, base := range []string{"/v1/notification-channels", "/v1/orgs/{org}/notification-channels", "/v1/orgs/{org}/projects/{project}/notification-channels"} {
		res.Routes["POST "+base] = notificationChannelCreate
		res.Routes["GET "+base+"/{fakeid}"] = notificationChannelGet
		res.Routes["PUT "+base+"/{fakeid}"] = notificationChannelUpdate
		res.Routes["DELETE "+base+"/{fakeid}"] = notificationChannelDelete
	}
	return res
}

func notificationChannelKey(r *http.Request) Key {
	return Key{Org: r.PathValue("org"), Project: r.PathValue("project"), ID: r.PathValue("fakeid")}
}

func notificationChannelDecode(s *Server, res Resource, w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "invalid JSON body: " + err.Error()})
		return nil, false
	}
	return body, true
}

func notificationChannelCreate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	body, ok := notificationChannelDecode(s, res, w, r)
	if !ok {
		return
	}
	key := notificationChannelKey(r)
	key.ID = str(body["identifier"])
	if key.ID == "" {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "identifier is required"})
		return
	}
	if e := s.parentsExist(res, key); e != nil {
		s.writeError(res, w, *e)
		return
	}
	if s.store.Has(res.Kind, key) {
		s.writeError(res, w, APIError{Status: http.StatusBadRequest, Code: "DUPLICATE_FIELD", Message: "notification channel " + key.ID + " already exists"})
		return
	}
	body["account"] = s.account
	body["created"] = 1700000000000
	body["last_modified"] = 1700000000000
	s.store.Put(res.Kind, key, body)
	s.afterCreate(res.Kind, key)
	writeJSON(w, http.StatusOK, body)
}

func notificationChannelGet(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := notificationChannelKey(r)
	if s.readGate(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	obj, ok := s.store.Get(res.Kind, key)
	if !ok {
		s.writeNotFound(res, w, key.ID)
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func notificationChannelUpdate(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := notificationChannelKey(r)
	body, ok := notificationChannelDecode(s, res, w, r)
	if !ok {
		return
	}
	obj, exists := s.store.Get(res.Kind, key)
	if !exists {
		s.writeNotFound(res, w, key.ID)
		return
	}
	for k, v := range body {
		if k != "identifier" && k != "created" && k != "account" {
			obj[k] = v
		}
	}
	obj["last_modified"] = 1700000001000
	s.store.Put(res.Kind, key, obj)
	writeJSON(w, http.StatusOK, obj)
}

func notificationChannelDelete(s *Server, res Resource, w http.ResponseWriter, r *http.Request) {
	key := notificationChannelKey(r)
	if !s.store.Delete(res.Kind, key) {
		s.writeNotFound(res, w, key.ID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
