package server

import (
	"testing"
)

func TestPatternParts_EdgeCases(t *testing.T) {
	tests := []struct {
		pattern    string
		wantMethod string
		wantName   string
		wantPath   string
	}{
		// Standard patterns
		{"GET /path", "GET", "", "/path"},
		{"/path", "", "", "/path"},
		{"example.com/", "", "example.com", "/"},
		{"GET example.com/path", "GET", "example.com", "/path"},

		// Edge cases: Whitespace
		{"  GET /path", "GET", "", "/path"}, // Leading space: now handled by TrimSpace
		{"GET  /path", "", "", ""},          // Multiple spaces: fails stricter regex (expects single space)
		{"GET /path  ", "GET", "", "/path"}, // Trailing space: now handled by TrimSpace

		// Edge cases: Method only or Host only (should they work?)
		{"GET ", "", "GET", ""},                         // Method + space: without path, it treats GET as host
		{"example.com", "", "example.com", ""},          // Host only
		{"POST example.com", "POST", "example.com", ""}, // Method + Host

		// Edge cases: Unusual paths
		{"GET /path with spaces", "GET", "", "/path with spaces"}, // Spaces in path
		{"GET /path/to/{id}", "GET", "", "/path/to/{id}"},         // Path variables
		{"GET /path/{$}", "GET", "", "/path/{$}"},                 // Exact match wildcard
	}

	for _, tt := range tests {
		m, n, p := PatternParts(tt.pattern)
		if m != tt.wantMethod || n != tt.wantName || p != tt.wantPath {
			t.Errorf("PatternParts(%q) = %q, %q, %q; want %q, %q, %q",
				tt.pattern, m, n, p, tt.wantMethod, tt.wantName, tt.wantPath)
		}
	}
}
