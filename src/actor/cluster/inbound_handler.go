package cluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/tripleclabs/westcoast/src/actor"
)

// RuntimeBridge is the interface that the Runtime implements to receive
// inbound messages from the cluster transport layer.
type RuntimeBridge interface {
	// DeliverLocal delivers a message to a local actor by ID.
	DeliverLocal(ctx context.Context, actorID string, payload any, askCtx *actor.AskRequestContext) actor.SubmitAck

	// DeliverPID delivers a message to a local actor by PID (for ask replies
	// that come back from remote nodes).
	DeliverPID(ctx context.Context, pid actor.PID, payload any) actor.PIDSendAck

	// NodeID returns this node's identity as a string.
	NodeID() string
}

// SystemEnvelopeHandler handles a system-level envelope (gossip, pubsub
// broadcast, etc.) that is not destined for an actor.
type SystemEnvelopeHandler func(from NodeID, env Envelope)

// InboundDispatcher handles envelopes arriving from remote nodes.
// It routes system envelopes (gossip, pubsub) to registered handlers,
// actor messages to the local Runtime, and forwards envelopes not
// destined for this node to the next hop.
type InboundDispatcher struct {
	bridge  RuntimeBridge
	codec   Codec
	cluster *Cluster // non-nil enables multi-hop forwarding

	mu       sync.RWMutex
	handlers map[string]SystemEnvelopeHandler // TypeName → handler
}

// NewInboundDispatcher creates a dispatcher that routes inbound envelopes to the given bridge.
func NewInboundDispatcher(bridge RuntimeBridge, codec Codec) *InboundDispatcher {
	return &InboundDispatcher{
		bridge:   bridge,
		codec:    codec,
		handlers: make(map[string]SystemEnvelopeHandler),
	}
}

// SetCluster enables multi-hop forwarding through the cluster.
func (d *InboundDispatcher) SetCluster(c *Cluster) {
	d.cluster = c
}

// RegisterHandler registers a handler for a system envelope type.
// When an envelope arrives with the given TypeName, the handler is called
// instead of routing to an actor.
func (d *InboundDispatcher) RegisterHandler(typeName string, handler SystemEnvelopeHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[typeName] = handler
}

// Dispatch processes an incoming envelope from a remote node.
// Returns an error if the payload cannot be decoded. Callers should
// log or count these errors — a non-nil error means the message was lost.
func (d *InboundDispatcher) Dispatch(ctx context.Context, from NodeID, env Envelope) error {
	// Multi-hop forwarding: if the envelope is not for us, forward it.
	if d.cluster != nil && env.TargetNode != "" && env.TargetNode != NodeID(d.bridge.NodeID()) {
		return d.cluster.SendRemote(ctx, env.TargetNode, env)
	}

	// System envelope handlers (gossip, pubsub broadcast, etc.).
	d.mu.RLock()
	handler, isSystem := d.handlers[env.TypeName]
	d.mu.RUnlock()
	if isSystem {
		handler(from, env)
		return nil
	}

	// Decode the payload for actor delivery.
	var payload any
	if err := d.codec.Decode(env.Payload, &payload); err != nil {
		return fmt.Errorf("dispatch: decode payload (type=%s, from=%s): %w", env.TypeName, from, err)
	}

	// Ask request: deliver with ask context so the actor can reply.
	if env.IsAsk && env.AskReplyTo != nil {
		askCtx := &actor.AskRequestContext{
			RequestID: env.AskRequestID,
			ReplyTo: actor.PID{
				Namespace:  env.AskReplyTo.Namespace,
				ActorID:    env.AskReplyTo.ActorID,
				Generation: env.AskReplyTo.Generation,
			},
		}
		d.bridge.DeliverLocal(ctx, env.TargetActorID, payload, askCtx)
		return nil
	}

	// Ask reply returning to us.
	if isAskReplyLocal(env.Namespace, d.bridge.NodeID()) {
		pid := actor.PID{
			Namespace:  env.Namespace,
			ActorID:    env.TargetActorID,
			Generation: env.Generation,
		}
		d.bridge.DeliverPID(ctx, pid, payload)
		return nil
	}

	// Standard fire-and-forget delivery.
	d.bridge.DeliverLocal(ctx, env.TargetActorID, payload, nil)
	return nil
}
