package cluster

import (
	"context"

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

// InboundDispatcher handles envelopes arriving from remote nodes.
// It decodes the payload and routes to the local Runtime, or forwards
// the envelope to the next hop if the target is a different node.
type InboundDispatcher struct {
	bridge  RuntimeBridge
	codec   Codec
	cluster *Cluster // non-nil enables multi-hop forwarding
}

func NewInboundDispatcher(bridge RuntimeBridge, codec Codec) *InboundDispatcher {
	return &InboundDispatcher{
		bridge: bridge,
		codec:  codec,
	}
}

// SetCluster enables multi-hop forwarding through the cluster.
func (d *InboundDispatcher) SetCluster(c *Cluster) {
	d.cluster = c
}

// Dispatch processes an incoming envelope from a remote node.
func (d *InboundDispatcher) Dispatch(ctx context.Context, env Envelope) {
	// Multi-hop forwarding: if the envelope is not for us, forward it.
	if d.cluster != nil && env.TargetNode != "" && env.TargetNode != NodeID(d.bridge.NodeID()) {
		d.cluster.SendRemote(ctx, env.TargetNode, env)
		return
	}

	// Decode the payload.
	var payload any
	if err := d.codec.Decode(env.Payload, &payload); err != nil {
		return // drop malformed payloads
	}

	// If this is an ask reply being returned to us, route via PID.
	if env.IsAsk && env.AskReplyTo != nil {
		// This is a request (not a reply). Deliver to the target actor
		// with the ask context so the actor can reply.
		askCtx := &actor.AskRequestContext{
			RequestID: env.AskRequestID,
			ReplyTo: actor.PID{
				Namespace:  env.AskReplyTo.Namespace,
				ActorID:    env.AskReplyTo.ActorID,
				Generation: env.AskReplyTo.Generation,
			},
		}
		d.bridge.DeliverLocal(ctx, env.TargetActorID, payload, askCtx)
		return
	}

	// Check if this is an ask reply (payload going to an ask-reply PID).
	if isAskReplyLocal(env.Namespace, d.bridge.NodeID()) {
		pid := actor.PID{
			Namespace:  env.Namespace,
			ActorID:    env.TargetActorID,
			Generation: env.Generation,
		}
		d.bridge.DeliverPID(ctx, pid, payload)
		return
	}

	// Standard fire-and-forget delivery.
	d.bridge.DeliverLocal(ctx, env.TargetActorID, payload, nil)
}
