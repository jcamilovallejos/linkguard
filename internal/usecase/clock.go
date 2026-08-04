package usecase

import "time"

// SystemClock is the production domain.Clock backed by the real wall
// clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
