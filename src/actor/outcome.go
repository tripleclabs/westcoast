package actor

import "sync"

type outcomeStore struct {
	mu        sync.Mutex
	byID      map[uint64]ProcessingOutcome
	lifecycle []LifecycleHookOutcome
}

func newOutcomeStore() *outcomeStore {
	return &outcomeStore{
		byID:      map[uint64]ProcessingOutcome{},
		lifecycle: make([]LifecycleHookOutcome, 0, 32),
	}
}

func (o *outcomeStore) put(out ProcessingOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byID[out.MessageID] = out
}

func (o *outcomeStore) get(msgID uint64) (ProcessingOutcome, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out, ok := o.byID[msgID]
	return out, ok
}

func (o *outcomeStore) putLifecycle(out LifecycleHookOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.lifecycle = append(o.lifecycle, out)
}

func (o *outcomeStore) lifecycleByActor(actorID string) []LifecycleHookOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]LifecycleHookOutcome, 0, len(o.lifecycle))
	for _, v := range o.lifecycle {
		if v.ActorID == actorID {
			out = append(out, v)
		}
	}
	return out
}
