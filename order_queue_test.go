package orderdlq

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	if got := retryDelay("3", 0); got != 3*time.Second {
		t.Fatalf("Retry-After delay = %s, want 3s", got)
	}
	if got := retryDelay("", 2); got != 4*time.Second {
		t.Fatalf("exponential delay = %s, want 4s", got)
	}
}
