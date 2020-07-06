package backoff

import "time"

// FF is set to Fast Forward time during testing.
// When true delays are 100 times shorter.
var FF bool

// NewExponential returns an exponential Sleep.
// First it sleeps 0.1 sec then 0.2 etc, sleep time is capped at max.
//
// Examples:
//
// Limit by number of retries:
//   for exp := backoff.NewExponential(10 * time.Second); exp.Retries() < 10; exp.Sleep() {
// Limit by time:
//	end := time.Now().Add(10 * time.Minute)
//	for exp := backoff.NewExponential(10 * time.Second); !time.Now().After(end); exp.Sleep() {
func NewExponential(max time.Duration) *Exponential {
	if max == 0 {
		max = 10 * time.Second
	}
	return &Exponential{
		max:    max,
		factor: 1,
	}
}

// Exponential Sleep
type Exponential struct {
	// max sleep time
	max time.Duration
	// wait factor
	factor int64
	// number of retries so far
	retries int
}

// Sleep exponentially longer on each invocation with a limit of max time.
func (ex *Exponential) Sleep() {
	d := int64(ex.factor) * 100 * time.Millisecond.Nanoseconds()
	if d > ex.max.Nanoseconds() || ex.factor < 0 {
		d = ex.max.Nanoseconds()
	} else {
		ex.factor <<= 1
	}
	if FF {
		d /= 100
	}
	ex.retries++
	time.Sleep(time.Duration(d))
}

// Retries returns the number of retries so far.
func (ex *Exponential) Retries() int {
	return ex.retries
}
