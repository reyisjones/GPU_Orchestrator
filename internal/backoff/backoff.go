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

// Package backoff provides exponential backoff utilities with jitter
// to prevent thundering herd problems when retrying failed operations.
package backoff

import (
	"math"
	"math/rand"
	"time"
)

// NextBackoff calculates the next backoff duration using exponential backoff with jitter.
//
// The formula is:
//
//	backoff = base * 2^attempt + random jitter
//
// where jitter is a random value between 0 and base * 2^attempt * 0.1 (10% jitter).
//
// This prevents synchronized retries across multiple goroutines/controllers.
//
// Parameters:
//   - base: the base backoff duration (e.g., 30 seconds)
//   - attempt: the retry attempt number (0-indexed)
//
// Returns:
//   - the calculated backoff duration, capped at a reasonable maximum (5 minutes)
//
// Example:
//
//	backoff := NextBackoff(30*time.Second, 0) // ~30s + jitter
//	backoff := NextBackoff(30*time.Second, 1) // ~60s + jitter
//	backoff := NextBackoff(30*time.Second, 2) // ~120s + jitter
func NextBackoff(base time.Duration, attempt int) time.Duration {
	// Prevent overflow by capping attempt to a reasonable maximum
	maxAttempt := 10
	if attempt > maxAttempt {
		attempt = maxAttempt
	}

	// Calculate exponential backoff: base * 2^attempt
	exponentialDuration := float64(base) * math.Pow(2, float64(attempt))

	// Cap at 5 minutes to prevent extremely long wait times
	maxDuration := 5 * time.Minute
	if time.Duration(exponentialDuration) > maxDuration {
		exponentialDuration = float64(maxDuration)
	}

	// Add jitter: 0-10% of the exponential duration
	jitter := time.Duration(rand.Float64() * exponentialDuration * 0.1)

	return time.Duration(exponentialDuration) + jitter
}

// CalculateNextRetryTime calculates when to retry based on the last attempt time.
// It returns the time to wait before the next retry.
func CalculateNextRetryTime(baseDuration time.Duration, attempt int) time.Duration {
	return NextBackoff(baseDuration, attempt)
}

// ShouldRetry determines if a retry should be attempted based on the current retry count.
func ShouldRetry(currentRetries, maxRetries int32) bool {
	return currentRetries < maxRetries
}
