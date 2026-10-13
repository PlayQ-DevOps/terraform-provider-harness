package fakeharness

import (
	"encoding/json"
	"sort"
	"sync"
)

// Key identifies an object within a kind. Org and Project are empty for account-level
// objects; an organization's own Key has only ID set, a project's Key has Org and ID.
type Key struct {
	Org     string
	Project string
	ID      string
}

// Store is the fake's in-memory object store: kind -> Key -> decoded JSON object.
// Objects are stored exactly as the provider sent them (plus any Decorate fields), so a
// resource's Read gets back what its Create wrote without the fake knowing the schema.
type Store struct {
	mu   sync.Mutex
	objs map[string]map[Key]map[string]any
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{objs: map[string]map[Key]map[string]any{}}
}

func cloneObject(obj map[string]any) map[string]any {
	if obj == nil {
		return nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// Get returns a copy of the object, if present.
func (st *Store) Get(kind string, key Key) (map[string]any, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	obj, ok := st.objs[kind][key]
	return cloneObject(obj), ok
}

// Put stores a copy of obj under key, replacing any existing object.
func (st *Store) Put(kind string, key Key, obj map[string]any) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.objs[kind] == nil {
		st.objs[kind] = map[Key]map[string]any{}
	}
	st.objs[kind][key] = cloneObject(obj)
}

// Delete removes the object and reports whether it existed.
func (st *Store) Delete(kind string, key Key) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.objs[kind][key]; !ok {
		return false
	}
	delete(st.objs[kind], key)
	return true
}

// Has reports whether the object exists.
func (st *Store) Has(kind string, key Key) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	_, ok := st.objs[kind][key]
	return ok
}

// Mutate edits the stored object in place (simulating an out-of-band change) and reports
// whether it existed.
func (st *Store) Mutate(kind string, key Key, fn func(obj map[string]any)) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	obj, ok := st.objs[kind][key]
	if !ok {
		return false
	}
	fn(obj)
	return true
}

// List returns copies of the objects of kind whose Org and Project equal scope's, in a
// stable order by ID.
func (st *Store) List(kind string, scope Key) []map[string]any {
	st.mu.Lock()
	defer st.mu.Unlock()
	var keys []Key
	for k := range st.objs[kind] {
		if k.Org == scope.Org && k.Project == scope.Project {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, cloneObject(st.objs[kind][k]))
	}
	return out
}

// Keys returns every key of kind, sorted.
func (st *Store) Keys(kind string) []Key {
	st.mu.Lock()
	defer st.mu.Unlock()
	keys := make([]Key, 0, len(st.objs[kind]))
	for k := range st.objs[kind] {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Org != keys[j].Org {
			return keys[i].Org < keys[j].Org
		}
		if keys[i].Project != keys[j].Project {
			return keys[i].Project < keys[j].Project
		}
		return keys[i].ID < keys[j].ID
	})
	return keys
}

// Len returns the number of objects of kind.
func (st *Store) Len(kind string) int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.objs[kind])
}

func (st *Store) reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.objs = map[string]map[Key]map[string]any{}
}
