package actor

type actorState struct {
	revision int
	payload  any
}

func newActorState(initial any) actorState {
	return actorState{revision: 0, payload: initial}
}

func (s *actorState) apply(next any) {
	s.payload = next
	s.revision++
}

func (s *actorState) value() any {
	return s.payload
}
