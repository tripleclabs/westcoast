package actor

import "sync"

type Mailbox struct {
	mu     sync.Mutex
	queue  []Message
	head   int
	max    int
	notify chan struct{}
	closed bool
}

func NewMailbox(capacity int) *Mailbox {
	if capacity <= 0 {
		capacity = 16
	}
	initialCap := capacity
	if initialCap > 16 {
		initialCap = 16
	}
	return &Mailbox{
		queue:  make([]Message, 0, initialCap),
		max:    capacity,
		notify: make(chan struct{}, 1),
	}
}

func (m *Mailbox) Enqueue(msg Message) SubmitResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return SubmitRejectedStop
	}
	if len(m.queue)-m.head >= m.max {
		return SubmitRejectedFull
	}

	wasEmpty := len(m.queue)-m.head == 0
	m.queue = append(m.queue, msg)
	if wasEmpty {
		select {
		case m.notify <- struct{}{}:
		default:
		}
	}
	return SubmitAccepted
}

func (m *Mailbox) Notify() <-chan struct{} { return m.notify }

func (m *Mailbox) Dequeue() (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue)-m.head == 0 {
		return Message{}, false
	}
	msg := m.queue[m.head]
	m.queue[m.head] = Message{}
	m.head++
	if m.head >= len(m.queue) {
		m.queue = m.queue[:0]
		m.head = 0
	} else if m.head > 64 && m.head*2 >= len(m.queue) {
		remaining := len(m.queue) - m.head
		copy(m.queue[:remaining], m.queue[m.head:])
		m.queue = m.queue[:remaining]
		m.head = 0
	}
	if len(m.queue)-m.head > 0 {
		select {
		case m.notify <- struct{}{}:
		default:
		}
	}
	return msg, true
}

func (m *Mailbox) Depth() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue) - m.head
}

func (m *Mailbox) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
}
