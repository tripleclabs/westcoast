// Package crdt provides conflict-free replicated data types (CRDTs)
// suitable for eventually-consistent distributed systems.
//
// This package has zero external dependencies and can be extracted
// into a standalone module.
package crdt

// VectorTimestamp is a map of node ID to logical clock value.
// Each node increments its own entry; merging takes the max of each.
type VectorTimestamp map[string]uint64

// Clone returns a deep copy.
func (vt VectorTimestamp) Clone() VectorTimestamp {
	if vt == nil {
		return nil
	}
	out := make(VectorTimestamp, len(vt))
	for k, v := range vt {
		out[k] = v
	}
	return out
}

// Merge returns a new timestamp with the component-wise max of both inputs.
func (vt VectorTimestamp) Merge(other VectorTimestamp) VectorTimestamp {
	out := vt.Clone()
	if out == nil {
		out = make(VectorTimestamp)
	}
	for k, v := range other {
		if v > out[k] {
			out[k] = v
		}
	}
	return out
}

// HappensBefore returns true if vt strictly precedes other.
func (vt VectorTimestamp) HappensBefore(other VectorTimestamp) bool {
	atLeastOneLess := false
	for k, v := range vt {
		ov := other[k]
		if v > ov {
			return false
		}
		if v < ov {
			atLeastOneLess = true
		}
	}
	for k, ov := range other {
		if _, ok := vt[k]; !ok && ov > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// Concurrent returns true if neither timestamp happens before the other.
func (vt VectorTimestamp) Concurrent(other VectorTimestamp) bool {
	return !vt.HappensBefore(other) && !other.HappensBefore(vt)
}

// vectorClock tracks causality for a single node.
// Not safe for concurrent use — the ORSet holds a mutex around all access.
type vectorClock struct {
	nodeID string
	ts     VectorTimestamp
}

func newVectorClock(nodeID string) *vectorClock {
	return &vectorClock{
		nodeID: nodeID,
		ts:     VectorTimestamp{nodeID: 0},
	}
}

// Tick increments this node's counter and returns the new timestamp.
func (vc *vectorClock) Tick() VectorTimestamp {
	vc.ts[vc.nodeID]++
	return vc.ts.Clone()
}

// Merge incorporates a remote timestamp and returns the new local timestamp.
func (vc *vectorClock) Merge(remote VectorTimestamp) VectorTimestamp {
	vc.ts = vc.ts.Merge(remote)
	return vc.ts.Clone()
}

// Current returns a snapshot of the current timestamp.
func (vc *vectorClock) Current() VectorTimestamp {
	return vc.ts.Clone()
}
