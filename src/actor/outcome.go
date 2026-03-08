package actor

import "sync"

type outcomeStore struct {
	mu   sync.Mutex
	byID map[uint64]ProcessingOutcome
}

func newOutcomeStore() *outcomeStore {
	return &outcomeStore{byID: map[uint64]ProcessingOutcome{}}
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
