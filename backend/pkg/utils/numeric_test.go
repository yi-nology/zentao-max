package utils

import (
	"encoding/json"
	"testing"
)

func TestToFloat64(t *testing.T) {
	n := json.Number("2.5")
	cases := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"nil", nil, 0},
		{"float64", 3.5, 3.5},
		{"float32", float32(1.5), 1.5},
		{"int", 7, 7},
		{"int64", int64(8), 8},
		{"jsonNumber", n, 2.5},
		{"string", "3.5", 3.5},
		{"stringInt", "8", 8},
		{"badString", "abc", 0},
		{"otherType", true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ToFloat64(c.in); got != c.want {
				t.Fatalf("ToFloat64(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	n := json.Number("42")
	cases := []struct {
		name string
		in   interface{}
		want int
	}{
		{"nil", nil, 0},
		{"int", 42, 42},
		{"int64", int64(43), 43},
		{"float64", 42.9, 42},
		{"jsonNumber", n, 42},
		{"string", "42", 42},
		{"stringFloat", "3.5", 3},
		{"badString", "abc", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ToInt(c.in); got != c.want {
				t.Fatalf("ToInt(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
