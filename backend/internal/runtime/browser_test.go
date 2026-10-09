package runtime

import (
	"testing"
)

func TestFormatServerURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{":8080", "http://localhost:8080"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"0.0.0.0:8080", "http://localhost:8080"},
		{"[::]:8080", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"192.168.1.50:9000", "http://192.168.1.50:9000"},
	}

	for _, tc := range tests {
		got := FormatServerURL(tc.input)
		if got != tc.expected {
			t.Errorf("FormatServerURL(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
