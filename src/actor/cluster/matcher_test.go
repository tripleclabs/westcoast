package cluster

import (
	"testing"
)

func meta(id string, tags map[string]string) NodeMeta {
	return NodeMeta{ID: NodeID(id), Tags: tags}
}

func TestTagEquals(t *testing.T) {
	m := TagEquals("region", "us-east-1")
	if !m(meta("n1", map[string]string{"region": "us-east-1"})) {
		t.Error("should match")
	}
	if m(meta("n2", map[string]string{"region": "eu-west-1"})) {
		t.Error("should not match")
	}
	if m(meta("n3", map[string]string{})) {
		t.Error("missing tag should not match")
	}
}

func TestTagExists(t *testing.T) {
	m := TagExists("gpus")
	if !m(meta("n1", map[string]string{"gpus": "4"})) {
		t.Error("should match")
	}
	if m(meta("n2", map[string]string{"cpus": "8"})) {
		t.Error("should not match")
	}
}

func TestTagNotExists(t *testing.T) {
	m := TagNotExists("gpus")
	if m(meta("n1", map[string]string{"gpus": "4"})) {
		t.Error("should not match")
	}
	if !m(meta("n2", map[string]string{"cpus": "8"})) {
		t.Error("should match")
	}
}

func TestTagIn(t *testing.T) {
	m := TagIn("region", "us-east-1", "us-west-2")
	if !m(meta("n1", map[string]string{"region": "us-east-1"})) {
		t.Error("should match")
	}
	if !m(meta("n2", map[string]string{"region": "us-west-2"})) {
		t.Error("should match")
	}
	if m(meta("n3", map[string]string{"region": "eu-west-1"})) {
		t.Error("should not match")
	}
}

func TestTagGTE(t *testing.T) {
	m := TagGTE("gpus", 2)
	if !m(meta("n1", map[string]string{"gpus": "4"})) {
		t.Error("4 >= 2")
	}
	if !m(meta("n2", map[string]string{"gpus": "2"})) {
		t.Error("2 >= 2")
	}
	if m(meta("n3", map[string]string{"gpus": "1"})) {
		t.Error("1 < 2")
	}
	if m(meta("n4", map[string]string{"gpus": "not-a-number"})) {
		t.Error("non-numeric should not match")
	}
	if m(meta("n5", map[string]string{})) {
		t.Error("missing should not match")
	}
}

func TestTagLTE(t *testing.T) {
	m := TagLTE("load", 50)
	if !m(meta("n1", map[string]string{"load": "30"})) {
		t.Error("30 <= 50")
	}
	if m(meta("n2", map[string]string{"load": "80"})) {
		t.Error("80 > 50")
	}
}

func TestMatchAll(t *testing.T) {
	m := MatchAll(
		TagEquals("region", "us-east-1"),
		TagGTE("gpus", 1),
	)
	if !m(meta("n1", map[string]string{"region": "us-east-1", "gpus": "4"})) {
		t.Error("should match both")
	}
	if m(meta("n2", map[string]string{"region": "us-east-1", "gpus": "0"})) {
		t.Error("gpus < 1")
	}
	if m(meta("n3", map[string]string{"region": "eu-west-1", "gpus": "4"})) {
		t.Error("wrong region")
	}
}

func TestMatchAny(t *testing.T) {
	m := MatchAny(
		TagEquals("region", "us-east-1"),
		TagEquals("region", "us-west-2"),
	)
	if !m(meta("n1", map[string]string{"region": "us-east-1"})) {
		t.Error("should match")
	}
	if !m(meta("n2", map[string]string{"region": "us-west-2"})) {
		t.Error("should match")
	}
	if m(meta("n3", map[string]string{"region": "eu-west-1"})) {
		t.Error("should not match")
	}
}

func TestNot(t *testing.T) {
	m := Not(TagEquals("role", "canary"))
	if !m(meta("n1", map[string]string{"role": "worker"})) {
		t.Error("should match non-canary")
	}
	if m(meta("n2", map[string]string{"role": "canary"})) {
		t.Error("should not match canary")
	}
}

func TestFilterNodes(t *testing.T) {
	nodes := []NodeMeta{
		meta("n1", map[string]string{"gpus": "4"}),
		meta("n2", map[string]string{"gpus": "0"}),
		meta("n3", map[string]string{"gpus": "8"}),
		meta("n4", map[string]string{}),
	}

	gpu := FilterNodes(nodes, TagGTE("gpus", 1))
	if len(gpu) != 2 {
		t.Errorf("expected 2 GPU nodes, got %d", len(gpu))
	}
}

func TestFilterNodes_NilMatcher(t *testing.T) {
	nodes := []NodeMeta{meta("n1", nil), meta("n2", nil)}
	result := FilterNodes(nodes, nil)
	if len(result) != 2 {
		t.Error("nil matcher should return all")
	}
}

func TestRankByTag(t *testing.T) {
	r := RankByTag("memory-gb", Highest)
	n1 := meta("n1", map[string]string{"memory-gb": "64"})
	n2 := meta("n2", map[string]string{"memory-gb": "128"})
	n3 := meta("n3", map[string]string{})

	if r(n2) <= r(n1) {
		t.Error("128 should rank higher than 64")
	}
	if r(n3) != 0 {
		t.Error("missing tag should rank 0")
	}
}

func TestRankByTag_Lowest(t *testing.T) {
	r := RankByTag("load", Lowest)
	n1 := meta("n1", map[string]string{"load": "20"})
	n2 := meta("n2", map[string]string{"load": "80"})

	if r(n1) <= r(n2) {
		t.Error("20 should rank higher than 80 when preferring lowest")
	}
}

func TestMatchAllNodes(t *testing.T) {
	m := MatchAllNodes()
	if !m(meta("any", nil)) {
		t.Error("should match everything")
	}
}
