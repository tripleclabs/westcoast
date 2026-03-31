package crdt

import "fmt"

// Tag uniquely identifies an add operation in the OR-Set.
// It encodes the originating node and a monotonic sequence number,
// making it globally unique and deterministically comparable.
type Tag string

// NewTag creates a tag from a node ID and sequence number.
func NewTag(nodeID string, seq uint64) Tag {
	return Tag(fmt.Sprintf("%s-%d", nodeID, seq))
}

// Dominates returns true if this tag should win a deterministic tiebreak
// against other. Used when vector clocks are concurrent (neither happens
// before the other) and we need a consistent winner across all nodes.
//
// The specific ordering doesn't matter as long as it's total and
// deterministic — every node must pick the same winner.
func (t Tag) Dominates(other Tag) bool {
	return t > other
}

// String returns the tag as a string.
func (t Tag) String() string { return string(t) }
