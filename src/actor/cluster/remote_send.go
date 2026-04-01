package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
	"github.com/tripleclabs/westcoast/src/internal/metrics"
)

// nodeIDFromNamespace extracts the target node ID from a PID namespace.
// For normal PIDs, the namespace IS the node ID. For ask-reply PIDs
// like "__ask_reply@node-1", it extracts "node-1".
func nodeIDFromNamespace(ns string) NodeID {
	if strings.HasPrefix(ns, "__ask_reply@") {
		return NodeID(ns[len("__ask_reply@"):])
	}
	return NodeID(ns)
}

// RemoteSender handles encoding and sending messages to remote nodes.
type RemoteSender struct {
	cluster *Cluster
	codec   Codec
	metrics metrics.Hooks
}

// NewRemoteSender creates a RemoteSender that uses the given cluster, codec, and metrics hooks.
func NewRemoteSender(cluster *Cluster, codec Codec, m metrics.Hooks) *RemoteSender {
	if m == nil {
		m = metrics.NopHooks{}
	}
	return &RemoteSender{
		cluster: cluster,
		codec:   codec,
		metrics: m,
	}
}

// Send encodes a message payload and sends it to a remote node as an Envelope.
func (rs *RemoteSender) Send(ctx context.Context, senderActorID string, pid actor.PID, payload any, msgID uint64) (actor.PIDSendAck, error) {
	targetNode := nodeIDFromNamespace(pid.Namespace)
	start := time.Now()

	encoded, err := rs.codec.Encode(payload)
	if err != nil {
		rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "encode_failed")
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound, MessageID: msgID},
			fmt.Errorf("encode payload: %w", err)
	}

	env := Envelope{
		SenderNode:     rs.cluster.LocalNodeID(),
		SenderActorID:  senderActorID,
		TargetNode:     targetNode,
		TargetActorID:  pid.ActorID,
		Namespace:      pid.Namespace,
		Generation:     pid.Generation,
		TypeName:       typeNameOf(payload),
		MessageID:      msgID,
		Payload:        encoded,
		SentAtUnixNano: time.Now().UnixNano(),
	}

	if err := rs.cluster.SendRemote(ctx, targetNode, env); err != nil {
		rs.metrics.ObserveRemoteSendLatency(string(targetNode), time.Since(start))
		rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "send_failed")
		return actor.PIDSendAck{Outcome: actor.PIDUnresolved, MessageID: msgID}, err
	}

	rs.metrics.ObserveRemoteSendLatency(string(targetNode), time.Since(start))
	rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "delivered")
	return actor.PIDSendAck{Outcome: actor.PIDDelivered, MessageID: msgID}, nil
}

// SendAsk sends an ask request to a remote node, embedding the reply-to PID.
func (rs *RemoteSender) SendAsk(ctx context.Context, senderActorID string, pid actor.PID, payload any, msgID uint64, askRequestID string, replyTo actor.PID) (actor.PIDSendAck, error) {
	targetNode := nodeIDFromNamespace(pid.Namespace)
	start := time.Now()

	encoded, err := rs.codec.Encode(payload)
	if err != nil {
		rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "encode_failed")
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound, MessageID: msgID},
			fmt.Errorf("encode payload: %w", err)
	}

	env := Envelope{
		SenderNode:    rs.cluster.LocalNodeID(),
		SenderActorID: senderActorID,
		TargetNode:    targetNode,
		TargetActorID: pid.ActorID,
		Namespace:     pid.Namespace,
		Generation:    pid.Generation,
		TypeName:      typeNameOf(payload),
		MessageID:     msgID,
		Payload:       encoded,
		IsAsk:         true,
		AskRequestID:  askRequestID,
		AskReplyTo: &RemotePID{
			Node:       NodeID(replyTo.Namespace),
			Namespace:  replyTo.Namespace,
			ActorID:    replyTo.ActorID,
			Generation: replyTo.Generation,
		},
		SentAtUnixNano: time.Now().UnixNano(),
	}

	if err := rs.cluster.SendRemote(ctx, targetNode, env); err != nil {
		rs.metrics.ObserveRemoteSendLatency(string(targetNode), time.Since(start))
		rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "send_failed")
		return actor.PIDSendAck{Outcome: actor.PIDUnresolved, MessageID: msgID}, err
	}

	rs.metrics.ObserveRemoteSendLatency(string(targetNode), time.Since(start))
	rs.metrics.ObserveRemoteSendOutcome(string(targetNode), "delivered")
	return actor.PIDSendAck{Outcome: actor.PIDDelivered, MessageID: msgID}, nil
}

// typeNameOf returns the reflect type name of a value, matching actor.Message.TypeName.
func typeNameOf(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%T", v)
}
