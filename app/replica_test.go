package app

import (
	"testing"
)

func TestIsDecimal(t *testing.T) {
	var validSizeTests = []struct {
		input  string
		output bool
	}{
		{"1G", true},
		{"1g", true},
		{"1K", true},
		{"1k", true},
		{"1M", true},
		{"1m", true},
		{"1Gb", true},
		{"1GB", true},
		{"1gB", true},
		{"1gb", true},
		{"1.2g", true},
		{"1.2G", true},
		{"0.2gb", true},
		{"1Gbb", false},
		{"1gbb", false},
		{"1gi", false},
		{"1Gi", false},
		{"1Gib", false},
		{"1gib", false},
		{".2gb", false},
	}

	for _, tt := range validSizeTests {
		out := IsDecimal(tt.input)
		t.Logf("Size passed is %v, expected output %v, got %v", tt.input, tt.output, out)
		if out != tt.output {
			t.Errorf("IsDecimal(%v) => %v, expected output %v", tt.input, out, tt.output)
		}
	}
}

func TestIsBinary(t *testing.T) {
	var validSizeTests = []struct {
		input  string
		output bool
	}{
		{"1Gi", true},
		{"1gi", true},
		{"1Ki", true},
		{"1ki", true},
		{"1Mi", true},
		{"1mi", true},
		{"0.2gi", true},
		{"1Ti", true},
		{"1.2gi", true},
		{"1Gb", false},
		{"1GB", false},
		{"1gB", false},
		{"1gb", false},
		{"1Gbb", false},
		{"1gbb", false},
		{"1kb", false},
		{"1Gib", false},
		{"1gib", false},
		{".2gi", false},
	}

	for _, tt := range validSizeTests {
		out := IsBinary(tt.input)
		t.Logf("Size passed is %v, expected output %v, got %v", tt.input, tt.output, out)
		if out != tt.output {
			t.Errorf("IsDecimal(%v) => %v, expected output %v", tt.input, out, tt.output)
		}
	}
}
