/*
Copyright 2025 GPU_Orchestrator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backoff

import (
	"testing"
	"time"
)

func TestNextBackoff_Progression(t *testing.T) {
	base := 30 * time.Second

	tests := []struct {
		name    string
		attempt int
		minDur  time.Duration
		maxDur  time.Duration
	}{
		{
			name:    "attempt 0 should be base duration plus jitter",
			attempt: 0,
			minDur:  base,
			maxDur:  base + (base / 10),
		},
		{
			name:    "attempt 1 should be 2x base plus jitter",
			attempt: 1,
			minDur:  2 * base,
			maxDur:  (2 * base) + (base / 5),
		},
		{
			name:    "attempt 2 should be 4x base plus jitter",
			attempt: 2,
			minDur:  4 * base,
			maxDur:  (4 * base) + (base / 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NextBackoff(base, tt.attempt)
			if result < tt.minDur {
				t.Errorf("NextBackoff(%v, %d) = %v, want >= %v", base, tt.attempt, result, tt.minDur)
			}
			if result > tt.maxDur*2 { // Allow some variance due to jitter
				t.Errorf("NextBackoff(%v, %d) = %v, want <= %v", base, tt.attempt, result, tt.maxDur*2)
			}
		})
	}
}

func TestNextBackoff_MaximumCapIsEnforced(t *testing.T) {
	base := 30 * time.Second
	maxAttempt := 100 // Should be capped

	result := NextBackoff(base, maxAttempt)
	maxAllowed := 5 * time.Minute

	if result > maxAllowed {
		t.Errorf("NextBackoff(%v, %d) = %v, should be capped at %v", base, maxAttempt, result, maxAllowed)
	}
}

func TestNextBackoff_NegativeAttempt(t *testing.T) {
	base := 30 * time.Second
	result := NextBackoff(base, -1)

	if result <= 0 {
		t.Errorf("NextBackoff with negative attempt should return positive duration, got %v", result)
	}
}

func TestNextBackoff_ZeroBase(t *testing.T) {
	result := NextBackoff(0, 1)

	if result < 0 {
		t.Errorf("NextBackoff with zero base should not return negative duration, got %v", result)
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name         string
		currentRetry int32
		maxRetries   int32
		expected     bool
	}{
		{
			name:         "under limit",
			currentRetry: 0,
			maxRetries:   3,
			expected:     true,
		},
		{
			name:         "at limit",
			currentRetry: 3,
			maxRetries:   3,
			expected:     false,
		},
		{
			name:         "over limit",
			currentRetry: 5,
			maxRetries:   3,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.currentRetry, tt.maxRetries)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%d, %d) = %v, want %v", tt.currentRetry, tt.maxRetries, result, tt.expected)
			}
		})
	}
}

func BenchmarkNextBackoff(b *testing.B) {
	base := 30 * time.Second
	for i := 0; i < b.N; i++ {
		NextBackoff(base, 3)
	}
}
