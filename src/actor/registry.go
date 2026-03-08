package actor

import "sync"

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
