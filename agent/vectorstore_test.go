package agent

import (
	"math"
	"testing"
)

func TestEmbeddingSerdeRoundtrip(t *testing.T) {
	cases := [][]float32{
		{},
		{0},
		{1.5, -2.25, 0, 1e-9, 1e9},
		{float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())},
	}

	for i, in := range cases {
		bytes := serializeEmbedding(in)
		out, err := deserializeEmbedding(bytes)
		if err != nil {
			t.Fatalf("case %d: deserialize error: %v", i, err)
		}
		if len(out) != len(in) {
			t.Fatalf("case %d: length mismatch got %d want %d", i, len(out), len(in))
		}
		for j := range in {
			a, b := in[j], out[j]
			isNaN := math.IsNaN(float64(a)) && math.IsNaN(float64(b))
			if !isNaN && a != b {
				t.Fatalf("case %d index %d: got %v want %v", i, j, b, a)
			}
		}
	}
}

func TestDeserializeEmbeddingInvalidLength(t *testing.T) {
	if _, err := deserializeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for length-3 buffer, got nil")
	}
	if _, err := deserializeEmbedding([]byte{1}); err == nil {
		t.Fatal("expected error for length-1 buffer, got nil")
	}
}

func TestSerializeEmbeddingByteLength(t *testing.T) {
	got := serializeEmbedding([]float32{1, 2, 3, 4})
	if len(got) != 16 {
		t.Fatalf("expected 16 bytes for 4 floats, got %d", len(got))
	}
}
