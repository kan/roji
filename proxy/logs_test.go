package proxy

import (
	"testing"
	"time"
)

func TestLogBuffer_ListFiltered(t *testing.T) {
	lb := NewLogBuffer(100)

	// Add test logs
	now := time.Now()
	logs := []RequestLog{
		{Timestamp: now.Add(-3 * time.Hour), Method: "GET", Host: "api.localhost", Path: "/users", Status: 200, Service: "api"},
		{Timestamp: now.Add(-2 * time.Hour), Method: "POST", Host: "api.localhost", Path: "/users", Status: 201, Service: "api"},
		{Timestamp: now.Add(-1 * time.Hour), Method: "GET", Host: "web.localhost", Path: "/", Status: 200, Service: "web"},
		{Timestamp: now, Method: "DELETE", Host: "api.localhost", Path: "/users/1", Status: 204, Service: "api"},
	}
	for _, log := range logs {
		lb.Add(log)
	}

	tests := []struct {
		name     string
		filter   LogFilter
		expected int
	}{
		{
			name:     "no filter returns all",
			filter:   LogFilter{},
			expected: 4,
		},
		{
			name:     "filter by service",
			filter:   LogFilter{Service: "api"},
			expected: 3,
		},
		{
			name:     "filter by host",
			filter:   LogFilter{Host: "web.localhost"},
			expected: 1,
		},
		{
			name:     "filter by method",
			filter:   LogFilter{Method: "GET"},
			expected: 2,
		},
		{
			name:     "filter by time range",
			filter:   LogFilter{From: now.Add(-90 * time.Minute), To: now.Add(-30 * time.Minute)},
			expected: 1,
		},
		{
			name:     "combined filters",
			filter:   LogFilter{Service: "api", Method: "GET"},
			expected: 1,
		},
		{
			name:     "no matches",
			filter:   LogFilter{Service: "nonexistent"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lb.ListFiltered(tt.filter)
			if len(result) != tt.expected {
				t.Errorf("ListFiltered() returned %d logs, expected %d", len(result), tt.expected)
			}
		})
	}
}

func TestCsvEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with,comma", "\"with,comma\""},
		{"with\"quote", "\"with\"\"quote\""},
		{"with\nnewline", "\"with\nnewline\""},
		{"/path/to/file", "/path/to/file"},
		{"has,\"both", "\"has,\"\"both\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := csvEscape(tt.input)
			if result != tt.expected {
				t.Errorf("csvEscape(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
