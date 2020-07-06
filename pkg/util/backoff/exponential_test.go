package backoff

import (
	"testing"
	"time"
)

func TestNewExponential(t *testing.T) {
	tests := []struct {
		it          string
		fastForward bool
		max         time.Duration
		retries     int
		want        time.Duration
	}{
		{
			it:      "should_take_1.5secs_for_4_retries",
			retries: 4,
			want:    1500 * time.Millisecond,
		},
		{
			it:          "should_take_160mS_for_8_retries_in_fast_forward_mode",
			fastForward: true,
			max:         5 * time.Second,
			retries:     8,
			want:        160 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			FF = tt.fastForward
			var i int
			start := time.Now()
			for exp := NewExponential(tt.max); exp.Retries() < tt.retries; exp.Sleep() {
				i++
			}

			if tt.retries != i {
				t.Errorf("Expected %d retries, got: %d", tt.retries, i)
			}

			const maxDelta = 100 * time.Millisecond
			got := time.Now().Sub(start)
			d := got - tt.want
			if d > maxDelta || d < -maxDelta {
				t.Errorf("Expected %v, got %v", tt.want, got)
			}
		})
	}
}
