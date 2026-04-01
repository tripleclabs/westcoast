package cluster

import "strconv"

// NodeMatcher is a predicate on NodeMeta. Used for placement decisions
// in DaemonSets, singleton affinity, worker filtering, etc.
type NodeMatcher func(NodeMeta) bool

// MatchAll returns a matcher that requires all sub-matchers to pass (AND).
func MatchAll(matchers ...NodeMatcher) NodeMatcher {
	return func(meta NodeMeta) bool {
		for _, m := range matchers {
			if !m(meta) {
				return false
			}
		}
		return true
	}
}

// MatchAny returns a matcher that requires at least one sub-matcher to pass (OR).
func MatchAny(matchers ...NodeMatcher) NodeMatcher {
	return func(meta NodeMeta) bool {
		for _, m := range matchers {
			if m(meta) {
				return true
			}
		}
		return false
	}
}

// Not negates a matcher.
func Not(matcher NodeMatcher) NodeMatcher {
	return func(meta NodeMeta) bool {
		return !matcher(meta)
	}
}

// TagEquals matches nodes where the tag has the exact value.
func TagEquals(key, value string) NodeMatcher {
	return func(meta NodeMeta) bool {
		return meta.Tags[key] == value
	}
}

// TagExists matches nodes that have the tag set (any value).
func TagExists(key string) NodeMatcher {
	return func(meta NodeMeta) bool {
		_, ok := meta.Tags[key]
		return ok
	}
}

// TagNotExists matches nodes that do NOT have the tag set.
func TagNotExists(key string) NodeMatcher {
	return Not(TagExists(key))
}

// TagIn matches nodes where the tag value is one of the given values.
func TagIn(key string, values ...string) NodeMatcher {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return func(meta NodeMeta) bool {
		return set[meta.Tags[key]]
	}
}

// TagGTE matches nodes where the tag value is >= threshold (numeric comparison).
// Non-numeric or missing tags do not match.
func TagGTE(key string, threshold int) NodeMatcher {
	return func(meta NodeMeta) bool {
		v, err := strconv.Atoi(meta.Tags[key])
		if err != nil {
			return false
		}
		return v >= threshold
	}
}

// TagLTE matches nodes where the tag value is <= threshold (numeric comparison).
func TagLTE(key string, threshold int) NodeMatcher {
	return func(meta NodeMeta) bool {
		v, err := strconv.Atoi(meta.Tags[key])
		if err != nil {
			return false
		}
		return v <= threshold
	}
}

// MatchAllNodes matches every node unconditionally.
func MatchAllNodes() NodeMatcher {
	return func(NodeMeta) bool { return true }
}

// FilterNodes returns the subset of nodes matching the predicate.
func FilterNodes(nodes []NodeMeta, matcher NodeMatcher) []NodeMeta {
	if matcher == nil {
		return nodes
	}
	var out []NodeMeta
	for _, n := range nodes {
		if matcher(n) {
			out = append(out, n)
		}
	}
	return out
}

// NodeRanker scores a node for placement preference. Higher = preferred.
type NodeRanker func(NodeMeta) int

// RankByTag ranks nodes by a numeric tag value. Direction controls whether
// higher or lower values are preferred.
func RankByTag(key string, direction RankDirection) NodeRanker {
	return func(meta NodeMeta) int {
		v, err := strconv.Atoi(meta.Tags[key])
		if err != nil {
			return 0
		}
		if direction == Lowest {
			return -v
		}
		return v
	}
}

// RankDirection controls whether higher or lower values are preferred.
type RankDirection int

const (
	// Highest prefers nodes with the largest tag value.
	Highest RankDirection = iota
	// Lowest prefers nodes with the smallest tag value.
	Lowest
)
