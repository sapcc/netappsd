package netappsd

import "testing"

func TestRun(t *testing.T) {
	want := "Hello"
	if got := run(); got != want {
		t.Errorf("run() = %q, want %q", got, want)
	}
}
