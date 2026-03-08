package integration_test

import (
	"testing"
	"time"
)

type inc struct{ N int }
type panicMsg struct{}
type counterQuery struct{ Ch chan int }

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}
