package upgrader

import (
	"testing"
	"time"
)

func TestExtractImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "simple image with tag",
			image:    "istio/proxyv2:1.17.0",
			expected: "1.17.0",
		},
		{
			name:     "image with registry and tag",
			image:    "gcr.io/istio/proxyv2:1.17.0",
			expected: "1.17.0",
		},
		{
			name:     "image with registry port and tag",
			image:    "registry.io:5000/istio/proxyv2:1.17.0",
			expected: "1.17.0",
		},
		{
			name:     "image with multiple path segments",
			image:    "gcr.io/google-containers/istio/proxyv2:1.20.0",
			expected: "1.20.0",
		},
		{
			name:     "image without tag",
			image:    "istio/proxyv2",
			expected: "",
		},
		{
			name:     "image with digest format",
			image:    "istio/proxyv2@sha256:abc123def456",
			expected: "",
		},
		{
			name:     "image with registry port but no tag",
			image:    "registry.io:5000/istio/proxyv2",
			expected: "",
		},
		{
			name:     "empty image",
			image:    "",
			expected: "",
		},
		{
			name:     "image with latest tag",
			image:    "istio/proxyv2:latest",
			expected: "latest",
		},
		{
			name:     "localhost registry with port",
			image:    "localhost:5000/proxyv2:1.18.0",
			expected: "1.18.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageTag(tt.image)
			if result != tt.expected {
				t.Errorf("extractImageTag(%q) = %q, want %q", tt.image, result, tt.expected)
			}
		})
	}
}

func TestDateEqual(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")

	tests := []struct {
		name     string
		date1    time.Time
		date2    time.Time
		expected bool
	}{
		{
			name:     "same date same time",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "same date different time",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC),
			expected: true,
		},
		{
			name:     "different day",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 16, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "different month",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 2, 15, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "different year",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "same date different timezone",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 15, 17, 30, 0, 0, loc),
			expected: true,
		},
		{
			name:     "zero times",
			date1:    time.Time{},
			date2:    time.Time{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DateEqual(tt.date1, tt.date2)
			if result != tt.expected {
				t.Errorf("DateEqual(%v, %v) = %v, want %v", tt.date1, tt.date2, result, tt.expected)
			}
		})
	}
}
