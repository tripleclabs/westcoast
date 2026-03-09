package actor

import "sync"

type outcomeStore struct {
	mu              sync.Mutex
	byID            map[uint64]ProcessingOutcome
	lifecycle       []LifecycleHookOutcome
	guardrail       []GuardrailOutcome
	ask             []AskOutcome
	routing         []RoutingOutcome
	batch           []BatchOutcome
	readiness       []ReadinessValidationRecord
	readinessCursor int
}

const readinessHistoryLimit = 256

func newOutcomeStore() *outcomeStore {
	return &outcomeStore{
		byID:      map[uint64]ProcessingOutcome{},
		lifecycle: make([]LifecycleHookOutcome, 0, 32),
		guardrail: make([]GuardrailOutcome, 0, 64),
		ask:       make([]AskOutcome, 0, 128),
		routing:   make([]RoutingOutcome, 0, 256),
		batch:     make([]BatchOutcome, 0, 256),
		readiness: make([]ReadinessValidationRecord, 0, readinessHistoryLimit),
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
	if len(o.readiness) < readinessHistoryLimit {
		o.readiness = append(o.readiness, out)
		return
	}
	o.readiness[o.readinessCursor] = out
	o.readinessCursor = (o.readinessCursor + 1) % readinessHistoryLimit
}

func (o *outcomeStore) putAsk(out AskOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ask = append(o.ask, out)
}

func (o *outcomeStore) askByActor(actorID string) []AskOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]AskOutcome, 0, len(o.ask))
	for _, v := range o.ask {
		if v.ActorID == actorID {
			out = append(out, v)
		}
	}
	return out
}

func (o *outcomeStore) putRouting(out RoutingOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.routing = append(o.routing, out)
}

func (o *outcomeStore) routingByRouter(routerID string) []RoutingOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]RoutingOutcome, 0, len(o.routing))
	for _, v := range o.routing {
		if v.RouterID == routerID {
			out = append(out, v)
		}
	}
	return out
}

func (o *outcomeStore) putBatch(out BatchOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.batch = append(o.batch, out)
}

func (o *outcomeStore) batchByActor(actorID string) []BatchOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]BatchOutcome, 0, len(o.batch))
	for _, v := range o.batch {
		if v.ActorID == actorID {
			out = append(out, v)
		}
	}
	return out
}

func (o *outcomeStore) readinessAll() []ReadinessValidationRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]ReadinessValidationRecord, len(o.readiness))
	if len(o.readiness) < readinessHistoryLimit {
		copy(out, o.readiness)
		return out
	}
	n := copy(out, o.readiness[o.readinessCursor:])
	copy(out[n:], o.readiness[:o.readinessCursor])
	return out
}
