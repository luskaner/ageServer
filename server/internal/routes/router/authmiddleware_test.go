package router

import (
	"testing"

	i "github.com/luskaner/ageServer/server/internal"
)

func TestSafeString(t *testing.T) {
	data := i.A{"hello", 42, nil}

	if s, ok := safeString(data, 0); !ok || s != "hello" {
		t.Errorf("index 0: got %q %v", s, ok)
	}
	if _, ok := safeString(data, 1); ok {
		t.Error("index 1 is int, should not assert to string")
	}
	if _, ok := safeString(data, 5); ok {
		t.Error("out of range should fail")
	}
	if _, ok := safeString(nil, 0); ok {
		t.Error("nil data should fail")
	}
}

func TestSafeNestedFloat(t *testing.T) {
	// Structure expected by safeNestedFloat:
	// data[index][0] must be i.A and data[index][0][1] must be float64.
	nested := i.A{
		"padding",
		i.A{
			i.A{
				"label",
				float64(12345),
			},
		},
	}

	got := safeNestedFloat(nested, 1)
	if got != 12345 {
		t.Fatalf("got %v, want 12345", got)
	}

	if got := safeNestedFloat(i.A{}, 0); got != 0 {
		t.Errorf("empty data: got %v", got)
	}
}

func TestSafeNestedArray(t *testing.T) {
	inner := i.A{"a", "b"}
	data := i.A{inner, "other"}

	arr, ok := safeNestedArray(data, 0)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v %v", arr, ok)
	}
}
