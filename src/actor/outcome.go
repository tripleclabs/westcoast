package actor

import "sync"

type outcomeStore struct {
	mu        sync.Mutex
	byID      map[uint64]ProcessingOutcome
	lifecycle []LifecycleHookOutcome
	guardrail []GuardrailOutcome
	readiness []ReadinessValidationRecord
}

func newOutcomeStore() *outcomeStore {
	return &outcomeStore{
		byID:      map[uint64]ProcessingOutcome{},
		lifecycle: make([]LifecycleHookOutcome, 0, 32),
		guardrail: make([]GuardrailOutcome, 0, 64),
		readiness: make([]ReadinessValidationRecord, 0, 16),
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

func (o *outcomeStore) putGuardrail(out GuardrailOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.guardrail = append(o.guardrail, out)
}

func (o *outcomeStore) guardrailByActor(actorID string) []GuardrailOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]GuardrailOutcome, 0, len(o.guardrail))
	for _, v := range o.guardrail {
		if v.ActorID == actorID {
			out = append(out, v)
		}
	}
	return out
}

func (o *outcomeStore) putReadiness(out ReadinessValidationRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.readiness = append(o.readiness, out)
}

func (o *outcomeStore) readinessAll() []ReadinessValidationRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]ReadinessValidationRecord, len(o.readiness))
	copy(out, o.readiness)
	return out
}
