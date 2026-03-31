package cluster

import (
	"testing"
	"time"
)

func TestEnvelope_RoundTrip(t *testing.T) {
	original := Envelope{
		SenderNode:     "node-1",
		SenderActorID:  "actor-a",
		TargetNode:     "node-2",
		TargetActorID:  "actor-b",
		Namespace:      "node-2",
		Generation:     3,
		TypeName:       "MyMessage",
		SchemaVersion:  "v2",
		MessageID:      42,
		Payload:        []byte("hello world"),
		IsAsk:          true,
		AskRequestID:   "ask-1",
		AskReplyTo:     &RemotePID{Node: "node-1", Namespace: "__ask_reply@node-1", ActorID: "ask-1", Generation: 1},
		SentAtUnixNano: time.Now().UnixNano(),
	}

	data, err := encodeEnvelope(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.SenderNode != original.SenderNode {
		t.Errorf("SenderNode: got %s, want %s", decoded.SenderNode, original.SenderNode)
	}
	if decoded.TargetNode != original.TargetNode {
		t.Errorf("TargetNode: got %s, want %s", decoded.TargetNode, original.TargetNode)
	}
	if decoded.TargetActorID != original.TargetActorID {
		t.Errorf("TargetActorID: got %s, want %s", decoded.TargetActorID, original.TargetActorID)
	}
	if decoded.Generation != original.Generation {
		t.Errorf("Generation: got %d, want %d", decoded.Generation, original.Generation)
	}
	if decoded.TypeName != original.TypeName {
		t.Errorf("TypeName: got %s, want %s", decoded.TypeName, original.TypeName)
	}
	if decoded.MessageID != original.MessageID {
		t.Errorf("MessageID: got %d, want %d", decoded.MessageID, original.MessageID)
	}
	if decoded.IsAsk != original.IsAsk {
		t.Errorf("IsAsk: got %v, want %v", decoded.IsAsk, original.IsAsk)
	}
	if decoded.AskRequestID != original.AskRequestID {
		t.Errorf("AskRequestID: got %s, want %s", decoded.AskRequestID, original.AskRequestID)
	}
	if decoded.AskReplyTo == nil {
		t.Fatal("AskReplyTo should not be nil")
	}
	if decoded.AskReplyTo.Node != "node-1" {
		t.Errorf("AskReplyTo.Node: got %s, want node-1", decoded.AskReplyTo.Node)
	}
	if string(decoded.Payload) != "hello world" {
		t.Errorf("Payload: got %s, want hello world", decoded.Payload)
	}
}

func TestEnvelope_NilAskReplyTo(t *testing.T) {
	original := Envelope{
		SenderNode:    "node-1",
		TargetNode:    "node-2",
		TargetActorID: "actor-b",
		Payload:       []byte("fire-and-forget"),
	}

	data, err := encodeEnvelope(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.AskReplyTo != nil {
		t.Error("AskReplyTo should be nil for fire-and-forget")
	}
	if decoded.IsAsk {
		t.Error("IsAsk should be false")
	}
}
