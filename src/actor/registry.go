package actor

import (
	"sync"
	"time"
)

type actorRegistry struct {
	mu     sync.RWMutex
	actors map[string]*actorInstance
}

func newRegistry() *actorRegistry {
	return &actorRegistry{actors: map[string]*actorInstance{}}
}

func (r *actorRegistry) put(id string, a *actorInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.actors[id]; ok {
		return ErrDuplicateActorID
	}
	r.actors[id] = a
	return nil
}

func (r *actorRegistry) get(id string) (*actorInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.actors[id]
	return a, ok
}

func (r *actorRegistry) remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.actors[id]; !ok {
		return false
	}
	delete(r.actors, id)
	return true
}

type registryEntry struct {
	name    string
	actorID string
	pid     PID
	updated time.Time
}

type namedRegistry struct {
	mu      sync.RWMutex
	byName  map[string]registryEntry
	byActor map[string]map[string]struct{}
	now     func() time.Time
}

func newNamedRegistry(now func() time.Time) *namedRegistry {
	return &namedRegistry{
		byName:  map[string]registryEntry{},
		byActor: map[string]map[string]struct{}{},
		now:     now,
	}
}

func (r *namedRegistry) register(name, actorID string, pid PID) (registryEntry, error) {
	if err := validateRegistryName(name); err != nil {
		return registryEntry{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byName[name]; ok {
		return registryEntry{}, ErrRegistryDuplicateName
	}
	entry := registryEntry{name: name, actorID: actorID, pid: pid, updated: r.now()}
	r.byName[name] = entry
	if _, ok := r.byActor[actorID]; !ok {
		r.byActor[actorID] = map[string]struct{}{}
	}
	r.byActor[actorID][name] = struct{}{}
	return entry, nil
}

func (r *namedRegistry) lookup(name string) (registryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.byName[name]
	return entry, ok
}

func (r *namedRegistry) unregister(name string) (registryEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.byName[name]
	if !ok {
		return registryEntry{}, false
	}
	delete(r.byName, name)
	if names, ok := r.byActor[entry.actorID]; ok {
		delete(names, name)
		if len(names) == 0 {
			delete(r.byActor, entry.actorID)
		}
	}
	return entry, true
}

func (r *namedRegistry) unregisterByActor(actorID string) []registryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	names, ok := r.byActor[actorID]
	if !ok {
		return nil
	}
	out := make([]registryEntry, 0, len(names))
	for name := range names {
		entry, ok := r.byName[name]
		if !ok {
			continue
		}
		out = append(out, entry)
		delete(r.byName, name)
	}
	delete(r.byActor, actorID)
	return out
}
