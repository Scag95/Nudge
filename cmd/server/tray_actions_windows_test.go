//go:build windows

package main

import (
	"context"
	"testing"
	"time"
)

func newTestTray() *winTray {
	return &winTray{logf: func(string, ...any) {}}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func stubAutostart(t *testing.T, f func(ctx context.Context, state string) error) {
	t.Helper()
	old := runAutostartScript
	runAutostartScript = f
	t.Cleanup(func() { runAutostartScript = old })
}

func TestSetAutostartRefusesOverlap(t *testing.T) {
	started := make(chan string, 4)
	release := make(chan struct{})
	stubAutostart(t, func(ctx context.Context, state string) error {
		started <- state
		<-release
		return nil
	})
	tr := newTestTray()
	if !tr.setAutostart(true) {
		t.Fatal("first setAutostart should start")
	}
	if got := <-started; got != "on" {
		t.Fatalf("state = %q, want on", got)
	}
	if tr.setAutostart(false) {
		t.Fatal("second setAutostart while in flight must return false")
	}
	select {
	case s := <-started:
		t.Fatalf("a second run started (%s)", s)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	waitFor(t, "busy to clear", func() bool { return !tr.autostartBusy.Load() })
}

func TestSetAutostartBusyClearsAfterCompletion(t *testing.T) {
	stubAutostart(t, func(ctx context.Context, state string) error { return nil })
	tr := newTestTray()
	if !tr.setAutostart(false) {
		t.Fatal("setAutostart should start")
	}
	waitFor(t, "busy to clear", func() bool { return !tr.autostartBusy.Load() })
	if !tr.setAutostart(true) {
		t.Fatal("a new toggle must be accepted after completion")
	}
	waitFor(t, "busy to clear", func() bool { return !tr.autostartBusy.Load() })
}

func TestSetAutostartTimeoutCancelsRun(t *testing.T) {
	old := autostartTimeout
	autostartTimeout = 30 * time.Millisecond
	t.Cleanup(func() { autostartTimeout = old })
	cancelled := make(chan struct{})
	stubAutostart(t, func(ctx context.Context, state string) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})
	tr := newTestTray()
	if !tr.setAutostart(true) {
		t.Fatal("setAutostart should start")
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("the blocked run was not cancelled by the timeout")
	}
	waitFor(t, "busy to clear", func() bool { return !tr.autostartBusy.Load() })
}
