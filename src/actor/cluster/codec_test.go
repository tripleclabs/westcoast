package cluster

import (
	"testing"
)

type testPayload struct {
	Name  string
	Value int
}

type nestedPayload struct {
	Inner testPayload
	Tags  []string
}

func TestGobCodec_RoundTrip(t *testing.T) {
	c := NewGobCodec()
	c.Register(testPayload{})

	original := testPayload{Name: "hello", Value: 42}
	data, err := c.Encode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded any
	if err := c.Decode(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, ok := decoded.(testPayload)
	if !ok {
		t.Fatalf("expected testPayload, got %T", decoded)
	}
	if got.Name != "hello" || got.Value != 42 {
		t.Fatalf("got %+v", got)
	}
}

func TestGobCodec_Nested(t *testing.T) {
	c := NewGobCodec()
	c.Register(nestedPayload{})

	original := nestedPayload{
		Inner: testPayload{Name: "inner", Value: 99},
		Tags:  []string{"a", "b"},
	}

	data, err := c.Encode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded any
	if err := c.Decode(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, ok := decoded.(nestedPayload)
	if !ok {
		t.Fatalf("expected nestedPayload, got %T", decoded)
	}
	if got.Inner.Name != "inner" || len(got.Tags) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestGobCodec_String(t *testing.T) {
	c := NewGobCodec()

	data, err := c.Encode("plain string")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded any
	if err := c.Decode(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded != "plain string" {
		t.Fatalf("got %v", decoded)
	}
}

func TestGobCodec_DuplicateRegister(t *testing.T) {
	c := NewGobCodec()
	c.Register(testPayload{})
	c.Register(testPayload{}) // should not panic
}

func TestGobCodec_Name(t *testing.T) {
	c := NewGobCodec()
	if c.Name() != "gob" {
		t.Fatalf("expected gob, got %s", c.Name())
	}
}
