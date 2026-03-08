package actor

import "sync/atomic"

type Mailbox struct {
	ch     chan Message
	closed atomic.Bool
}

func NewMailbox(capacity int) *Mailbox {
	if capacity <= 0 {
		capacity = 1024
	}
	return &Mailbox{ch: make(chan Message, capacity)}
}

func (m *Mailbox) Enqueue(msg Message) SubmitResult {
	if m.closed.Load() {
		return SubmitRejectedStop
	}
	select {
	case m.ch <- msg:
		return SubmitAccepted
	default:
		return SubmitRejectedFull
	}
}

func (m *Mailbox) Channel() <-chan Message { return m.ch }

func (m *Mailbox) Depth() int { return len(m.ch) }

func (m *Mailbox) Close() {
	if m.closed.CompareAndSwap(false, true) {
		close(m.ch)
	}
}
