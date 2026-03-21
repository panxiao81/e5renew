package main

import "testing"

func TestMainCallsRun(t *testing.T) {
	originalRun := run
	t.Cleanup(func() {
		run = originalRun
	})

	called := false
	run = func() {
		called = true
	}

	main()

	if !called {
		t.Fatal("expected main to call run")
	}
}
