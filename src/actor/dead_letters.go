package actor

import (
	"sync"
	"time"
)

// DeadLetter represents a message that could not be delivered.
type DeadLetter struct {
	TargetActorID string
	TargetPID     PID
	Payload       any
	Reason        string // e.g. "actor_not_found", "mailbox_full", "stale_generation"
	Timestamp     time.Time
}

// DeadLetterHandler is called when a message cannot be delivered.
type DeadLetterHandler func(letter DeadLetter)

// deadLetterSink collects dead letters and optionally forwards to a handler.
type deadLetterSink struct {
	mu      sync.RWMutex
	handler DeadLetterHandler
	count   uint64
}

func newDeadLetterSink() *deadLetterSink {
	return &deadLetterSink{}
}

func (s *deadLetterSink) setHandler(h DeadLetterHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

func (s *deadLetterSink) emit(letter DeadLetter) {
	s.mu.RLock()
	s.count++
	h := s.handler
	s.mu.RUnlock()

	if h != nil {
		h(letter)
	}
}

// Count returns the total number of dead letters emitted.
func (s *deadLetterSink) Count() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

func (s *deadLetterSink) emitPID(pid PID, payload any, reason string) {
	s.emit(DeadLetter{
		TargetActorID: pid.ActorID,
		TargetPID:     pid,
		Payload:       payload,
		Reason:        reason,
		Timestamp:     time.Now(),
	})
}

func (s *deadLetterSink) emitActor(actorID string, payload any, reason string) {
	s.emit(DeadLetter{
		TargetActorID: actorID,
		Payload:       payload,
		Reason:        reason,
		Timestamp:     time.Now(),
	})
}
