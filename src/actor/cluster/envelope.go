package cluster

// Envelope is the wire format for messages between nodes.
// The Payload field carries the codec-encoded message body.
// Routing metadata lives in the envelope header so receivers can
// make routing decisions without decoding the payload.
type Envelope struct {
	// Sender identification
	SenderNode    NodeID
	SenderActorID string

	// Target identification
	TargetNode    NodeID
	TargetActorID string

	// PID fields for generation tracking
	Namespace  string
	Generation uint64

	// Message metadata (from actor.Message)
	TypeName      string
	SchemaVersion string
	MessageID     uint64

	// Codec-encoded payload
	Payload []byte

	// Ask/Reply support
	IsAsk        bool
	AskRequestID string
	AskReplyTo   *RemotePID // nil for fire-and-forget

	// Timing
	SentAtUnixNano int64
}

// RemotePID is a wire-safe representation of an actor PID.
type RemotePID struct {
	Node       NodeID
	Namespace  string
	ActorID    string
	Generation uint64
}
