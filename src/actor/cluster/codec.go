package cluster

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
)

// Codec serializes and deserializes message payloads for wire transport.
// TypeName and SchemaVersion travel in the Envelope metadata, not inside
// the encoded payload, so the receiver can route before decoding.
type Codec interface {
	// Encode serializes a value to bytes.
	Encode(v any) ([]byte, error)
	// Decode deserializes bytes into the provided pointer.
	Decode(data []byte, v any) error
	// Register makes a concrete type known to the codec.
	// For gob this maps to gob.Register; other codecs may no-op.
	Register(v any)
	// Name returns a human-readable codec identifier (e.g. "gob", "proto").
	Name() string
}

// GobCodec implements Codec using Go's encoding/gob.
type GobCodec struct {
	mu         sync.Mutex
	registered map[string]bool
}

// NewGobCodec returns a new GobCodec ready for use.
func NewGobCodec() *GobCodec {
	return &GobCodec{
		registered: make(map[string]bool),
	}
}

// Encode serializes a value to bytes using gob encoding.
func (c *GobCodec) Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&v); err != nil {
		return nil, fmt.Errorf("gob encode: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode deserializes gob-encoded bytes into the provided pointer.
func (c *GobCodec) Decode(data []byte, v any) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(v)
}

// Register makes a concrete type known to gob for interface encoding.
func (c *GobCodec) Register(v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%T", v)
	if c.registered[key] {
		return
	}
	gob.Register(v)
	c.registered[key] = true
}

// Name returns "gob" as the codec identifier.
func (c *GobCodec) Name() string { return "gob" }
